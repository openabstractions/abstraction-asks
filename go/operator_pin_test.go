package asks

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	cas "github.com/openabstractions/abstraction-cas/go"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestObservedReleaseCannotRemoveReplacementAnswer(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "application.json"))
	if err != nil {
		t.Fatal(err)
	}
	b.now = func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC) }
	slots := map[string]string{"program": filepath.Join(t.TempDir(), "app.exe"), "action": "abstraction.model/lookup", "resource": "hf"}
	first, _, err := b.AskApplication("runtime", "runtime", "first-use:key", FirstUseKey, slots, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.AnswerApplication(first.ID, "allow"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReleaseAnsweredApplicationID("runtime", "first-use:key", first.ID, "allow"); err != nil {
		t.Fatal(err)
	}
	second, _, err := b.AskApplication("runtime", "runtime", "first-use:key", FirstUseKey, slots, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.AnswerApplication(second.ID, "allow"); err != nil {
		t.Fatal(err)
	}
	if yes, err := b.ReleaseAnsweredApplicationID("runtime", "first-use:key", first.ID, "allow"); err != nil || yes {
		t.Fatalf("stale Q1 released Q2: %v %v", yes, err)
	}
	if current, outcome, err := b.ObserveApplication("runtime", "first-use:key"); err != nil || outcome != "answered" || current.ID != second.ID {
		t.Fatalf("replacement changed: %+v %s %v", current, outcome, err)
	}
}

func TestBookPinLockWaitsHonorContext(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "application.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, lock := range []string{"mutex", "file"} {
		t.Run(lock, func(t *testing.T) {
			release, entered, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			if lock == "mutex" {
				b.mu.Lock()
				close(entered)
				go func() { <-release; b.mu.Unlock(); done <- nil }()
			} else {
				go func() {
					done <- cas.ChangeLimit(b.path, MaxApplicationBytes, func(data []byte) ([]byte, error) { close(entered); <-release; return data, nil })
				}()
			}
			<-entered
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			calls := 0
			_, err := b.WithApplicationRecordContext(ctx, "missing", func(Record, string, []string) error { calls++; return nil })
			close(release)
			if lockErr := <-done; lockErr != nil {
				t.Fatal(lockErr)
			}
			if !errors.Is(err, context.DeadlineExceeded) || calls != 0 {
				t.Fatalf("late Book effect: calls=%d err=%v", calls, err)
			}
		})
	}
}
