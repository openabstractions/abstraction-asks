package asks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openabstractions/abstraction-identity/listen"
)

func unsavedFixture(t *testing.T) (*Book, func(string) Record) {
	t.Helper()
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	slots := map[string]string{"program": filepath.Join(t.TempDir(), "opencode.exe"), "action": "abstraction.credentials/apply", "resource": "credential:deepseek"}
	return b, func(key string) Record {
		r, _, err := b.AskApplication("scope", "app", key, FirstUseKey, slots, listen.Seen{})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
}

func notSaved() (bool, error) { return false, nil }

// Owner 2026-11-09: an Allow whose permission could not be saved is released;
// the same key then asks afresh. Only an operator's Allow is released.
func TestReleaseUnsavedAllowAsksAfreshUnderTheSameKey(t *testing.T) {
	b, ask := unsavedFixture(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pending := ask("pending")
	if released, err := b.ReleaseUnsavedAllow(pending.ID, notSaved); err != nil || released {
		t.Fatal("a pending question was released", released, err)
	}
	refused := ask("refused")
	if _, _, err := b.AnswerApplicationIfPendingAuthorized(refused.ID, "refuse", nil); err != nil {
		t.Fatal(err)
	}
	if released, err := b.ReleaseUnsavedAllow(refused.ID, notSaved); err != nil || released {
		t.Fatal("a refusal was released", released, err)
	}
	delegated := ask("delegated")
	if _, _, err := b.AnswerApplicationAsResponder(delegated.ID, "allow", exe, nil); err != nil {
		t.Fatal(err)
	}
	if released, err := b.ReleaseUnsavedAllow(delegated.ID, notSaved); err != nil || released {
		t.Fatal("a responder's Allow was released", released, err)
	}
	allowed := ask("allowed")
	if _, outcome, err := b.AnswerApplicationIfPendingAuthorized(allowed.ID, "allow", nil); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	if released, err := b.ReleaseUnsavedAllow(allowed.ID, notSaved); err != nil || !released {
		t.Fatal("the unsaved Allow stands", released, err)
	}
	if _, found, err := b.ApplicationRecord(allowed.ID); err != nil || found {
		t.Fatal("the released Allow is still retained", found, err)
	}
	if _, outcome, err := b.ObserveApplication("scope", "allowed"); err != nil || outcome != "unknown" {
		t.Fatal("the key still names the released Allow", outcome, err)
	}
	fresh := ask("allowed")
	if fresh.ID == allowed.ID || !fresh.Pending() {
		t.Fatal("the same key did not ask afresh", fresh)
	}
}

// keep runs under the Book lock: a save that saved the rule keeps the Allow,
// and a save pinned through WithApplicationRecordContext meanwhile waits and
// then finds no record.
func TestReleaseUnsavedAllowReadsKeepUnderTheBookLock(t *testing.T) {
	b, ask := unsavedFixture(t)
	answer := func(key string) Record {
		r := ask(key)
		if _, _, err := b.AnswerApplicationIfPendingAuthorized(r.ID, "allow", nil); err != nil {
			t.Fatal(err)
		}
		return r
	}
	saved := answer("saved")
	if released, err := b.ReleaseUnsavedAllow(saved.ID, func() (bool, error) { return true, nil }); err != nil || released {
		t.Fatal("a saved Allow was released", released, err)
	}
	unknown := answer("unknown")
	if released, err := b.ReleaseUnsavedAllow(unknown.ID, func() (bool, error) { return false, errors.New("rights: unreadable") }); err != nil || released {
		t.Fatal("an unknown save state released the Allow", released, err)
	}
	for _, r := range []Record{saved, unknown} {
		if _, found, err := b.ApplicationRecord(r.ID); err != nil || !found {
			t.Fatal("a kept Allow is gone", found, err)
		}
	}

	racing := answer("racing")
	pinned := make(chan bool, 1)
	_, err := b.ReleaseUnsavedAllow(racing.ID, func() (bool, error) {
		go func() {
			found := false
			_, err := b.WithApplicationRecordContext(context.Background(), racing.ID, func(Record, string, []string) error {
				found = true
				return nil
			})
			pinned <- err == nil && found
		}()
		// Without the Book lock the concurrent pin would find the record now.
		time.Sleep(50 * time.Millisecond)
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if <-pinned {
		t.Fatal("a concurrent save pinned the Allow while it was being released")
	}
}
