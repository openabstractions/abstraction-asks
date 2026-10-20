package asks

import "time"

// The outcomes a settled question records (QuestionEvents).
const (
	SettledAllowed     = "allowed"
	SettledAllowedOnce = "allowed once"
	SettledDeclined    = "declined"
	SettledBlocked     = "blocked"
	SettledExpired     = "expired"
	SettledWithdrawn   = "withdrawn"
)

// QuestionEvents receives each question an application Book raises and each
// one it settles, after the edit that did so is stored. Raised gets the new
// record. Settled gets the record as it was last kept, its outcome and when it
// settled. The callbacks run under the Book's lock and must not call the Book.
type QuestionEvents struct {
	Raised  func(Record)
	Settled func(r Record, outcome string, at time.Time)
}

// SetQuestionEvents installs events on this Book; nil callbacks are skipped.
func (b *Book) SetQuestionEvents(events QuestionEvents) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = events
}

// SettledOutcome is the outcome of an answered record: Once is allowed once,
// Never blocked, any other yes allowed, and any other no declined.
func SettledOutcome(r Record) string {
	switch {
	case r.Option == "once":
		return SettledAllowedOnce
	case r.Option == Never.Name:
		return SettledBlocked
	case r.Yes:
		return SettledAllowed
	}
	return SettledDeclined
}

// questionChanges is what one Book edit raised and settled.
type questionChanges struct {
	raised  []Record
	settled []settledQuestion
}

type settledQuestion struct {
	record  Record
	outcome string
}

// pendingQuestions snapshots the questions of f before an edit.
func pendingQuestions(f *book) (ids map[string]bool, pending map[string]Record) {
	ids, pending = map[string]bool{}, map[string]Record{}
	for _, r := range f.Asks {
		ids[r.ID] = true
		if r.Pending() {
			pending[r.ID] = *r
		}
	}
	return ids, pending
}

// diffQuestions compares f after an edit with its snapshot. A pending
// question that left the Book expired when expired names it, and was
// withdrawn otherwise.
func diffQuestions(f *book, ids map[string]bool, pending map[string]Record, expired []string) questionChanges {
	var out questionChanges
	present := map[string]bool{}
	for _, r := range f.Asks {
		present[r.ID] = true
		if !ids[r.ID] {
			out.raised = append(out.raised, *r)
		}
		if _, was := pending[r.ID]; (was || !ids[r.ID]) && !r.Pending() {
			out.settled = append(out.settled, settledQuestion{*r, SettledOutcome(*r)})
		}
	}
	gone := map[string]bool{}
	for _, id := range expired {
		gone[id] = true
	}
	for id, r := range pending {
		if present[id] {
			continue
		}
		outcome := SettledWithdrawn
		if gone[id] {
			outcome = SettledExpired
		}
		out.settled = append(out.settled, settledQuestion{r, outcome})
	}
	return out
}

// emit runs the events for one stored edit.
func (b *Book) emit(changes questionChanges) {
	if b.events.Raised != nil {
		for _, r := range changes.raised {
			b.events.Raised(r)
		}
	}
	if b.events.Settled != nil {
		now := b.clock().UTC()
		for _, s := range changes.settled {
			at := now
			if !s.record.Answered.IsZero() {
				at = s.record.Answered
			}
			b.events.Settled(s.record, s.outcome, at)
		}
	}
}
