package client

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

// ChoiceClient retains the caller's complete request references unchanged.
// Construct deadlines once in application state. No method refreshes a ref or retries.
type ChoiceClient struct{ transport listen.FrameClient }

func NewChoice(endpoint string) *ChoiceClient {
	return NewChoiceWithTransport(listen.FrameClient{Endpoint: endpoint})
}
func NewChoiceWithTransport(transport listen.FrameClient) *ChoiceClient {
	return &ChoiceClient{transport: transport.WithDefaults(5*time.Second, 1<<20)}
}
func (c *ChoiceClient) WithTimeout(timeout time.Duration) (*ChoiceClient, error) {
	if timeout <= 0 || timeout > 35*time.Second {
		return nil, errors.New("asks: waiting budget must be positive and at most 35s")
	}
	copy := *c
	copy.transport.Timeout = timeout
	return &copy, nil
}
func (c *ChoiceClient) AskContext(ctx context.Context, q ChoiceRequest) (ChoiceObservation, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceObservation{}, err
	}
	if !asks.ValidChoiceRequest(q) {
		return ChoiceObservation{}, errors.New("asks: invalid ordinary choice")
	}
	result, err := wire.NewChoiceApplicationClient(c.transport.WithContext(ctx)).Ask(q)
	result, err = checkedChoice(result, err)
	if err == nil && result.Outcome == ChoiceOutcomeAnswered {
		found := false
		for _, option := range q.Options {
			found = found || option.ID == result.Answer.Option
		}
		if !found {
			return ChoiceObservation{}, errors.New("asks: unoffered choice answer")
		}
	}
	return result, err
}

// ShareContext admits or replays q as AskContext does, sharing it with an
// assistant the person enrolled, which may choose only assistantOptions
// (ASK-A12). The options are part of the reference's immutable content. An
// older runtime answers unknown_method; the caller then decides explicitly
// whether to AskContext unshared.
func (c *ChoiceClient) ShareContext(ctx context.Context, q ChoiceRequest, assistantOptions []string) (ChoiceObservation, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceObservation{}, err
	}
	if !asks.ValidChoiceRequest(q) || !asks.ValidAssistantOptions(q, assistantOptions) {
		return ChoiceObservation{}, errors.New("asks: invalid shared ordinary choice")
	}
	result, err := wire.NewChoiceApplicationClient(c.transport.WithContext(ctx)).ShareChoice(q, assistantOptions)
	result, err = checkedChoice(result, err)
	if err == nil && result.Outcome == ChoiceOutcomeAnswered && !slices.ContainsFunc(q.Options, func(o ChoiceOption) bool { return o.ID == result.Answer.Option }) {
		return ChoiceObservation{}, errors.New("asks: unoffered choice answer")
	}
	return result, err
}
func (c *ChoiceClient) ObserveContext(ctx context.Context, ref RequestRef, waitMS int64) (ChoiceObservation, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceObservation{}, err
	}
	if !asks.ValidChoiceReference(ref) || waitMS < 0 || waitMS > 30000 {
		return ChoiceObservation{}, errors.New("asks: invalid ordinary observation")
	}
	return checkedChoice(wire.NewChoiceApplicationClient(c.transport.WithContext(ctx)).Observe(ref, waitMS))
}
func (c *ChoiceClient) AcknowledgeContext(ctx context.Context, ref RequestRef, answer ChoiceAnswer) (ChoiceAcknowledgement, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceAcknowledgement{}, err
	}
	return wire.NewChoiceApplicationClient(c.transport.WithContext(ctx)).Acknowledge(ref, answer)
}

// AnswerContext conditionally records the application's own choice. A terminal
// decision conflicts, including the same option. No method retries automatically.
func (c *ChoiceClient) AnswerContext(ctx context.Context, ref RequestRef, option string) (ChoiceDecision, error) {
	return c.answer(ctx, ref, option, true)
}

// ReconcileAnswerContext explicitly replays a possibly landed answer while
// retaining the original answering program and timestamp.
func (c *ChoiceClient) ReconcileAnswerContext(ctx context.Context, ref RequestRef, option string) (ChoiceDecision, error) {
	return c.answer(ctx, ref, option, false)
}
func (c *ChoiceClient) answer(ctx context.Context, ref RequestRef, option string, ifPending bool) (ChoiceDecision, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceDecision{}, err
	}
	if !asks.ValidChoiceReference(ref) || !asks.ValidChoiceOptionID(option) {
		return ChoiceDecision{}, errors.New("asks: invalid ordinary answer")
	}
	result, err := wire.NewChoiceApplicationClient(c.transport.WithContext(ctx)).Answer(ref, option, ifPending)
	return checkedOwnChoiceDecision(result, err, ref, option, false)
}
func (c *ChoiceClient) DismissContext(ctx context.Context, ref RequestRef) (ChoiceDecision, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceDecision{}, err
	}
	if !asks.ValidChoiceReference(ref) {
		return ChoiceDecision{}, errors.New("asks: invalid ordinary dismissal")
	}
	result, err := wire.NewChoiceApplicationClient(c.transport.WithContext(ctx)).Dismiss(ref)
	return checkedOwnChoiceDecision(result, err, ref, "", true)
}
func checkedOwnChoiceDecision(result ChoiceDecision, err error, ref RequestRef, option string, dismiss bool) (ChoiceDecision, error) {
	if err != nil {
		return ChoiceDecision{}, err
	}
	id := ""
	if r := result.Record; r != nil {
		if r.Request.RequestRef != ref {
			return ChoiceDecision{}, errors.New("asks: mismatched ordinary reference")
		}
		id = r.ID
	}
	return checkedChoiceDecision(result, nil, id, option, dismiss)
}
func checkedChoice(result ChoiceObservation, err error) (ChoiceObservation, error) {
	if err != nil {
		return ChoiceObservation{}, err
	}
	pending := result.Outcome == ChoiceOutcomePending
	answered, dismissed := result.Outcome == ChoiceOutcomeAnswered, result.Outcome == ChoiceOutcomeDismissed
	if (pending || answered || dismissed) != (result.ID != "") || (answered || dismissed) != (result.Answer != nil) || result.ID != "" && !asks.ValidRequestKey(result.ID) {
		return ChoiceObservation{}, errors.New("asks: inconsistent ordinary observation")
	}
	if a := result.Answer; a != nil && (!asks.ValidChoiceAnswer(*a) || a.ID != result.ID || dismissed && a.Option != "" || answered && !asks.ValidChoiceOptionID(a.Option)) {
		return ChoiceObservation{}, errors.New("asks: inconsistent ordinary answer")
	}
	return result, nil
}

// AnswerChoiceContext is the human UI's conditional submission. A decision
// recorded since its re-read reports conflict, even for the same option.
func (c *Operator) AnswerChoiceContext(ctx context.Context, id, option string) (ChoiceDecision, error) {
	return c.answerChoice(ctx, id, option, true)
}

// ReconcileChoiceAnswerContext explicitly replays a possibly landed decision.
// It preserves the original actor and timestamp and never changes a decision.
func (c *Operator) ReconcileChoiceAnswerContext(ctx context.Context, id, option string) (ChoiceDecision, error) {
	return c.answerChoice(ctx, id, option, false)
}
func (c *Operator) answerChoice(ctx context.Context, id, option string, ifPending bool) (ChoiceDecision, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceDecision{}, err
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).AnswerChoice(id, option, ifPending)
	return checkedChoiceDecision(result, err, id, option, false)
}
func (c *Operator) DismissChoiceContext(ctx context.Context, id string) (ChoiceDecision, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceDecision{}, err
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).DismissChoice(id)
	return checkedChoiceDecision(result, err, id, "", true)
}
func checkedChoiceDecision(result ChoiceDecision, err error, id, option string, dismiss bool) (ChoiceDecision, error) {
	if err != nil {
		return ChoiceDecision{}, err
	}
	terminal := result.Outcome == ChoiceDecisionOutcomeAnswered || result.Outcome == ChoiceDecisionOutcomeDismissed
	if terminal != (result.Record != nil) {
		return ChoiceDecision{}, errors.New("asks: inconsistent ordinary decision")
	}
	if r := result.Record; r != nil && (!asks.ValidChoiceRecord(*r) || r.ID != id || r.Answer == nil || r.Answer.Option != option || result.Outcome.String() != r.State.String() || dismiss != (r.State == ChoiceStateDismissed)) {
		return ChoiceDecision{}, errors.New("asks: mismatched ordinary decision")
	}
	return result, nil
}
func (c *Operator) ReadChoiceContext(ctx context.Context, id string) (ChoiceRead, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceRead{}, err
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).ReadChoice(id)
	if err != nil {
		return ChoiceRead{}, err
	}
	if (result.Outcome == OperatorQuestionOutcomeFound) != (result.Record != nil) || result.Record != nil && (!asks.ValidChoiceRecord(*result.Record) || result.Record.ID != id) {
		return ChoiceRead{}, errors.New("asks: inconsistent ordinary read")
	}
	return result, nil
}

// ReadChoiceDetailsContext reads one ordinary occurrence with its bound
// requester program, shared options and decision route (ASK-O12). An older
// runtime answers unknown_method.
func (c *Operator) ReadChoiceDetailsContext(ctx context.Context, id string) (ChoiceDetails, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceDetails{}, err
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).ReadChoiceDetails(id)
	if err != nil {
		return ChoiceDetails{}, err
	}
	found := result.Choice.Outcome == OperatorQuestionOutcomeFound
	if found != (result.Choice.Record != nil) || result.Choice.Record != nil && (!asks.ValidChoiceRecord(*result.Choice.Record) || result.Choice.Record.ID != id) ||
		!found && (result.AskerProgram != "" || len(result.AssistantOptions) != 0 || result.AnsweredVia != nil) ||
		found && len(result.AssistantOptions) != 0 && !asks.ValidAssistantOptions(result.Choice.Record.Request, result.AssistantOptions) {
		return ChoiceDetails{}, errors.New("asks: inconsistent ordinary details")
	}
	return result, nil
}

// ListAnswersContext pages the answer audit: every retained answer to a
// catalogue question or an ordinary choice recorded through operator@1 or
// responder@1, newest first (ASK-O15). An older runtime answers
// unknown_method.
func (c *Operator) ListAnswersContext(ctx context.Context, cursor string, limit int64) (AnswerEntryPage, error) {
	if err := ctx.Err(); err != nil {
		return AnswerEntryPage{}, err
	}
	if len(cursor) > 256 || limit < 1 || limit > 64 {
		return AnswerEntryPage{}, errors.New("asks: invalid answer audit range")
	}
	page, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).ListAnswers(cursor, limit)
	if err != nil {
		return AnswerEntryPage{}, err
	}
	if page.Outcome != OperatorPageOutcomePage {
		if len(page.Answers) != 0 || page.Next != "" || page.Complete || page.CompleteSinceMs != 0 {
			return AnswerEntryPage{}, errors.New("asks: inconsistent answer audit refusal")
		}
		return page, nil
	}
	if int64(len(page.Answers)) > limit || len(page.Next) > 256 || page.Complete != (page.Next == "") || page.CompleteSinceMs < 0 {
		return AnswerEntryPage{}, errors.New("asks: inconsistent answer audit page")
	}
	for i, a := range page.Answers {
		if !asks.ValidAnswerEntry(a) || i > 0 && a.AnsweredMs > page.Answers[i-1].AnsweredMs {
			return AnswerEntryPage{}, errors.New("asks: invalid audited answer")
		}
	}
	return page, nil
}
func (c *Operator) ListChoicesContext(ctx context.Context, cursor string, limit int64) (ChoicePage, error) {
	if err := ctx.Err(); err != nil {
		return ChoicePage{}, err
	}
	page, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).ListChoices(cursor, limit)
	if err != nil {
		return ChoicePage{}, err
	}
	if page.Outcome != OperatorPageOutcomePage {
		if len(page.Records) != 0 || page.Next != "" || page.Complete {
			return ChoicePage{}, errors.New("asks: inconsistent ordinary page refusal")
		}
		return page, nil
	}
	if int64(len(page.Records)) > limit || len(page.Next) > 256 || page.Complete != (page.Next == "") || !page.Complete && (len(page.Records) == 0 || page.Next == cursor) {
		return ChoicePage{}, errors.New("asks: inconsistent ordinary page")
	}
	ids, used := map[string]bool{}, 2048
	for _, r := range page.Records {
		if !asks.ValidChoiceRecord(r) || ids[r.ID] {
			return ChoicePage{}, errors.New("asks: invalid ordinary record")
		}
		ids[r.ID] = true
		data, e := json.Marshal(r)
		if e != nil {
			return ChoicePage{}, e
		}
		used += len(data) + 2048
		if used > 256<<10 {
			return ChoicePage{}, errors.New("asks: oversized ordinary page")
		}
	}
	return page, nil
}
