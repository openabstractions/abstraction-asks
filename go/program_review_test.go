package asks

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openabstractions/abstraction-identity/listen"
)

func TestProgramReviewWithdrawalSelectsOnlyItsOwnPendingOccurrence(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "application.json"))
	if err != nil {
		t.Fatal(err)
	}
	slots, err := ProgramReviewSlots(ProgramReviewFacts{Targets: []ProgramReviewTarget{{Program: `C:\fixture\provider.exe`, Revision: "revision", Image: strings.Repeat("a", 64), FileKey: "native-file", Spec: strings.Repeat("b", 64), Cause: "shipped:unrecorded", Supported: true}}})
	if err != nil {
		t.Fatal(err)
	}
	ask := func(key, program string) Record {
		t.Helper()
		var record Record
		var outcome string
		var err error
		if program == "" {
			record, outcome, err = b.AskApplicationFrom("runtime", "runtime", key, ProgramReviewKey, slots, listen.Seen{}, Publisher{})
		} else {
			record, outcome, err = b.AskApplicationAs("runtime", program, key, ProgramReviewKey, slots, listen.Seen{}, Publisher{})
		}
		if err != nil || outcome != "pending" {
			t.Fatal(outcome, err)
		}
		return record
	}
	stale := ask("group-stale", "")
	settled := b.Decided(stale.ID)
	if removed, err := b.WithdrawProgramReview(stale.ID); err != nil || !removed {
		t.Fatal(removed, err)
	}
	select {
	case <-settled:
	default:
		t.Fatal("withdrawal did not settle native presenter")
	}
	if _, outcome, err := b.AnswerApplication(stale.ID, "keep"); err != nil || outcome == "answered" {
		t.Fatal("withdrawn group answered", outcome, err)
	}
	answered := ask("group-answered", "")
	if _, outcome, err := b.AnswerApplication(answered.ID, "not_now"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	if removed, err := b.WithdrawProgramReview(answered.ID); err != nil || removed {
		t.Fatal("answered history removed", removed, err)
	}
	if record, found, err := b.ApplicationRecord(answered.ID); err != nil || !found || record.Option != "not_now" {
		t.Fatal(record, found, err)
	}
	foreign := ask("group-application", filepath.Join(t.TempDir(), "foreign.exe"))
	if removed, err := b.WithdrawProgramReview(foreign.ID); err != nil || removed {
		t.Fatal("app-origin occurrence withdrawn", removed, err)
	}
	if record, found, err := b.ApplicationRecord(foreign.ID); err != nil || !found || !record.Pending() {
		t.Fatal(record, found, err)
	}
	ordinary, outcome, err := b.AskApplication("runtime", "runtime", "ordinary", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	if removed, err := b.WithdrawProgramReview(ordinary.ID); err != nil || removed {
		t.Fatal("ordinary occurrence withdrawn", removed, err)
	}
}

func TestProgramReviewPredecessorRetirementKeepsOtherAuthorityAndHistory(t *testing.T) {
	for _, mode := range []string{"current", "app-origin", "other-program", "answered", "revision-conflict", "other-key"} {
		t.Run(mode, func(t *testing.T) {
			b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "application.json"))
			if err != nil {
				t.Fatal(err)
			}
			program := `C:\fixture\provider.exe`
			slots := map[string]string{"program": program, "title": "Provider"}
			key := SelfRegisteredKey
			if mode == "other-program" {
				slots["program"] = `C:\fixture\other.exe`
			}
			if mode == "other-key" {
				key = "download.reach"
				slots = map[string]string{"host": "example.com"}
			}
			var record Record
			var outcome string
			if mode == "app-origin" {
				record, outcome, err = b.AskApplicationAs("runtime", filepath.Join(t.TempDir(), "caller.exe"), "prior", key, slots, listen.Seen{}, Publisher{})
			} else {
				record, outcome, err = b.AskApplicationFrom("runtime", "runtime", "prior", key, slots, listen.Seen{}, Publisher{})
			}
			if err != nil || outcome != "pending" {
				t.Fatal(outcome, err)
			}
			if mode == "answered" {
				if _, outcome, err := b.AnswerApplication(record.ID, "keep"); err != nil || outcome != "answered" {
					t.Fatal(outcome, err)
				}
			}
			removed, err := b.WithdrawProgramReviewPredecessor(record.ID, program, func() error {
				if mode == "revision-conflict" {
					return errors.New("current approval revision changed")
				}
				return nil
			})
			if mode == "current" {
				if err != nil || !removed {
					t.Fatal(removed, err)
				}
				return
			}
			if removed || mode != "revision-conflict" && err != nil {
				t.Fatal("unrelated authority retired", removed, err)
			}
			if current, found, err := b.ApplicationRecord(record.ID); err != nil || !found || mode == "answered" && current.Option != "keep" {
				t.Fatal(current, found, err)
			}
		})
	}
}
