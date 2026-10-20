package client

import (
	"context"
	"encoding/json"
	"errors"
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Operator struct{ transport listen.FrameClient }

func NewOperator(endpoint string) *Operator {
	return NewOperatorWithTransport(listen.FrameClient{Endpoint: endpoint})
}

// NewOperatorWithTransport retains the caller's endpoint, server trust and waiting limits.
func NewOperatorWithTransport(transport listen.FrameClient) *Operator {
	return &Operator{transport: transport.WithDefaults(5*time.Second, 1<<20)}
}
func (c *Operator) ListQuestionsContext(ctx context.Context, cursor string, limit int64) (wire.OperatorPage, error) {
	if err := ctx.Err(); err != nil {
		return wire.OperatorPage{}, err
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return wire.OperatorPage{}, errors.New("asks: invalid history range")
	}
	page, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).ListQuestions(cursor, limit)
	if err != nil {
		return wire.OperatorPage{}, err
	}
	if page.Outcome != wire.OperatorPageOutcomePage {
		if len(page.Records) != 0 || page.Next != "" || page.Complete {
			return wire.OperatorPage{}, errors.New("asks: malformed history refusal")
		}
		return page, nil
	}
	if int64(len(page.Records)) > limit || len(page.Next) > 256 || page.Complete != (page.Next == "") || (!page.Complete && (len(page.Records) == 0 || page.Next == cursor)) {
		return wire.OperatorPage{}, errors.New("asks: malformed history page")
	}
	ids := map[string]bool{}
	// Reserve envelope/cursor and pretty-encoding overhead in addition to compact records.
	used := 2048
	for _, record := range page.Records {
		if !validOperatorRecord(record) || ids[record.ID] {
			return wire.OperatorPage{}, errors.New("asks: malformed history record")
		}
		data, e := json.Marshal(record)
		if e != nil {
			return wire.OperatorPage{}, e
		}
		used += len(data) + 1024 + 32*len(record.Options)
		if used > 256<<10 {
			return wire.OperatorPage{}, errors.New("asks: oversized history page")
		}
		ids[record.ID] = true
	}
	return page, nil
}

// AnswerQuestionContext never retries. After a transport error, inspect history
// or explicitly retry the same ID/option; the decision may already be recorded.
func (c *Operator) AnswerQuestionContext(ctx context.Context, id, option string) (wire.OperatorDecision, error) {
	if err := ctx.Err(); err != nil {
		return wire.OperatorDecision{}, err
	}
	if !operatorWord(id) || !operatorWord(option) {
		return wire.OperatorDecision{}, errors.New("asks: invalid answer")
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).AnswerQuestion(id, option)
	return checkedDecision(result, err, id, option)
}

// AnswerQuestionIfPendingContext records option only while the question is
// still pending (ASK-O8). An answered question is conflict, the same option
// included. It never retries: after a transport error, inspect history; an
// explicit retry of an answer that landed reads conflict.
func (c *Operator) AnswerQuestionIfPendingContext(ctx context.Context, id, option string) (wire.OperatorDecision, error) {
	if err := ctx.Err(); err != nil {
		return wire.OperatorDecision{}, err
	}
	if !operatorWord(id) || !operatorWord(option) {
		return wire.OperatorDecision{}, errors.New("asks: invalid answer")
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).AnswerQuestionIfPending(id, option)
	return checkedDecision(result, err, id, option)
}

func checkedDecision(result wire.OperatorDecision, err error, id, option string) (wire.OperatorDecision, error) {
	if err != nil {
		return wire.OperatorDecision{}, err
	}
	if (result.Outcome == wire.OperatorDecisionOutcomeAnswered) != (result.Record != nil) {
		return wire.OperatorDecision{}, errors.New("asks: malformed operator result")
	}
	if r := result.Record; r != nil && (!validOperatorRecord(*r) || r.ID != id || r.Option != option) {
		return wire.OperatorDecision{}, errors.New("asks: mismatched operator answer")
	}
	return result, nil
}

// RetireQuestionContext never retries. After a transport error, inspect history
// or explicitly retry the same ID; retirement may already be recorded.
func (c *Operator) RetireQuestionContext(ctx context.Context, id string) (wire.OperatorRetirement, error) {
	if err := ctx.Err(); err != nil {
		return wire.OperatorRetirement{}, err
	}
	if !operatorWord(id) {
		return wire.OperatorRetirement{}, errors.New("asks: invalid retirement")
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).RetireQuestion(id)
	if err != nil {
		return wire.OperatorRetirement{}, err
	}
	if r := result.Record; r != nil && (result.Outcome != wire.OperatorRetirementOutcomeRetired || !validOperatorRecord(*r) || r.ID != id) {
		return wire.OperatorRetirement{}, errors.New("asks: malformed operator retirement")
	}
	return result, nil
}

func (c *Operator) ReadAttributedQuestionContext(ctx context.Context, id string) (wire.AttributedQuestion, error) {
	if err := ctx.Err(); err != nil {
		return wire.AttributedQuestion{}, err
	}
	if !operatorWord(id) {
		return wire.AttributedQuestion{}, errors.New("asks: invalid question id")
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).ReadAttributedQuestion(id)
	if err != nil {
		return wire.AttributedQuestion{}, err
	}
	if err := validateQuestionRead(result.Question, id); err != nil {
		return wire.AttributedQuestion{}, err
	}
	if result.Answerer != "" && (result.Question.AnsweredBy == nil || *result.Question.AnsweredBy != wire.AnswererResponder || !identity.ValidSubjectProgram(result.Answerer)) ||
		result.Question.Outcome != wire.OperatorQuestionOutcomeFound && result.Answerer != "" {
		return wire.AttributedQuestion{}, errors.New("asks: malformed attributed question")
	}
	return result, nil
}

func validateQuestionRead(result wire.OperatorQuestion, id string) error {
	found := result.Outcome == wire.OperatorQuestionOutcomeFound
	program := result.ProgramDescription != "" || result.ProgramOriginalName != ""
	if found != (result.Record != nil) || !found && (result.Publisher != "" || result.PublisherTrusted || result.AnsweredBy != nil || program) ||
		program && !result.PublisherTrusted ||
		found && result.Record.Option == "" && result.AnsweredBy != nil || result.AnsweredBy != nil && !result.AnsweredBy.Known() {
		return errors.New("asks: malformed question read")
	}
	if r := result.Record; r != nil && (!validOperatorRecord(*r) || r.ID != id) {
		return errors.New("asks: mismatched question read")
	}
	return nil
}

func validateDetails(result wire.QuestionDetails, id string) error {
	if err := validateQuestionRead(result.Question.Question, id); err != nil {
		return err
	}
	q := result.Question.Question
	if result.Question.Answerer != "" && (q.AnsweredBy == nil || *q.AnsweredBy != wire.AnswererResponder || !identity.ValidSubjectProgram(result.Question.Answerer)) {
		return errors.New("asks: malformed permission attribution")
	}
	if result.Permission != nil && (q.Outcome != wire.OperatorQuestionOutcomeFound || q.Record.Key != asks.FirstUseKey || !asks.ValidPermissionRequest(*result.Permission)) ||
		q.Outcome == wire.OperatorQuestionOutcomeFound && q.Record.Key == asks.FirstUseKey && result.Permission == nil {
		return errors.New("asks: malformed permission details")
	}
	return nil
}

// ReadQuestionAskerContext reads trusted occurrence details with
// the bound program that asked it (ASK-O11). The record's asker must be the
// program's derived display name; only found carries a program.
func (c *Operator) ReadQuestionAskerContext(ctx context.Context, id string) (wire.QuestionAsker, error) {
	if err := ctx.Err(); err != nil {
		return wire.QuestionAsker{}, err
	}
	if !operatorWord(id) {
		return wire.QuestionAsker{}, errors.New("asks: invalid question id")
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).ReadQuestionAsker(id)
	if err != nil {
		return wire.QuestionAsker{}, err
	}
	if err := validateDetails(result.Details, id); err != nil {
		return wire.QuestionAsker{}, err
	}
	if err := validateAskerProgram(result.Details, result.Program); err != nil {
		return wire.QuestionAsker{}, err
	}
	return result, nil
}

// validateAskerProgram checks a reported asker program against the record it
// came with: a clean absolute path whose display name is the record's asker.
func validateAskerProgram(details wire.QuestionDetails, program string) error {
	if program == "" {
		return nil
	}
	q := details.Question.Question
	if q.Outcome != wire.OperatorQuestionOutcomeFound || q.Record == nil || len(program) > 4096 || !utf8.ValidString(program) ||
		!identity.ValidSubjectProgram(program) || strings.HasPrefix(program, identity.PackagedProgramPrefix) || q.Record.Asker != asks.AskerDisplay(program) {
		return errors.New("asks: malformed asker program")
	}
	return nil
}

func operatorWord(s string) bool {
	return len(s) > 0 && len(s) <= 128 && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}
func validOperatorRecord(r wire.RecordMetadata) bool {
	if !operatorWord(r.ID) || r.Asker == "" || r.Key == "" || r.Text == "" || len(r.Options) == 0 {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, r.Asked); err != nil {
		return false
	}
	if r.Option == "" {
		return r.Answered == "" && !r.Yes && !r.Kept
	}
	if _, err := time.Parse(time.RFC3339Nano, r.Answered); err != nil {
		return false
	}
	for _, option := range r.Options {
		if r.Option == option {
			return true
		}
	}
	return false
}
