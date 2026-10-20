package asks

import (
	"path/filepath"
	"testing"

	"github.com/openabstractions/abstraction-identity/listen"
)

func TestLiveQuestionWaitObservations(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, outcome, err := b.AskApplicationAs("scope", filepath.Join(t.TempDir(), "caller.exe"), "key",
		"download.reach", map[string]string{"host": "example.com"}, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	count := func(id string, want int) {
		t.Helper()
		if got := b.LiveQuestionWaiters(id); got != want {
			t.Fatalf("live waiters = %d, want %d", got, want)
		}
	}
	first, cancel := b.ObserveQuestionWait(record.ID)
	second, release := b.ObserveQuestionWait(record.ID)
	if first != second {
		t.Fatal("waiters do not share settlement")
	}
	count(record.ID, 2)
	cancel()
	cancel()
	count(record.ID, 1)
	unknown, stop := b.ObserveQuestionWait("unknown")
	stop()
	<-unknown
	count("unknown", 0)

	for range maxLiveQuestionWaits {
		b.ObserveQuestionWait(record.ID)
	}
	count(record.ID, maxLiveQuestionWaits)
	if err := b.Forget(record.ID); err != nil {
		t.Fatal(err)
	}
	<-second
	count(record.ID, 0)
	release()
	finished, stop := b.ObserveQuestionWait(record.ID)
	<-finished
	stop()
	count(record.ID, 0)
}
