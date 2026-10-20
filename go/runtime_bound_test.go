package asks

import (
	"path/filepath"
	"testing"

	"github.com/openabstractions/abstraction-identity/listen"
)

// TestWithdrawRuntimeBoundAtStartup is ASK-S1 across a restart: a runtime
// opening its Book withdraws every pending system-access question, which can
// then be neither answered nor replayed, and leaves answered and other
// questions as they are.
func TestWithdrawRuntimeBoundAtStartup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "reader.exe")
	system := func(key, code string) Record {
		r, outcome, err := b.AskApplicationAs("runtime", program, key, SystemSecurityLogKey, map[string]string{"code": code, "until": "later"}, listen.Seen{}, Publisher{})
		if err != nil || outcome != "pending" {
			t.Fatalf("%s: %q %v", key, outcome, err)
		}
		return r
	}
	stale := system("system-access:stale", "a1b2c3d4")
	answered := system("system-access:answered", "b1b2c3d4")
	if _, outcome, err := b.AnswerApplication(answered.ID, "once"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	other, outcome, err := b.AskApplication("runtime", "runtime", "reach", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}

	// The restart: a new runtime opens the same Book.
	restarted, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	withdrawn, err := restarted.WithdrawRuntimeBound()
	if err != nil || len(withdrawn) != 1 || withdrawn[0].ID != stale.ID {
		t.Fatalf("withdrew %+v %v, want only the pending system-access question", withdrawn, err)
	}
	if _, outcome, err := restarted.AnswerApplication(stale.ID, "once"); err != nil || outcome == "answered" {
		t.Fatalf("the withdrawn question was answered after the restart: %q %v", outcome, err)
	}
	if _, outcome, err := restarted.ObserveApplication("runtime", "system-access:stale"); err != nil || outcome != "gone" {
		t.Fatalf("the withdrawn key observes %q %v, want gone", outcome, err)
	}
	if r, found, err := restarted.ApplicationRecord(answered.ID); err != nil || !found || r.Option != "once" {
		t.Fatalf("an answered system-access question changed: %+v %v %v", r, found, err)
	}
	if r, found, err := restarted.ApplicationRecord(other.ID); err != nil || !found || !r.Pending() {
		t.Fatalf("an ordinary pending question was withdrawn: %+v %v %v", r, found, err)
	}
	if again, err := restarted.WithdrawRuntimeBound(); err != nil || len(again) != 0 {
		t.Fatalf("a second startup withdrew %+v %v", again, err)
	}
	for _, q := range Questions {
		if q.RuntimeBound != (q.Key == SystemSecurityLogKey) {
			t.Fatalf("%s RuntimeBound=%v: only the system-access question is bound to runtime memory", q.Key, q.RuntimeBound)
		}
	}
}
