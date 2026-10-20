package client

import (
	"context"
	"errors"
	"slices"
	"time"
	"unicode/utf8"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

type Responder struct{ transport listen.FrameClient }

func NewResponder(endpoint string) *Responder {
	return NewResponderWithTransport(listen.FrameClient{Endpoint: endpoint})
}

func NewResponderWithTransport(transport listen.FrameClient) *Responder {
	return &Responder{transport: transport.WithDefaults(5*time.Second, 1<<20)}
}

func (c *Responder) ListAnswerableContext(ctx context.Context, cursor string, limit int64) (wire.OperatorPage, error) {
	if err := ctx.Err(); err != nil {
		return wire.OperatorPage{}, err
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return wire.OperatorPage{}, errors.New("asks: invalid responder range")
	}
	page, err := wire.NewQuestionResponderClient(c.transport.WithContext(ctx)).ListAnswerable(cursor, limit)
	if err != nil {
		return wire.OperatorPage{}, err
	}
	if page.Outcome != wire.OperatorPageOutcomePage {
		if len(page.Records) != 0 || page.Next != "" || page.Complete {
			return wire.OperatorPage{}, errors.New("asks: malformed responder refusal")
		}
		return page, nil
	}
	if int64(len(page.Records)) > limit || len(page.Next) > 256 || page.Complete != (page.Next == "") {
		return wire.OperatorPage{}, errors.New("asks: malformed responder page")
	}
	for _, record := range page.Records {
		if !validOperatorRecord(record) || record.Option != "" {
			return wire.OperatorPage{}, errors.New("asks: malformed answerable record")
		}
	}
	return page, nil
}

// RequestAnsweringContext asks for this caller's right to answer for the
// person (ASK-R10): permitted when it holds a current enrollment; not_granted
// when the person is asked through the native permission prompt; forbidden
// for a script host, an operator program or another account. An older
// runtime answers unknown_method.
func (c *Responder) RequestAnsweringContext(ctx context.Context) (Decision, error) {
	if err := ctx.Err(); err != nil {
		return Decision{}, err
	}
	decision, err := wire.NewQuestionResponderClient(c.transport.WithContext(ctx)).RequestAnswering()
	if err != nil {
		return Decision{}, err
	}
	if decision.Outcome.String() == "" {
		return Decision{}, errors.New("asks: malformed answering decision")
	}
	return decision, nil
}

// ListChoicesContext lists the pending choices applications shared with an
// assistant that this caller may answer now, each with only its shared
// options (ASK-R9). An older runtime answers unknown_method.
func (c *Responder) ListChoicesContext(ctx context.Context, cursor string, limit int64) (wire.AnswerableChoicePage, error) {
	if err := ctx.Err(); err != nil {
		return wire.AnswerableChoicePage{}, err
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return wire.AnswerableChoicePage{}, errors.New("asks: invalid responder range")
	}
	page, err := wire.NewQuestionResponderClient(c.transport.WithContext(ctx)).ListChoices(cursor, limit)
	if err != nil {
		return wire.AnswerableChoicePage{}, err
	}
	if page.Outcome != wire.OperatorPageOutcomePage {
		if len(page.Choices) != 0 || page.Next != "" || page.Complete {
			return wire.AnswerableChoicePage{}, errors.New("asks: malformed responder refusal")
		}
		return page, nil
	}
	if int64(len(page.Choices)) > limit || len(page.Next) > 256 || page.Complete != (page.Next == "") {
		return wire.AnswerableChoicePage{}, errors.New("asks: malformed responder page")
	}
	for _, choice := range page.Choices {
		if !asks.ValidRequestKey(choice.ID) || choice.Asker == "" || choice.PendingUntilMs <= 0 || len(choice.Options) < 1 || len(choice.Options) > 3 {
			return wire.AnswerableChoicePage{}, errors.New("asks: malformed answerable choice")
		}
		for i, o := range choice.Options {
			if !asks.ValidChoiceOptionID(o.ID) || slices.ContainsFunc(choice.Options[:i], func(p wire.ChoiceOption) bool { return p.ID == o.ID }) {
				return wire.AnswerableChoicePage{}, errors.New("asks: malformed answerable choice")
			}
		}
	}
	return page, nil
}

// AnswerChoiceContext records a shared option for a pending shared choice
// (ASK-R9). Repeating the same option after a lost reply replays the
// recorded answer; no method retries automatically.
func (c *Responder) AnswerChoiceContext(ctx context.Context, id, option string) (wire.ChoiceDecision, error) {
	if err := ctx.Err(); err != nil {
		return wire.ChoiceDecision{}, err
	}
	if !asks.ValidRequestKey(id) || !asks.ValidChoiceOptionID(option) {
		return wire.ChoiceDecision{}, errors.New("asks: invalid shared choice answer")
	}
	result, err := wire.NewQuestionResponderClient(c.transport.WithContext(ctx)).AnswerChoice(id, option)
	result, err = checkedChoiceDecision(result, err, id, option, false)
	if err == nil && result.Outcome == wire.ChoiceDecisionOutcomeDismissed {
		return wire.ChoiceDecision{}, errors.New("asks: an assistant cannot dismiss")
	}
	return result, err
}

func (c *Responder) AnswerContext(ctx context.Context, id, option string) (wire.ResponderDecision, error) {
	if err := ctx.Err(); err != nil {
		return wire.ResponderDecision{}, err
	}
	if !operatorWord(id) || !operatorWord(option) {
		return wire.ResponderDecision{}, errors.New("asks: invalid delegated answer")
	}
	result, err := wire.NewQuestionResponderClient(c.transport.WithContext(ctx)).Answer(id, option)
	if err != nil {
		return wire.ResponderDecision{}, err
	}
	if (result.Outcome == wire.OperatorDecisionOutcomeAnswered) != (result.Record != nil) {
		return wire.ResponderDecision{}, errors.New("asks: malformed delegated answer")
	}
	if result.Record != nil && (!validOperatorRecord(*result.Record) || result.Record.ID != id || result.Record.Option != option) {
		return wire.ResponderDecision{}, errors.New("asks: mismatched delegated answer")
	}
	return result, nil
}
