package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/croessner/ldapbench/internal/metrics"
)

func TestPrintSummary(t *testing.T) {
	m := metrics.New()
	m.Attempts.Add(10)
	m.Success.Add(7)
	m.Fail.Add(3)

	var buf bytes.Buffer
	PrintSummary(&buf, m, 2*time.Second)
	out := buf.String()

	for _, want := range []string{"Summary", "attempts: 10", "success: 7", "fail: 3", "avg rps"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q in output: %s", want, out)
		}
	}
}
