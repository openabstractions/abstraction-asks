package asks

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/openabstractions/abstraction-identity/listen"
)

func TestPresentationFloodHoldsOneRequesterAndNotifiesOnce(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	b.now = func() time.Time { return now }
	root := t.TempDir()
	program := filepath.Join(root, "flood.exe")
	other := filepath.Join(root, "other.exe")
	raise := func(program string, i int) Record {
		t.Helper()
		key := fmt.Sprintf("popup-pacing-%d", i)
		permission, err := NewPermissionRequest(PermissionTarget{Account: b.owner, Program: program, Action: "abstraction.model/lookup", Resource: key})
		if err != nil {
			t.Fatal(err)
		}
		record, outcome, err := b.AskPermissionFrom("runtime", "same misleading title", key, permission, listen.Seen{}, Publisher{})
		if err != nil || outcome != "pending" {
			t.Fatal(outcome, err)
		}
		return record
	}
	s := PresentationSession{Account: b.owner, Session: "desktop"}
	first := raise(program, 0)
	claim, err := b.ClaimPresentation(s)
	if err != nil || claim.ID != first.ID {
		t.Fatal(claim, err)
	}
	now = now.Add(time.Second)
	for i := 0; i < 2; i++ {
		if ok, err := b.StartPresentation(s, claim.Token); !ok || err != nil {
			t.Fatal(ok, err)
		}
	}
	if len(b.popupTimes) != 1 {
		t.Fatal(b.popupTimes)
	}
	// Simulate the displayed presenter disappearing without Release.
	var held Record
	for i := 1; i < 100; i++ {
		now = now.Add(95 * time.Millisecond)
		held = raise(program, i)
	}
	separate := raise(other, 100)
	_, stop := b.ObserveQuestionWait(held.ID)
	defer stop()
	now = claim.Expires // presenter crashed without releasing its slot
	claim, err = b.ClaimPresentation(s)
	if err != nil || claim.ID != separate.ID || !b.ReleasePresentation(s, claim.Token, true) {
		t.Fatal(claim, err)
	}
	notice := b.PresentationHoldNotice(s)
	if notice.ID == "" || notice.Program == "" || notice.Program == "same misleading title" {
		t.Fatal(notice)
	}
	if again := b.PresentationHoldNotice(s); again != notice {
		t.Fatal(again)
	}
	if !b.AcknowledgePresentationHold(s, notice.Token) {
		t.Fatal("ack refused")
	}
	if again := b.PresentationHoldNotice(s); again.ID != "" {
		t.Fatal(again)
	}
	if claim, err := b.ClaimPresentation(s); err != nil || claim.ID != "" {
		t.Fatal(claim, err)
	}
	if record, found, err := b.ApplicationRecord(held.ID); err != nil || !found || !record.Pending() {
		t.Fatal(found, err)
	}
	if _, outcome, err := b.AnswerApplicationIfPendingAuthorized(held.ID, "allow", nil); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	now = now.Add(time.Second)
	if claim, err := b.ClaimPresentation(s); err != nil || claim.ID == "" {
		t.Fatal(claim, err)
	}
}
