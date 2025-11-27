package csvdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "users.csv")

	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp: %v", err)
	}

	return p
}

func TestLoad_OK(t *testing.T) {
	p := writeTemp(t, "username,password\nuser1,pass1\nuser2,pass2\n")
	u, err := Load(p)

	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(u.All) != 2 {
		t.Fatalf("expected 2 users, got %d", len(u.All))
	}

	if u.All[0].Username != "user1" || u.All[0].Password != "pass1" {
		t.Fatalf("unexpected first user: %+v", u.All[0])
	}
}

func TestLoad_HeaderError(t *testing.T) {
	p := writeTemp(t, "user,password\nfoo,bar\n")

	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "username,password") {
		t.Fatalf("expected header error, got %v", err)
	}
}

func TestLoad_ExpectedOKFilter(t *testing.T) {
	p := writeTemp(t, "username,password,expected_ok\nu1,p1,true\nu2,p2,false\n")

	u, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(u.All) != 1 || u.All[0].Username != "u1" || !u.All[0].ExpectedOK {
		t.Fatalf("unexpected filter result: %+v", u.All)
	}
}
