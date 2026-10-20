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

func operatorPageError(err error) wire.OperatorPageOutcome {
	if errors.Is(err, ErrOperatorForbidden) {
		return wire.OperatorPageOutcomeForbidden
	}
	return wire.OperatorPageOutcomeUnavailable
}
func operatorDecisionError(err error) wire.OperatorDecisionOutcome {
	if errors.Is(err, ErrOperatorForbidden) {
		return wire.OperatorDecisionOutcomeForbidden
	}
	return wire.OperatorDecisionOutcomeUnavailable
}
func operatorRetirementError(err error) wire.OperatorRetirementOutcome {
	if errors.Is(err, ErrOperatorForbidden) {
		return wire.OperatorRetirementOutcomeForbidden
	}
	return wire.OperatorRetirementOutcomeUnavailable
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
	v := wire.RecordMetadata{ID: r.ID, Asker: r.Asker, Key: r.Key, About: r.About, Text: r.Text, Options: append([]string{}, r.Options...), Asked: r.Asked.Format(time.RFC3339Nano), Option: r.Option, Kept: r.Kept, Yes: r.Yes}
	if !r.Answered.IsZero() {
		v.Answered = r.Answered.Format(time.RFC3339Nano)
	}
	return v
}
func pageRefusal(outcome wire.OperatorPageOutcome) wire.OperatorPage {
	return wire.OperatorPage{Outcome: outcome, Records: []wire.RecordMetadata{}}
}
func (r *operatorReceiver) ListQuestions(cursor string, limit int64) (wire.OperatorPage, error) {
	return r.listQuestions(cursor, limit, false)
}
func (r *operatorReceiver) listQuestions(cursor string, limit int64, typed bool) (wire.OperatorPage, error) {
	scope, err := r.authorize()
	if err != nil {
		return pageRefusal(operatorPageError(err)), nil
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return pageRefusal(wire.OperatorPageOutcomeInvalid), nil
	}
	records, revision, err := r.host.book.ApplicationHistory()
	if err != nil {
		return pageRefusal(wire.OperatorPageOutcomeUnavailable), nil
	}
	filtered := records[:0]
	for _, record := range records {
		if (record.TypedPermission != nil) == typed {
			filtered = append(filtered, record)
		}
	}
	records = filtered
	sum := sha256.Sum256([]byte(scope))
	prefix := r.host.operatorEpoch + ":" + hex.EncodeToString(sum[:]) + ":" + revision + ":"
	if typed {
		prefix = "typed:" + prefix
	}
	offset := 0
	if cursor != "" {
		word, ok := strings.CutPrefix(cursor, prefix)
		n, e := strconv.Atoi(word)
		if !ok || e != nil || n < 0 || n > len(records) || strconv.Itoa(n) != word {
			return pageRefusal(wire.OperatorPageOutcomeGap), nil
		}
		offset = n
	}
	page := wire.OperatorPage{Outcome: wire.OperatorPageOutcomePage, Records: []wire.RecordMetadata{}}
	// Reserve envelope/cursor and pretty-encoding overhead in addition to compact records.
	used := 2048
	for offset < len(records) && int64(len(page.Records)) < limit {
		record := operatorMetadata(records[offset])
		data, e := json.Marshal(record)
		cost := len(data) + 1024 + 32*len(record.Options)
		if e != nil || cost > (256<<10)-2048 {
			return pageRefusal(wire.OperatorPageOutcomeUnavailable), nil
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
		return pageRefusal(operatorPageError(err)), nil
	}
	return page, nil
}
func (r *operatorReceiver) RetireQuestion(id string) (wire.OperatorRetirement, error) {
	if _, err := r.authorize(); err != nil {
		return wire.OperatorRetirement{Outcome: operatorRetirementError(err)}, nil
	}
	if record, found, err := r.host.book.ApplicationRecord(id); err != nil {
		return wire.OperatorRetirement{Outcome: wire.OperatorRetirementOutcomeUnavailable}, nil
	} else if found && record.TypedPermission != nil {
		return wire.OperatorRetirement{Outcome: wire.OperatorRetirementOutcomeInvalid}, nil
	}
	record, outcome, err := r.host.book.RetireApplicationAuthorized(id, func() error { _, err := r.authorize(); return err })
	if err != nil {
		return wire.OperatorRetirement{Outcome: operatorRetirementError(err)}, nil
	}
	parsed, ok := wire.ParseOperatorRetirementOutcome(outcome)
	if !ok {
		parsed = wire.OperatorRetirementOutcomeUnavailable
	}
	result := wire.OperatorRetirement{Outcome: parsed}
	if parsed == wire.OperatorRetirementOutcomeRetired && record.ID != "" {
		value := operatorMetadata(record)
		result.Record = &value
	}
	return result, nil
}

// ReadQuestionAsker reads trusted details and the bound asking program from
// the same occurrence (ASK-O11). Only found carries a program.
func (r *operatorReceiver) ReadQuestionAsker(id string) (wire.QuestionAsker, error) {
	details, record := r.readDetails(id)
	return wire.QuestionAsker{Details: details, Program: record.AskerProgram}, nil
}

// readDetails reads one occurrence for an authorized operator. The record is
// the zero value unless the outcome is found.
func (r *operatorReceiver) readDetails(id string) (wire.QuestionDetails, asks.Record) {
	refused := func(outcome wire.OperatorQuestionOutcome) (wire.QuestionDetails, asks.Record) {
		return wire.QuestionDetails{Question: wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: outcome}}}, asks.Record{}
	}
	if _, err := r.authorize(); err != nil {
		return refused(operatorQuestionError(err))
	}
	if !asks.ValidRequestKey(id) {
		return refused(wire.OperatorQuestionOutcomeInvalid)
	}
	record, found, err := r.host.book.ApplicationRecord(id)
	if err != nil {
		return refused(wire.OperatorQuestionOutcomeUnavailable)
	}
	if !found {
		return refused(wire.OperatorQuestionOutcomeUnknown)
	}
	if _, err := r.authorize(); err != nil {
		return refused(operatorQuestionError(err))
	}
	result := wire.QuestionDetails{Question: attributedRecord(record)}
	if record.TypedPermission != nil {
		return refused(wire.OperatorQuestionOutcomeInvalid)
	}
	if record.Key == asks.FirstUseKey {
		permission, ok := asks.PermissionForRecord(r.host.owner, record)
		if !ok {
			return refused(wire.OperatorQuestionOutcomeUnavailable)
		}
		result.Permission = &permission
	}
	return result, record
}

// ReadAttributedQuestion reads recorded metadata and provenance (ASK-O7).
// Permission recovery belongs to ReadQuestionAsker.
func (r *operatorReceiver) ReadAttributedQuestion(id string) (wire.AttributedQuestion, error) {
	if _, err := r.authorize(); err != nil {
		return wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: operatorQuestionError(err)}}, nil
	}
	if !asks.ValidRequestKey(id) {
		return wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: wire.OperatorQuestionOutcomeInvalid}}, nil
	}
	record, found, err := r.host.book.ApplicationRecord(id)
	if err != nil {
		return wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: wire.OperatorQuestionOutcomeUnavailable}}, nil
	}
	if !found {
		return wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: wire.OperatorQuestionOutcomeUnknown}}, nil
	}
	if _, err = r.authorize(); err != nil {
		return wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: operatorQuestionError(err)}}, nil
	}
	if record.TypedPermission != nil {
		return wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: wire.OperatorQuestionOutcomeInvalid}}, nil
	}
	return attributedRecord(record), nil
}

func attributedRecord(record asks.Record) wire.AttributedQuestion {
	value := operatorMetadata(record)
	read := wire.OperatorQuestion{Outcome: wire.OperatorQuestionOutcomeFound, Record: &value,
		Publisher: record.Publisher, PublisherTrusted: record.PublisherTrusted,
		ProgramDescription: record.ProgramDescription, ProgramOriginalName: record.ProgramOriginalName}
	if by, ok := wire.ParseAnswerer(record.AnsweredBy); ok && !record.Pending() {
		read.AnsweredBy = &by
	}
	return wire.AttributedQuestion{Question: read, Answerer: record.Answerer}
}
func operatorQuestionError(err error) wire.OperatorQuestionOutcome {
	if errors.Is(err, ErrOperatorForbidden) {
		return wire.OperatorQuestionOutcomeForbidden
	}
	return wire.OperatorQuestionOutcomeUnavailable
}
func (r *operatorReceiver) AnswerQuestion(id, option string) (wire.OperatorDecision, error) {
	return r.answer(id, option, func(id, option string, authorize func() error) (asks.Record, string, error) {
		return r.host.book.AnswerLegacyApplicationAuthorized(id, option, false, authorize)
	})
}

// AnswerQuestionIfPending records option only while the question is pending
// (ASK-O8); an answered question is conflict and nothing is replayed.
func (r *operatorReceiver) AnswerQuestionIfPending(id, option string) (wire.OperatorDecision, error) {
	return r.answer(id, option, func(id, option string, authorize func() error) (asks.Record, string, error) {
		return r.host.book.AnswerLegacyApplicationAuthorized(id, option, true, authorize)
	})
}

func (r *operatorReceiver) answer(id, option string, record func(id, option string, authorize func() error) (asks.Record, string, error)) (wire.OperatorDecision, error) {
	if _, err := r.authorize(); err != nil {
		return wire.OperatorDecision{Outcome: operatorDecisionError(err)}, nil
	}
	answered, outcome, err := record(id, option, func() error { _, err := r.authorize(); return err })
	if err != nil {
		return wire.OperatorDecision{Outcome: operatorDecisionError(err)}, nil
	}
	parsed, ok := wire.ParseOperatorDecisionOutcome(outcome)
	if !ok {
		parsed = wire.OperatorDecisionOutcomeUnavailable
	}
	result := wire.OperatorDecision{Outcome: parsed}
	if parsed == wire.OperatorDecisionOutcomeAnswered {
		value := operatorMetadata(answered)
		result.Record = &value
	}
	return result, nil
}
