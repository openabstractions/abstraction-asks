package asks

import (
	"testing"
	"time"
)

// A question that leaves the Book settles as expired when expiry removed
// it, withdrawn otherwise; an answer settles by its option.
func TestQuestionEventsOutcomes(t *testing.T) {
	asked := time.Now().UTC()
	before := &book{Asks: []*Record{{ID: "a", Asked: asked}, {ID: "b", Asked: asked}, {ID: "c", Asked: asked}, {ID: "d", Asked: asked}}}
	ids, pending := pendingQuestions(before)
	after := &book{Asks: []*Record{
		{ID: "a", Asked: asked, Option: "once", Yes: true, Answered: asked.Add(time.Second)},
		{ID: "b", Asked: asked, Option: "never", Kept: true, Answered: asked.Add(time.Second)},
		{ID: "e", Asked: asked},
	}}
	changes := diffQuestions(after, ids, pending, []string{"c"})
	if len(changes.raised) != 1 || changes.raised[0].ID != "e" {
		t.Fatalf("raised %+v", changes.raised)
	}
	got := map[string]string{}
	for _, s := range changes.settled {
		got[s.record.ID] = s.outcome
	}
	want := map[string]string{"a": SettledAllowedOnce, "b": SettledBlocked, "c": SettledExpired, "d": SettledWithdrawn}
	if len(got) != len(want) {
		t.Fatalf("settled %+v", got)
	}
	for id, outcome := range want {
		if got[id] != outcome {
			t.Fatalf("settled %+v", got)
		}
	}
	if SettledOutcome(Record{Option: "allow", Yes: true}) != SettledAllowed || SettledOutcome(Record{Option: "not_now"}) != SettledDeclined {
		t.Fatal("allow and not now")
	}
}
