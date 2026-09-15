package client

import (
	"context"
	"encoding/json"
	"errors"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
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
	if page.Outcome != "page" {
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
		if !validOperatorRecord(record) || ids[record.Id] {
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
		ids[record.Id] = true
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
	if err != nil {
		return wire.OperatorDecision{}, err
	}
	if (result.Outcome == "answered") != (result.Record != nil) {
		return wire.OperatorDecision{}, errors.New("asks: malformed operator result")
	}
	if r := result.Record; r != nil && (!validOperatorRecord(*r) || r.Id != id || r.Option != option) {
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
	if r := result.Record; r != nil && (result.Outcome != "retired" || !validOperatorRecord(*r) || r.Id != id) {
		return wire.OperatorRetirement{}, errors.New("asks: malformed operator retirement")
	}
	return result, nil
}
func operatorWord(s string) bool {
	return len(s) > 0 && len(s) <= 128 && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}
func validOperatorRecord(r wire.RecordMetadata) bool {
	if !operatorWord(r.Id) || r.Asker == "" || r.Key == "" || r.Text == "" || len(r.Options) == 0 {
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
