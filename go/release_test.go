package asks

import (
	"path/filepath"
	"testing"

	"github.com/openabstractions/abstraction-identity/listen"
)

// ReleaseAnsweredApplication removes only an answered question with the named
// option, leaves no tombstone, and lets the same key admit a new question
// (ASK-R2, ASK-A7).
func TestReleaseAnsweredApplicationFreesTheKey(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "application.json"))
	if err != nil {
		t.Fatal(err)
	}
	slots := map[string]string{"program": filepath.Join(t.TempDir(), "app.exe"), "action": "abstraction.model/lookup", "resource": "hf"}
	asked, outcome, err := b.AskApplication("runtime", "runtime", "first-use:key", FirstUseKey, slots, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	if released, err := b.ReleaseAnsweredApplication("runtime", "first-use:key", "allow"); err != nil || released {
		t.Fatalf("a pending question was released: %v %v", released, err)
	}
	if _, outcome, err = b.AnswerApplication(asked.ID, "allow"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	if released, err := b.ReleaseAnsweredApplication("runtime", "first-use:key", "refuse"); err != nil || released {
		t.Fatalf("an allow was released as a refuse: %v %v", released, err)
	}
	if released, err := b.ReleaseAnsweredApplication("other-scope", "first-use:key", "allow"); err != nil || released {
		t.Fatalf("another scope released the question: %v %v", released, err)
	}
	if released, err := b.ReleaseAnsweredApplication("runtime", "first-use:key", "allow"); err != nil || !released {
		t.Fatalf("the answered allow was not released: %v %v", released, err)
	}
	if _, outcome, err = b.ObserveApplication("runtime", "first-use:key"); err != nil || outcome != "unknown" {
		t.Fatalf("a released key must observe unknown, never gone: %q %v", outcome, err)
	}
	if _, found, err := b.ApplicationRecord(asked.ID); err != nil || found {
		t.Fatalf("a released question must not be retained: %v %v", found, err)
	}
	again, outcome, err := b.AskApplication("runtime", "runtime", "first-use:key", FirstUseKey, slots, listen.Seen{})
	if err != nil || outcome != "pending" || again.ID == asked.ID {
		t.Fatalf("the key did not admit a new question: %+v %q %v", again, outcome, err)
	}
	if released, err := b.ReleaseAnsweredApplication("runtime", "first-use:key", "allow"); err != nil || released {
		t.Fatalf("a second release removed the new pending question: %v %v", released, err)
	}

	// A dismissed question stays gone.
	if _, outcome, err := b.RetireApplicationAuthorized(again.ID, nil); err != nil || outcome != "retired" {
		t.Fatal(outcome, err)
	}
	if released, err := b.ReleaseAnsweredApplication("runtime", "first-use:key", "allow"); err != nil || released {
		t.Fatalf("a dismissed question was released: %v %v", released, err)
	}
	if _, outcome, err = b.ObserveApplication("runtime", "first-use:key"); err != nil || outcome != "gone" {
		t.Fatalf("a dismissed key must stay gone: %q %v", outcome, err)
	}
}
