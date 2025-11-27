package runner

import (
	"testing"

	"github.com/croessner/ldapbench/internal/config"
	"github.com/croessner/ldapbench/internal/csvdata"
	"github.com/croessner/ldapbench/internal/metrics"
)

type fakeClient struct {
	bindErr   error
	searchErr error
}

func (f *fakeClient) BindLookup() error                                   { return nil }
func (f *fakeClient) LookupDN(username string) (string, error)            { return "dn-" + username, nil }
func (f *fakeClient) UserBind(dn, password string) error                  { return f.bindErr }
func (f *fakeClient) UserSearch(dn, password, filter string) (int, error) { return 1, f.searchErr }
func (f *fakeClient) Close()                                              {}

func TestPrepareFilter(t *testing.T) {
	cfg := &config.Config{Filter: "(uid=%s)"}
	r := &Runner{cfg: cfg}

	if got := r.prepareFilter("alice"); got != "(uid=alice)" {
		t.Fatalf("unexpected filter: %s", got)
	}

	cfg2 := &config.Config{Filter: "(objectClass=person)"}
	r2 := &Runner{cfg: cfg2}

	if got := r2.prepareFilter("ignored"); got != cfg2.Filter {
		t.Fatalf("unexpected filter passthrough: %s", got)
	}
}

func TestRunOnce_ModeAuth_Success(t *testing.T) {
	cfg := &config.Config{Mode: config.ModeAuth}
	users := &csvdata.Users{All: []csvdata.User{{Username: "bob", Password: "pw"}}}
	m := metrics.New()
	r := &Runner{cfg: cfg, client: &fakeClient{}, users: users, m: m}

	r.runOnce()

	att, suc, fal, _ := m.Snapshot()
	if att != 1 || suc != 1 || fal != 0 {
		t.Fatalf("metrics mismatch: att=%d suc=%d fail=%d", att, suc, fal)
	}
}
