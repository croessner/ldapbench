package check

// Package check provides a lightweight connectivity/config verification that can
// be executed via --check to validate CLI parameters, CSV, and LDAP access
// without running the full benchmark.

import (
	"fmt"
	"strings"

	"github.com/croessner/ldapbench/internal/config"
	"github.com/croessner/ldapbench/internal/csvdata"
	"github.com/croessner/ldapbench/internal/ldapclient"
)

// newClient is a small indirection to allow tests to inject a fake LDAP client
// without changing the public API. In production it points to ldapclient.New.
var newClient = ldapclient.New

// Run performs a short verification sequence and returns precise errors.
func Run(cfg *config.Config) error {
	// Load CSV
	users, err := csvdata.Load(cfg.CSVPath)
	if err != nil {
		return fmt.Errorf("csv error: %w", err)
	}

	if len(users.All) == 0 {
		return fmt.Errorf("csv error: no users found in %s", cfg.CSVPath)
	}

	fmt.Printf("OK: CSV '%s' loaded (%d users)\n", cfg.CSVPath, len(users.All))

	// LDAP client and lookup bind
	client, err := newClient(cfg)
	if err != nil {
		return fmt.Errorf("ldap client error: %w", err)
	}

	defer client.Close()

	if err := client.BindLookup(); err != nil {
		return fmt.Errorf("lookup bind failed: %w", err)
	}

	fmt.Println("OK: Lookup bind")

	// Check example user (first entry)
	u := users.All[0]

	dn, err := client.LookupDN(u.Username)
	if err != nil {
		return fmt.Errorf("lookup dn failed for user '%s': %w", u.Username, err)
	}

	fmt.Printf("OK: DN for user '%s' found: %s\n", u.Username, dn)

	// Depending on mode: test user bind and/or search
	switch cfg.Mode {
	case config.ModeAuth:
		if err := client.UserBind(dn, u.Password); err != nil {
			return fmt.Errorf("user bind failed for '%s': %w", u.Username, err)
		}

		fmt.Printf("OK: User bind for '%s'\n", u.Username)

	case config.ModeSearch:
		filter := cfg.Filter
		if strings.Contains(filter, "%s") {
			filter = fmt.Sprintf(filter, u.Username)
		}

		if _, err := client.UserSearch(dn, u.Password, filter); err != nil {
			return fmt.Errorf("user search failed for '%s' with filter '%s': %w", u.Username, filter, err)
		}

		fmt.Printf("OK: Search with filter '%s'\n", filter)

	case config.ModeBoth:
		if err := client.UserBind(dn, u.Password); err != nil {
			return fmt.Errorf("user bind failed for '%s': %w", u.Username, err)
		}

		fmt.Printf("OK: User bind for '%s'\n", u.Username)

		filter := cfg.Filter
		if strings.Contains(filter, "%s") {
			filter = fmt.Sprintf(filter, u.Username)
		}

		if _, err := client.UserSearch(dn, u.Password, filter); err != nil {
			return fmt.Errorf("user search failed for '%s' with filter '%s': %w", u.Username, filter, err)
		}

		fmt.Printf("OK: Search with filter '%s'\n", filter)
	}

	return nil
}
