package metrics

import (
	"testing"
	"time"
)

func TestNewAndSnapshot(t *testing.T) {
	m := New()
	if time.Since(m.Start) > time.Second {
		t.Fatalf("unexpected start time: %v", m.Start)
	}

	m.Attempts.Add(2)
	m.Success.Add(1)
	m.Fail.Add(1)

	att, suc, fal, el := m.Snapshot()
	if att != 2 || suc != 1 || fal != 1 {
		t.Fatalf("snapshot mismatch: got %d/%d/%d", att, suc, fal)
	}

	if el <= 0 {
		t.Fatalf("elapsed should be > 0, got %v", el)
	}
}
