package ldapclient

// Package ldapclient wraps basic LDAP operations used by the benchmark: a
// lookup (service) connection for DN resolution as well as per-user bind and
// search operations.

import (
	"crypto/tls"
	"fmt"
	"strings"
	"sync"

	"github.com/croessner/ldapbench/internal/config"
	"github.com/go-ldap/ldap/v3"
)

// Client exposes the minimal operations required by the runner.
type Client interface {
	BindLookup() error
	LookupDN(username string) (string, error)
	UserBind(dn, password string) error
	UserSearch(dn, password, filter string) (int, error) // returns entry count
	Close()
}

type client struct {
	cfg  *config.Config
	conn *ldap.Conn // shared lookup connection
	mu   sync.Mutex

	// pool of persistent user connections reused across operations
	pool chan *ldap.Conn
}

// New creates a new client and establishes the lookup connection.
func New(cfg *config.Config) (Client, error) {
	c := &client{cfg: cfg}

	if err := c.connectLookup(); err != nil {
		return nil, err
	}

	// Initialize user connection pool (lazy). We only create the buffered
	// channel with the target capacity (concurrency * connections) but do not
	// pre-dial all connections at startup to avoid connection storms and
	// server-side limits causing immediate EOFs. Connections are established on
	// demand by workers.
	size := cfg.Concurrency * cfg.Connections
	if size < 1 {
		size = 1
	}

	c.pool = make(chan *ldap.Conn, size)

	return c, nil
}

// connectLookup dials the server for the service/lookup account.
func (c *client) connectLookup() error {
	var l *ldap.Conn
	var err error

	// Dial according to URL scheme. ldap library supports ldap://, ldaps://, and ldapi://.
	// We only apply StartTLS on plain ldap://; ldaps:// uses TLS from the start and
	// ldapi:// (Unix domain socket) does not support StartTLS.
	if strings.HasPrefix(c.cfg.LDAPURL, "ldaps://") {
		l, err = ldap.DialURL(c.cfg.LDAPURL, ldap.DialWithTLSConfig(c.cfg.TLSConfig()))
	} else {
		l, err = ldap.DialURL(c.cfg.LDAPURL)
		if err == nil && c.cfg.StartTLS && strings.HasPrefix(c.cfg.LDAPURL, "ldap://") {
			if err2 := l.StartTLS(c.cfg.TLSConfig()); err2 != nil {
				l.Close()
				err = err2
			}
		}
	}

	if err != nil {
		return err
	}

	// Apply a reasonable timeout for lookup operations
	l.SetTimeout(c.cfg.Timeout)
	c.mu.Lock()
	c.conn = l
	c.mu.Unlock()

	return nil
}

// BindLookup performs a simple bind with the lookup account.
func (c *client) BindLookup() error {
	c.mu.Lock()
	l := c.conn
	c.mu.Unlock()

	return l.Bind(c.cfg.LookupBindDN, c.cfg.LookupBindPass)
}

// LookupDN finds a user's DN using the configured UID attribute.
func (c *client) LookupDN(username string) (string, error) {
	c.mu.Lock()
	l := c.conn
	c.mu.Unlock()

	filter := fmt.Sprintf("(&(%s=%s)(objectClass=person))", c.cfg.UIDAttr, ldap.EscapeFilter(username))
	req := ldap.NewSearchRequest(
		c.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, int(c.cfg.Timeout.Seconds()), false,
		filter,
		[]string{"dn"},
		nil,
	)

	res, err := l.Search(req)
	if err != nil {
		return "", err
	}

	if len(res.Entries) == 0 {
		return "", fmt.Errorf("user not found")
	}

	return res.Entries[0].DN, nil
}

// dialUser creates a new connection for a user operation (bind/search).
func (c *client) dialUser() (*ldap.Conn, error) {
	// Same scheme handling as connectLookup.
	if strings.HasPrefix(c.cfg.LDAPURL, "ldaps://") {
		l, err := ldap.DialURL(c.cfg.LDAPURL, ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: c.cfg.InsecureSkipVerify}))
		if err != nil {
			return nil, err
		}

		l.SetTimeout(c.cfg.Timeout)

		return l, nil
	}

	l, err := ldap.DialURL(c.cfg.LDAPURL)
	if err != nil {
		return nil, err
	}

	if c.cfg.StartTLS && strings.HasPrefix(c.cfg.LDAPURL, "ldap://") {
		if err := l.StartTLS(c.cfg.TLSConfig()); err != nil {
			l.Close()

			return nil, err
		}
	}

	l.SetTimeout(c.cfg.Timeout)

	return l, nil
}

// UserBind performs a bind using the provided DN and password on a fresh
// connection to avoid cross-talk and to simulate real-world auth traffic.
func (c *client) UserBind(dn, password string) error {
	l := c.getConn()
	if l == nil {
		return fmt.Errorf("no connection available")
	}

	// Rebind on the persistent connection; do not unbind/close.
	err := l.Bind(dn, password)
	c.putConn(l, err)

	return err
}

// UserSearch binds as the user and executes a search; returns number of entries.
func (c *client) UserSearch(dn, password, filter string) (int, error) {
	l := c.getConn()
	if l == nil {
		return 0, fmt.Errorf("no connection available")
	}

	if err := l.Bind(dn, password); err != nil {
		c.putConn(l, err)

		return 0, err
	}

	req := ldap.NewSearchRequest(
		c.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, int(c.cfg.Timeout.Seconds()), false,
		filter, []string{"dn"}, nil,
	)

	res, err := l.Search(req)
	c.putConn(l, err)
	if err != nil {
		return 0, err
	}

	return len(res.Entries), nil
}

// Close closes the lookup connection.
func (c *client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	if c.pool != nil {
		close(c.pool)
		for l := range c.pool {
			if l != nil {
				l.Close()
			}
		}
	}
}

// getConn borrows a user connection from the pool.
func (c *client) getConn() *ldap.Conn {
	// Try to reuse an existing connection if available without blocking.
	select {
	case l := <-c.pool:
		return l
	default:
	}

	// Otherwise dial a new one on demand. If dialing fails, return nil so the
	// caller can count a failure without blocking.
	l, err := c.dialUser()
	if err != nil {
		return nil
	}

	return l
}

// putConn returns the connection to the pool. If err suggests the connection is
// broken, the connection is closed and replaced with a fresh one.
func (c *client) putConn(l *ldap.Conn, err error) {
	if l == nil {
		return
	}

	if err != nil {
		// On error, consider the connection tainted: close it and do not
		// return it to the pool. We do not immediately redial here to keep
		// pressure off the server; subsequent getConn will dial on demand.
		l.Close()

		return
	}

	// Return to pool if there is space; otherwise close to avoid leaking file descriptors.
	select {
	case c.pool <- l:
	default:
		l.Close()
	}
}
