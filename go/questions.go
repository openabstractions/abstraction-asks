package asks

import (
	"errors"
	"github.com/openabstractions/abstraction-asks/go/catalogue"
)

type Option = catalogue.Option
type Question struct {
	Key     string
	Text    string
	About   string
	Options []Option
	// Closed offers exactly Options, without the Never every other catalogue
	// question adds.
	Closed bool
	// RuntimeBound marks a question whose answer acts only through state held
	// in the memory of the runtime that admitted it (ASK-S1: the protected
	// operation's ticket). A runtime that has just bound its endpoints
	// withdraws every pending one (Book.WithdrawRuntimeBound), so none is
	// answered after the state it was bound to is gone.
	RuntimeBound bool
}

var ErrUnknownQuestion = catalogue.ErrUnknownQuestion
var ErrUnknownOption = catalogue.ErrUnknownOption
var ErrBadSlot = catalogue.ErrBadSlot
var Never = catalogue.Never
var Questions = func() []Question {
	out := make([]Question, len(catalogue.Questions))
	for i, q := range catalogue.Questions {
		out[i] = Question(q)
	}
	return out
}()

const maxSlot = catalogue.MaxSlot

func validQuestionSlot(value string) bool { return catalogue.ValidQuestionSlot(value) }

const SelfRegisteredKey = catalogue.SelfRegisteredKey
const AgentEnrollmentKey = catalogue.AgentEnrollmentKey
const AgentUpdateKey = catalogue.AgentUpdateKey
const SetupModelsKey = catalogue.SetupModelsKey
const ProgramReviewKey = catalogue.ProgramReviewKey

type ProgramReviewFacts = catalogue.ProgramReviewFacts
type ProgramReviewTarget = catalogue.ProgramReviewTarget

func ProgramReviewSlots(f ProgramReviewFacts) (map[string]string, error) {
	return catalogue.ProgramReviewSlots(f)
}
func ProgramReviewFactsFromSlots(slots map[string]string) (ProgramReviewFacts, error) {
	return catalogue.ProgramReviewFactsFromSlots(slots)
}
func ProgramReviewOptions(slots map[string]string) []string {
	return catalogue.ProgramReviewOptions(slots)
}

// ProgramReviewUnsignedChanged is catalogue.ProgramReviewUnsignedChanged.
const ProgramReviewUnsignedChanged = catalogue.ProgramReviewUnsignedChanged

func ProgramReviewUnsigned(slots map[string]string) (string, bool) {
	return catalogue.ProgramReviewUnsigned(slots)
}
func ProgramReviewPresentation(slots map[string]string) (title, instruction, details string, err error) {
	return catalogue.ProgramReviewPresentation(slots)
}

const FirstUseKey = catalogue.FirstUseKey
const SystemSecurityLogKey = catalogue.SystemSecurityLogKey
const PairingKey = catalogue.PairingKey
const ApplicationCommandOnceKey = catalogue.ApplicationCommandOnceKey

func Find(key string) (Question, bool) { q, ok := catalogue.Find(key); return Question(q), ok }
func SelfRegistered(key, about, text string) (string, string, bool) {
	return catalogue.SelfRegistered(key, about, text)
}
func FirstUse(key, about, text string) (string, string, string, bool) {
	return catalogue.FirstUse(key, about, text)
}
func RuntimeOnlyKey(key string) bool                 { return catalogue.RuntimeOnlyKey(key) }
func (q Question) Option(name string) (Option, bool) { return catalogue.Question(q).Option(name) }
func (q Question) OptionNames() []string             { return catalogue.Question(q).OptionNames() }
func (q Question) Render(a Ask) (string, error) {
	return catalogue.Question(q).Render(catalogue.Ask{Asker: a.Asker, Key: a.Key, Slots: a.Slots})
}
func (q Question) Summary(about string) string { return catalogue.Question(q).Summary(about) }

// WithdrawProgramReview retires one service-owned in-memory review occurrence.
// It has no IPC surface and accepts neither application-origin nor other keys.
// Answered history is retained; a withdrawn pending occurrence cannot answer.
func (b *Book) WithdrawProgramReview(id string) (bool, error) {
	if id == "" {
		return false, errors.New("asks: runtime review occurrence required")
	}
	record, outcome, err := b.retireApplicationSelected(id, nil, true, func(record Record) bool {
		q, ok := Find(record.Key)
		return record.Key == ProgramReviewKey && record.AskerProgram == "" && ok && q.RuntimeBound && RuntimeOnlyKey(q.Key)
	})
	return record.ID == id && outcome == "retired", err
}

// WithdrawProgramReviewPredecessor retires only the exact native PROGRAM
// question whose current directory approval the service rechecks atomically.
func (b *Book) WithdrawProgramReviewPredecessor(id, programSlot string, authorize func() error) (bool, error) {
	if id == "" || programSlot == "" || authorize == nil {
		return false, errors.New("asks: current program review custody required")
	}
	record, outcome, err := b.retireApplicationSelected(id, authorize, true, func(record Record) bool {
		q, ok := Find(record.Key)
		return ok && record.Key == SelfRegisteredKey && RuntimeOnlyKey(q.Key) && record.AskerProgram == "" && record.Content != nil && record.About == programSlot && record.Content.Slots["program"] == programSlot
	})
	return record.ID == id && outcome == "retired", err
}

func (b *Book) WithdrawRuntimeBound() ([]Record, error) {
	if !b.application {
		return nil, errors.New("asks: application Book required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var withdrawn []Record
	unchanged := errors.New("asks: nothing runtime-bound pending")
	err := b.changeApplication(func(f *book) error {
		kept := f.Asks[:0]
		for _, r := range f.Asks {
			if q, ok := Find(r.Key); ok && q.RuntimeBound && r.Pending() {
				withdrawn = append(withdrawn, *r)
				continue
			}
			kept = append(kept, r)
		}
		if len(withdrawn) == 0 {
			return unchanged
		}
		f.Asks = kept
		return nil
	})
	if errors.Is(err, unchanged) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, r := range withdrawn {
		b.settle(r.ID)
	}
	return withdrawn, nil
}
