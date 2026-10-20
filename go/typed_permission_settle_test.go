package asks

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/openabstractions/abstraction-identity/listen"
)

// ASK-A13: a pending typed occurrence leaves with its admissions only when
// the owner's decision settles it; a waiting observer is released; an
// answered occurrence stays; a fresh request admits again.
func TestWithdrawSettledPermission(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := typedFixture(b, false, false)
	record, outcome, err := b.AskTypedPermissionFrom("owner", "app", "key", p, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	if _, _, err := b.AskTypedPermissionFrom("owner", "app", "second-key", p, listen.Seen{}, Publisher{}); err != nil {
		t.Fatal(err)
	}
	if withdrawn, err := b.WithdrawSettledPermission(record.ID, func(TypedPermission) bool { return false }); err != nil || withdrawn {
		t.Fatal("an unsettled request was withdrawn", withdrawn, err)
	}
	released := b.Decided(record.ID)
	var seen TypedPermission
	if withdrawn, err := b.WithdrawSettledPermission(record.ID, func(q TypedPermission) bool { seen = q; return true }); err != nil || !withdrawn || seen.Digest != p.Digest {
		t.Fatal("a settled request stayed", withdrawn, err)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("a waiting observer was not released")
	}
	for _, key := range []string{"key", "second-key"} {
		if _, outcome, err := b.ObserveApplication("owner", key); err != nil || outcome != "unknown" {
			t.Fatalf("admission %s after withdrawal: %s %v", key, outcome, err)
		}
	}
	again, outcome, err := b.AskTypedPermissionFrom("owner", "app", "key", p, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" || again.ID == record.ID {
		t.Fatalf("a fresh request after withdrawal %v %s %v", again.ID, outcome, err)
	}
	if _, _, err := b.AnswerTypedPermissionIfPendingAuthorized(again.ID, "allow", func(Record, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if withdrawn, err := b.WithdrawSettledPermission(again.ID, func(TypedPermission) bool { return true }); err != nil || withdrawn {
		t.Fatal("an answered request was withdrawn", withdrawn, err)
	}
}
