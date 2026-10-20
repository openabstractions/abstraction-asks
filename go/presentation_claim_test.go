package asks

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/openabstractions/abstraction-identity/listen"
)

func TestPresentationClaimUsesLiveWaitAndRecoversExpiredSlot(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	b.now = func() time.Time { return now }
	root := t.TempDir()
	raise := func(key string) Record {
		t.Helper()
		record, outcome, err := b.AskApplicationAs("scope", filepath.Join(root, key+".exe"), key, ApplicationCommandOnceKey,
			map[string]string{"program": "OpenCode", "operation": "Open ComfyUI", "operationmore": "",
				"command": "open", "code": key, "until": now.Add(time.Minute).UTC().Format(time.RFC3339)},
			listen.Seen{}, Publisher{})
		if err != nil || outcome != "pending" {
			t.Fatal(outcome, err)
		}
		return record
	}
	older := raise("12345678901234567890")
	now = now.Add(time.Second)
	newer := raise("12345678901234567891")
	_, stop := b.ObserveQuestionWait(newer.ID)
	defer stop()
	session := PresentationSession{Account: b.owner, Session: "session"}
	if _, err := b.ClaimPresentation(PresentationSession{Account: "foreign", Session: session.Session}); err == nil {
		t.Fatal("foreign account admitted")
	}
	first, err := b.ClaimPresentation(session)
	if err != nil || first.ID != newer.ID || first.Token == "" {
		t.Fatal(first, err)
	}
	if other, err := b.ClaimPresentation(session); err != ErrPresentationBusy || other.ID != "" {
		t.Fatal("a competing presenter acquired the slot", other, err)
	}
	now = now.Add(time.Second)
	renewed, err := b.RenewPresentation(session, first.Token)
	if err != nil || !renewed.Expires.After(first.Expires) {
		t.Fatal(renewed, err)
	}
	now = renewed.Expires
	stop()
	next, err := b.ClaimPresentation(session)
	if err != nil || next.ID != older.ID || next.Token == first.Token {
		t.Fatal(next, err)
	}
	b.ReleasePresentation(session, first.Token, true)
	if other, err := b.ClaimPresentation(session); err != ErrPresentationBusy || other.ID != "" {
		t.Fatal("stale release freed the new slot", other, err)
	}
	if _, outcome, err := b.AnswerApplication(older.ID, "refuse"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	if renewed, err := b.RenewPresentation(session, next.Token); err != nil || renewed.ID != "" {
		t.Fatal("answered occurrence renewed", renewed, err)
	}
	if other, err := b.ClaimPresentation(session); err != ErrPresentationBusy || other.ID != "" {
		t.Fatal("settlement freed the slot before dismissal", other, err)
	}
	b.ReleasePresentation(session, next.Token, true)
	last, err := b.ClaimPresentation(session)
	if err != nil || last.ID != newer.ID {
		t.Fatal(last, err)
	}
	b.ReleasePresentation(session, last.Token, false)
	retry, err := b.ClaimPresentation(session)
	if err != nil || retry.ID != newer.ID {
		t.Fatal("deferred occurrence did not retry", retry, err)
	}
	b.ReleasePresentation(session, retry.Token, true)
	if next, err := b.ClaimPresentation(session); err != nil || next.ID != "" {
		t.Fatal("dismissed pending occurrence showed again", next, err)
	}
	if record, found, err := b.ApplicationRecord(newer.ID); err != nil || !found || !record.Pending() {
		t.Fatal("dismissal altered the question", record, err)
	}
}
