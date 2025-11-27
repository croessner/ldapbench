package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/croessner/ldapbench/internal/config"
	"github.com/croessner/ldapbench/internal/ldapclient"
)

// fake LDAP client implementing the interface used by check.Run
type fakeClient struct{}

func (f *fakeClient) BindLookup() error                                   { return nil }
func (f *fakeClient) LookupDN(username string) (string, error)            { return "dn-" + username, nil }
func (f *fakeClient) UserBind(dn, password string) error                  { return nil }
func (f *fakeClient) UserSearch(dn, password, filter string) (int, error) { return 1, nil }
func (f *fakeClient) Close()                                              {}

func TestRun_CheckAllModes(t *testing.T) {
	// prepare temp CSV
	dir := t.TempDir()

	csv := filepath.Join(dir, "users.csv")
	if err := os.WriteFile(csv, []byte("username,password\nuser1,pass1\n"), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	// inject fake client factory
	old := newClient
	newClient = func(cfg *config.Config) (ldapclient.Client, error) { return &fakeClient{}, nil }
	t.Cleanup(func() { newClient = old })

	// base cfg values used by check
	base := &config.Config{CSVPath: csv, BaseDN: "dc=example,dc=org", UIDAttr: "uid", LookupBindDN: "cn=svc", LookupBindPass: "pw"}

	for _, mode := range []config.Mode{config.ModeAuth, config.ModeSearch, config.ModeBoth} {
		c := *base
		c.Mode = mode

		if err := Run(&c); err != nil {
			t.Fatalf("Run failed for mode %s: %v", mode, err)
		}
	}
}
