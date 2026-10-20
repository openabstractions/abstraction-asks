package asks

import (
	"errors"
	"slices"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
)

// Shared ordinary choices: an enrolled assistant answers the options an
// application shared with it (ASK-A12, ASK-R9), and the Book keeps an audit
// of those answers that acknowledgement leaves in place (ASK-O13).

func choiceCandidate(a choiceAdmission) ChoiceCandidate {
	c := ChoiceCandidate{Record: cloneChoice(*a.Record), AskerProgram: a.AskerProgram, Shared: slices.Clone(a.Shared)}
	if a.Via != nil {
		c.Via = *a.Via
	}
	return c
}

// SharedChoices lists the pending shared choices in admission order, with
// the ordinary revision a responder cursor binds.
func (b *Book) SharedChoices() ([]ChoiceCandidate, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return nil, "", errors.New("asks: application Book required")
	}
	f, err := b.readChoices()
	if err != nil {
		return nil, "", err
	}
	out := []ChoiceCandidate{}
	for _, a := range f.Choices {
		if a.Record != nil && a.Record.State == wire.ChoiceStatePending && len(a.Shared) > 0 {
			out = append(out, choiceCandidate(a))
		}
	}
	return out, choiceFingerprint(f.Choices), nil
}

// AnswerSharedChoice records an enrolled assistant's answer (ASK-R9). The
// Book checks the choice is shared and the option is one of its shared
// options, then calls authorize with the candidate inside the same edit,
// after lock waiting. The same answerer repeating the same option replays
// the recorded answer; every other terminal state is conflict. recorded
// reports whether this call recorded the answer.
func (b *Book) AnswerSharedChoice(id, option, answerer string, authorize func(ChoiceCandidate) error) (result wire.ChoiceDecision, recorded bool, err error) {
	if !b.application || !ValidRequestKey(id) || !ValidChoiceOptionID(option) || !validAnswerer(answerer) || authorize == nil {
		return wire.ChoiceDecision{Outcome: wire.ChoiceDecisionOutcomeInvalid}, false, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	result = wire.ChoiceDecision{Outcome: wire.ChoiceDecisionOutcomeUnknown}
	err = b.changeApplication(func(f *book) error {
		i := f.choiceID(id)
		if i < 0 {
			return nil
		}
		a := &f.Choices[i]
		now := f.choiceTime(b.clock().UnixMilli())
		if now <= 0 {
			return errors.New("asks: ordinary service time unavailable")
		}
		if now >= a.Reference.ReplayUntilMs {
			f.noteChoiceTime(now)
			result.Outcome = wire.ChoiceDecisionOutcomeExpired
			return nil
		}
		if a.Record == nil {
			result.Outcome = wire.ChoiceDecisionOutcomeGone
			return nil
		}
		r := a.Record
		if r.State == wire.ChoiceStateExpired || now >= r.Request.PendingUntilMs && r.State == wire.ChoiceStatePending {
			r.State = wire.ChoiceStateExpired
			f.noteChoiceTime(now)
			result.Outcome = wire.ChoiceDecisionOutcomeExpired
			return nil
		}
		if !slices.Contains(a.Shared, option) {
			return ErrChoiceNotShared
		}
		if e := authorize(choiceCandidate(*a)); e != nil {
			return e
		}
		if r.State != wire.ChoiceStatePending {
			if a.Route != ChoiceRouteResponder || r.State != wire.ChoiceStateAnswered || r.Answer.Option != option || r.Answer.AnsweredBy != answerer {
				result.Outcome = wire.ChoiceDecisionOutcomeConflict
				return nil
			}
		} else {
			r.State = wire.ChoiceStateAnswered
			r.Answer = &wire.ChoiceAnswer{ID: a.ID, Option: option, AnsweredMs: now, AnsweredBy: answerer}
			a.Route = ChoiceRouteResponder
			f.noteChoiceTime(now)
			label := ""
			for _, o := range r.Request.Options {
				if o.ID == option {
					label = o.Label
				}
			}
			f.recordAnswer(wire.AnswerEntry{ID: a.ID, Kind: wire.AnswerKindChoice, Asker: r.Asker, AskerProgram: a.AskerProgram, Publisher: r.Publisher, PublisherTrusted: r.PublisherTrusted,
				Title: r.Request.Title, Option: option, Label: label, AnsweredBy: wire.AnswererResponder, Answerer: answerer}, now)
			recorded = true
		}
		copy := cloneChoice(*r)
		result = wire.ChoiceDecision{Outcome: wire.ChoiceDecisionOutcomeAnswered, Record: &copy}
		return nil
	})
	if err != nil {
		return wire.ChoiceDecision{}, false, err
	}
	if recorded {
		b.settle(id)
	}
	return result, recorded, nil
}

// ChoiceAttribution is a retained ordinary record with the version 7 fields
// the service keeps beside it (ASK-O12).
type ChoiceAttribution struct {
	Record       wire.ChoiceRecord
	AskerProgram string
	Shared       []string
	Route        string
}

// ChoiceAttributed reads one retained ordinary record with its attribution.
func (b *Book) ChoiceAttributed(id string) (ChoiceAttribution, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return ChoiceAttribution{}, false, errors.New("asks: application Book required")
	}
	f, err := b.readChoices()
	if err != nil {
		return ChoiceAttribution{}, false, err
	}
	i := f.choiceID(id)
	if i < 0 || f.Choices[i].Record == nil {
		return ChoiceAttribution{}, false, nil
	}
	a := f.Choices[i]
	return ChoiceAttribution{Record: cloneChoice(*a.Record), AskerProgram: a.AskerProgram, Shared: slices.Clone(a.Shared), Route: a.Route}, true, nil
}

// AuditedAnswers returns the answer audit newest first, the time from which
// it is complete (zero when nothing was evicted) and the revision a cursor
// binds (ASK-O15).
func (b *Book) AuditedAnswers() ([]wire.AnswerEntry, int64, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return nil, 0, "", errors.New("asks: application Book required")
	}
	f, err := b.readChoices()
	if err != nil {
		return nil, 0, "", err
	}
	out := slices.Clone(f.Answers)
	slices.Reverse(out)
	if out == nil {
		out = []wire.AnswerEntry{}
	}
	return out, f.AnswersSinceMs, choiceFingerprint(struct {
		Answers []wire.AnswerEntry
		SinceMs int64
	}{f.Answers, f.AnswersSinceMs}), nil
}
