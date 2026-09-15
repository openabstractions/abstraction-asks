package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// AuthorizeOperator is trusted host configuration, checked for every operator
// call after receiving same-account Program proof. It may consult rights. A
// successful decision authorizes operator use; the caller supplies the choice.
// Return ErrOperatorForbidden for policy denial; all other callback errors
// report unavailable. The callback is rechecked inside the conditional edit.
type AuthorizeOperator func(context.Context, *identity.Peer) error

var ErrOperatorForbidden = errors.New("asks: operator forbidden")

func operatorError(err error) string {
	if errors.Is(err, ErrOperatorForbidden) {
		return "forbidden"
	}
	return "unavailable"
}

func (h *Host) EnableOperator(authorize AuthorizeOperator) error {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	if h.serving || h.ctx.Err() != nil {
		return errors.New("asks: configure operator before Serve")
	}
	if authorize == nil {
		return errors.New("asks: operator authorization required")
	}
	h.operator = authorize
	return nil
}
func (h *Host) OperatorAvailable() bool {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	return h.operator != nil && h.ctx.Err() == nil
}

type operatorReceiver struct {
	receiver
	ctx context.Context
}

func (r *operatorReceiver) authorize() (string, error) {
	if err := r.ctx.Err(); err != nil {
		return "", err
	}
	scope, _, err := r.scope()
	if err != nil {
		return "", ErrOperatorForbidden
	}
	if r.host.operator == nil {
		return "", ErrOperatorForbidden
	}
	if err = r.ctx.Err(); err != nil {
		return "", err
	}
	peer, err := r.call.Peer()
	if err != nil {
		return "", err
	}
	if err = r.host.operator(r.ctx, peer); err != nil {
		return "", err
	}
	return scope, r.ctx.Err()
}
func operatorMetadata(r asks.Record) wire.RecordMetadata {
	v := wire.RecordMetadata{Id: r.ID, Asker: r.Asker, Key: r.Key, About: r.About, Text: r.Text, Options: append([]string{}, r.Options...), Asked: r.Asked.Format(time.RFC3339Nano), Option: r.Option, Kept: r.Kept, Yes: r.Yes}
	if !r.Answered.IsZero() {
		v.Answered = r.Answered.Format(time.RFC3339Nano)
	}
	return v
}
func pageRefusal(outcome string) wire.OperatorPage {
	return wire.OperatorPage{Outcome: outcome, Records: []wire.RecordMetadata{}}
}
func (r *operatorReceiver) ListQuestions(cursor string, limit int64) (wire.OperatorPage, error) {
	scope, err := r.authorize()
	if err != nil {
		return pageRefusal(operatorError(err)), nil
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return pageRefusal("invalid"), nil
	}
	records, revision, err := r.host.book.ApplicationHistory()
	if err != nil {
		return pageRefusal("unavailable"), nil
	}
	sum := sha256.Sum256([]byte(scope))
	prefix := r.host.operatorEpoch + ":" + hex.EncodeToString(sum[:]) + ":" + revision + ":"
	offset := 0
	if cursor != "" {
		word, ok := strings.CutPrefix(cursor, prefix)
		n, e := strconv.Atoi(word)
		if !ok || e != nil || n < 0 || n > len(records) || strconv.Itoa(n) != word {
			return pageRefusal("gap"), nil
		}
		offset = n
	}
	page := wire.OperatorPage{Outcome: "page", Records: []wire.RecordMetadata{}}
	// Reserve envelope/cursor and pretty-encoding overhead in addition to compact records.
	used := 2048
	for offset < len(records) && int64(len(page.Records)) < limit {
		record := operatorMetadata(records[offset])
		data, e := json.Marshal(record)
		cost := len(data) + 1024 + 32*len(record.Options)
		if e != nil || cost > (256<<10)-2048 {
			return pageRefusal("unavailable"), nil
		}
		if used+cost > 256<<10 {
			break
		}
		used += cost
		page.Records = append(page.Records, record)
		offset++
	}
	page.Complete = offset == len(records)
	if !page.Complete {
		page.Next = prefix + strconv.Itoa(offset)
	}
	// Recheck receiving proof and current authorization before disclosing records.
	if _, err = r.authorize(); err != nil {
		return pageRefusal(operatorError(err)), nil
	}
	return page, nil
}
func (r *operatorReceiver) RetireQuestion(id string) (wire.OperatorRetirement, error) {
	if _, err := r.authorize(); err != nil {
		return wire.OperatorRetirement{Outcome: operatorError(err)}, nil
	}
	record, outcome, err := r.host.book.RetireApplicationAuthorized(id, func() error { _, err := r.authorize(); return err })
	if err != nil {
		return wire.OperatorRetirement{Outcome: operatorError(err)}, nil
	}
	result := wire.OperatorRetirement{Outcome: outcome}
	if outcome == "retired" && record.ID != "" {
		value := operatorMetadata(record)
		result.Record = &value
	}
	return result, nil
}
func (r *operatorReceiver) AnswerQuestion(id, option string) (wire.OperatorDecision, error) {
	if _, err := r.authorize(); err != nil {
		return wire.OperatorDecision{Outcome: operatorError(err)}, nil
	}
	record, outcome, err := r.host.book.AnswerApplicationAuthorized(id, option, func() error { _, err := r.authorize(); return err })
	if err != nil {
		return wire.OperatorDecision{Outcome: operatorError(err)}, nil
	}
	result := wire.OperatorDecision{Outcome: outcome}
	if outcome == "answered" {
		value := operatorMetadata(record)
		result.Record = &value
	}
	return result, nil
}
