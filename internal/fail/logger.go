package fail

// Package fail provides optional batched failure logging to a CSV file.

import (
	"encoding/csv"
	"os"
	"sync"
	"time"
)

// Record describes a failed attempt.
type Record struct {
	Timestamp time.Time
	Operation string // lookup|bind|search
	Username  string
	DN        string
	Filter    string
	Error     string
}

// Logger writes failure records to a CSV file in batches.
type Logger struct {
	path   string
	batch  int
	ch     chan Record
	wg     sync.WaitGroup
	stopCh chan struct{}
}

// New creates a new Logger. When path is empty, returns nil.
func New(path string, batch int) *Logger {
	if path == "" {
		return nil
	}

	if batch <= 0 {
		batch = 256
	}

	l := &Logger{path: path, batch: batch, ch: make(chan Record, batch*4), stopCh: make(chan struct{})}
	l.wg.Add(1)
	go l.run()

	return l
}

// Log queues a record for writing.
func (l *Logger) Log(rec Record) {
	if l == nil {
		return
	}

	select {
	case l.ch <- rec:
	default:
		// drop on backpressure to not affect benchmark timing
	}
}

// Close flushes and stops the logger.
func (l *Logger) Close() {
	if l == nil {
		return
	}

	close(l.stopCh)
	l.wg.Wait()
}

func (l *Logger) run() {
	defer l.wg.Done()

	// Open file once; append mode.
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// if opening fails, drain channel and exit silently
		for range l.ch {
		}

		return
	}

	defer f.Close()

	w := csv.NewWriter(f)
	// Write header
	_ = w.Write([]string{"timestamp", "operation", "username", "dn", "filter", "error"})
	w.Flush()

	buf := make([]Record, 0, l.batch)
	flush := func() {
		if len(buf) == 0 {
			return
		}

		for _, r := range buf {
			_ = w.Write([]string{
				r.Timestamp.Format(time.RFC3339Nano), r.Operation, r.Username, r.DN, r.Filter, r.Error,
			})
		}

		w.Flush()
		buf = buf[:0]
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			// drain remaining records
			for {
				select {
				case r := <-l.ch:
					buf = append(buf, r)
					if len(buf) >= l.batch {
						flush()
					}
				default:
					flush()
					return
				}
			}
		case r := <-l.ch:
			buf = append(buf, r)
			if len(buf) >= l.batch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
