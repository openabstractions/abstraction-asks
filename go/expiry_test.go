package asks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openabstractions/abstraction-identity/listen"
)

// TestPendingQuestionsExpire is ASK-A7: a question nobody answered within the
// pending lifetime is no longer retained. Every call reads it as unknown, its
// idempotency key admits a new question, and an answered question never
// expires.
func TestPendingQuestionsExpire(t *testing.T) {
	if DefaultPendingLifetime != 15*time.Minute {
		t.Fatalf("DefaultPendingLifetime = %v, want the decided 15 minutes", DefaultPendingLifetime)
	}
	path := filepath.Join(t.TempDir(), "application.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return clock }
	slots := map[string]string{"host": "example.com"}
	pending, outcome, err := b.AskApplication("scope", "app", "expires", "download.reach", slots, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	kept, _, err := b.AskApplication("scope", "app", "answered", "download.reach", map[string]string{"host": "answered.example"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if _, outcome, err = b.AnswerApplication(kept.ID, "refuse"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}

	clock = clock.Add(DefaultPendingLifetime - time.Nanosecond)
	if _, outcome, err = b.ObserveApplication("scope", "expires"); err != nil || outcome != "pending" {
		t.Fatalf("a question inside its lifetime must stay pending: %q %v", outcome, err)
	}
	clock = clock.Add(time.Nanosecond)
	if _, outcome, err = b.ObserveApplication("scope", "expires"); err != nil || outcome != "unknown" {
		t.Fatalf("an expired question must observe unknown: %q %v", outcome, err)
	}
	if _, found, err := b.ApplicationRecord(pending.ID); err != nil || found {
		t.Fatalf("an expired question must not be retained: %v %v", found, err)
	}
	history, _, err := b.ApplicationHistory()
	if err != nil || len(history) != 1 || history[0].ID != kept.ID {
		t.Fatalf("history must keep only the answered question: %+v %v", history, err)
	}
	if _, outcome, err = b.AnswerApplication(pending.ID, "allow"); err != nil || outcome != "unknown" {
		t.Fatalf("answering an expired question must be unknown: %q %v", outcome, err)
	}
	if _, outcome, err = b.AnswerApplicationIfPendingAuthorized(pending.ID, "allow", nil); err != nil || outcome != "unknown" {
		t.Fatalf("a conditional answer to an expired question must be unknown: %q %v", outcome, err)
	}
	if _, outcome, err = b.RetireApplicationAuthorized(pending.ID, nil); err != nil || outcome != "unknown" {
		t.Fatalf("dismissing an expired question must be unknown: %q %v", outcome, err)
	}
	again, outcome, err := b.AskApplication("scope", "app", "expires", "download.reach", slots, listen.Seen{})
	if err != nil || outcome != "pending" || again.ID == pending.ID || !again.Asked.Equal(clock) {
		t.Fatalf("the expired question's key must admit a new question: %+v %q %v", again, outcome, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(data), pending.ID) {
		t.Fatalf("the next write must remove the expired question and its admission from the state file: %v", err)
	}
	if _, outcome, err = b.ObserveApplication("scope", "answered"); err != nil || outcome != "answered" {
		t.Fatalf("an answered question never expires: %q %v", outcome, err)
	}

	if err := b.SetPendingLifetime(0); err == nil {
		t.Fatal("a pending lifetime must be positive")
	}
	if err := b.SetPendingLifetime(time.Minute); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	if _, outcome, err = b.ObserveApplication("scope", "expires"); err != nil || outcome != "unknown" {
		t.Fatalf("a configured lifetime must apply: %q %v", outcome, err)
	}
}

// A question read as decided while it looked expired, then revived by a
// longer lifetime and answered, settles without closing its decided channel
// twice: the Book keeps no channel it already closed.
func TestDecidedOnALapsedQuestionThenItsAnswerSettlesOnce(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "application.json"))
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return clock }
	pending, outcome, err := b.AskApplication("scope", "app", "lapses", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	if err := b.SetPendingLifetime(time.Minute); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(2 * time.Minute)
	select {
	case <-b.Decided(pending.ID):
	default:
		t.Fatal("a lapsed question must read as decided")
	}
	if err := b.SetPendingLifetime(time.Hour); err != nil {
		t.Fatal(err)
	}
	waiting := b.Decided(pending.ID)
	if _, outcome, err := b.AnswerApplication(pending.ID, "allow"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	<-waiting
}

// TestKeyLifetimeOutlastsTheBookLifetime is ASK-A7's per-key lifetime: a key
// the host gave a longer lifetime stays pending past the Book's own
// lifetime, other keys expire at it, and PendingUntil states each one.
func TestKeyLifetimeOutlastsTheBookLifetime(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "application.json"))
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return clock }
	if err := b.SetKeyLifetime("download.reach", 0); err == nil {
		t.Fatal("a key lifetime must be positive")
	}
	if err := b.SetKeyLifetime("", time.Hour); err == nil {
		t.Fatal("a key lifetime must name a key")
	}
	if err := b.SetKeyLifetime("download.reach", 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	long, outcome, err := b.AskApplication("scope", "app", "long", "download.reach", map[string]string{"host": "long.example"}, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	if got := b.PendingUntil(long); !got.Equal(long.Asked.Add(24 * time.Hour)) {
		t.Fatalf("PendingUntil = %v, want asked + 24 h", got)
	}
	clock = clock.Add(DefaultPendingLifetime)
	if _, outcome, err = b.ObserveApplication("scope", "long"); err != nil || outcome != "pending" {
		t.Fatalf("a question under its key's longer lifetime expired at the Book's: %q %v", outcome, err)
	}
	clock = long.Asked.Add(24 * time.Hour)
	if _, outcome, err = b.ObserveApplication("scope", "long"); err != nil || outcome != "unknown" {
		t.Fatalf("a question past its key's lifetime stayed: %q %v", outcome, err)
	}
}
