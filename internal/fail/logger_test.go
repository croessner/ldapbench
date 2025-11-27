package fail

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogger_WriteAndClose(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fail.csv")

	l := New(p, 2)
	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	// enqueue a few records
	l.Log(Record{Timestamp: time.Now(), Operation: "lookup", Username: "u", DN: "dn", Error: "e"})
	l.Log(Record{Timestamp: time.Now(), Operation: "bind", Username: "u2", DN: "dn2", Error: "e2"})

	// ensure flush on close
	l.Close()

	f, err := os.Open(p)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	// read lines
	sc := bufio.NewScanner(f)
	var lines []string

	for sc.Scan() {
		lines = append(lines, sc.Text())
	}

	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(lines) < 3 { // header + 2 records
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	if want := "timestamp,operation,username,dn,filter,error"; !strings.Contains(lines[0], want) {
		t.Fatalf("missing header, got: %q", lines[0])
	}
}
