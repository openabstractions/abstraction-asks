package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
)

func (r *operatorReceiver) ListChoices(cursor string, limit int64) (wire.ChoicePage, error) {
	refused := func(outcome wire.OperatorPageOutcome) wire.ChoicePage {
		return wire.ChoicePage{Outcome: outcome, Records: []wire.ChoiceRecord{}}
	}
	scope, err := r.authorize()
	if err != nil {
		return refused(operatorPageError(err)), nil
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return refused(wire.OperatorPageOutcomeInvalid), nil
	}
	records, revision, err := r.host.book.ChoiceHistory()
	if err != nil {
		return refused(wire.OperatorPageOutcomeUnavailable), nil
	}
	sum := sha256.Sum256([]byte(scope))
	prefix := r.host.operatorEpoch + ":" + hex.EncodeToString(sum[:]) + ":" + revision + ":"
	offset := 0
	if cursor != "" {
		word, ok := strings.CutPrefix(cursor, prefix)
		n, e := strconv.Atoi(word)
		if !ok || e != nil || n < 0 || n > len(records) || strconv.Itoa(n) != word {
			return refused(wire.OperatorPageOutcomeGap), nil
		}
		offset = n
	}
	page := wire.ChoicePage{Outcome: wire.OperatorPageOutcomePage, Records: []wire.ChoiceRecord{}}
	used := 2048
	for offset < len(records) && int64(len(page.Records)) < limit {
		record := records[offset]
		raw, e := json.Marshal(record)
		cost := len(raw) + 2048
		if e != nil || cost > (256<<10)-2048 {
			return refused(wire.OperatorPageOutcomeUnavailable), nil
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
	if _, err = r.authorize(); err != nil {
		return refused(operatorPageError(err)), nil
	}
	return page, nil
}
func (r *operatorReceiver) ReadChoice(id string) (wire.ChoiceRead, error) {
	if _, err := r.authorize(); err != nil {
		return wire.ChoiceRead{Outcome: operatorQuestionError(err)}, nil
	}
	if !asks.ValidRequestKey(id) {
		return wire.ChoiceRead{Outcome: wire.OperatorQuestionOutcomeInvalid}, nil
	}
	record, found, err := r.host.book.ChoiceRecord(id)
	if err != nil {
		return wire.ChoiceRead{Outcome: wire.OperatorQuestionOutcomeUnavailable}, nil
	}
	if !found {
		return wire.ChoiceRead{Outcome: wire.OperatorQuestionOutcomeUnknown}, nil
	}
	if _, err = r.authorize(); err != nil {
		return wire.ChoiceRead{Outcome: operatorQuestionError(err)}, nil
	}
	return wire.ChoiceRead{Outcome: wire.OperatorQuestionOutcomeFound, Record: &record}, nil
}
func (r *operatorReceiver) AnswerChoice(id, option string, ifPending bool) (wire.ChoiceDecision, error) {
	return r.decideChoice(id, option, false, ifPending)
}
func (r *operatorReceiver) DismissChoice(id string) (wire.ChoiceDecision, error) {
	return r.decideChoice(id, "", true, false)
}
func (r *operatorReceiver) decideChoice(id, option string, dismiss, ifPending bool) (wire.ChoiceDecision, error) {
	failed := func(err error) wire.ChoiceDecision {
		outcome, _ := wire.ParseChoiceDecisionOutcome(operatorDecisionError(err).String())
		return wire.ChoiceDecision{Outcome: outcome}
	}
	if _, err := r.authorize(); err != nil {
		return failed(err), nil
	}
	actor, err := r.choiceActor()
	if err != nil {
		return failed(err), nil
	}
	result, err := r.host.book.DecideChoiceAuthorized(id, option, actor, dismiss, ifPending, func() error {
		if _, err := r.authorize(); err != nil {
			return err
		}
		return r.call.Recheck()
	})
	if err != nil {
		return failed(err), nil
	}
	return result, nil
}

// ReadChoiceDetails reads one retained ordinary occurrence with its bound
// requester program, shared options and decision route (ASK-O12).
func (r *operatorReceiver) ReadChoiceDetails(id string) (wire.ChoiceDetails, error) {
	refused := func(outcome wire.OperatorQuestionOutcome) wire.ChoiceDetails {
		return wire.ChoiceDetails{Choice: wire.ChoiceRead{Outcome: outcome}, AssistantOptions: []string{}}
	}
	if _, err := r.authorize(); err != nil {
		return refused(operatorQuestionError(err)), nil
	}
	if !asks.ValidRequestKey(id) {
		return refused(wire.OperatorQuestionOutcomeInvalid), nil
	}
	read, found, err := r.host.book.ChoiceAttributed(id)
	if err != nil {
		return refused(wire.OperatorQuestionOutcomeUnavailable), nil
	}
	if !found {
		return refused(wire.OperatorQuestionOutcomeUnknown), nil
	}
	if _, err = r.authorize(); err != nil {
		return refused(operatorQuestionError(err)), nil
	}
	out := wire.ChoiceDetails{Choice: wire.ChoiceRead{Outcome: wire.OperatorQuestionOutcomeFound, Record: &read.Record}, AskerProgram: read.AskerProgram, AssistantOptions: read.Shared}
	if out.AssistantOptions == nil {
		out.AssistantOptions = []string{}
	}
	if route, ok := wire.ParseChoiceRoute(read.Route); ok {
		out.AnsweredVia = &route
	}
	return out, nil
}

// ListAnswers pages the answer audit, newest first (ASK-O15).
func (r *operatorReceiver) ListAnswers(cursor string, limit int64) (wire.AnswerEntryPage, error) {
	answers, page, err := pageAudit(r, cursor, limit, "answers", (*asks.Book).AuditedAnswers)
	if page.Outcome != wire.OperatorPageOutcomePage {
		return wire.AnswerEntryPage{Outcome: page.Outcome, Answers: []wire.AnswerEntry{}}, err
	}
	return wire.AnswerEntryPage{Outcome: page.Outcome, Answers: answers, Next: page.Next, Complete: page.Complete, CompleteSinceMs: page.CompleteSinceMs}, nil
}

// auditPage is one audit page's outcome, cursor and completeness.
type auditPage struct {
	Outcome         wire.OperatorPageOutcome
	Next            string
	Complete        bool
	CompleteSinceMs int64
}

// pageAudit pages one newest-first audit list within ASK-O2's page, cursor
// and frame bounds, rechecking authorization before and after the read.
func pageAudit[T any](r *operatorReceiver, cursor string, limit int64, list string, read func(*asks.Book) ([]T, int64, string, error)) ([]T, auditPage, error) {
	scope, err := r.authorize()
	if err != nil {
		return nil, auditPage{Outcome: operatorPageError(err)}, nil
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return nil, auditPage{Outcome: wire.OperatorPageOutcomeInvalid}, nil
	}
	answers, since, revision, err := read(r.host.book)
	if err != nil {
		return nil, auditPage{Outcome: wire.OperatorPageOutcomeUnavailable}, nil
	}
	sum := sha256.Sum256([]byte(scope))
	prefix := r.host.operatorEpoch + ":" + hex.EncodeToString(sum[:]) + ":" + list + ":" + revision + ":"
	offset := 0
	if cursor != "" {
		word, ok := strings.CutPrefix(cursor, prefix)
		n, e := strconv.Atoi(word)
		if !ok || e != nil || n < 0 || n > len(answers) || strconv.Itoa(n) != word {
			return nil, auditPage{Outcome: wire.OperatorPageOutcomeGap}, nil
		}
		offset = n
	}
	out := []T{}
	used := 2048
	for offset < len(answers) && int64(len(out)) < limit {
		raw, e := json.Marshal(answers[offset])
		cost := len(raw) + 1024
		if e != nil || cost > (256<<10)-2048 {
			return nil, auditPage{Outcome: wire.OperatorPageOutcomeUnavailable}, nil
		}
		if used+cost > 256<<10 {
			break
		}
		used += cost
		out = append(out, answers[offset])
		offset++
	}
	page := auditPage{Outcome: wire.OperatorPageOutcomePage, Complete: offset == len(answers), CompleteSinceMs: since}
	if !page.Complete {
		page.Next = prefix + strconv.Itoa(offset)
	}
	if _, err = r.authorize(); err != nil {
		return nil, auditPage{Outcome: operatorPageError(err)}, nil
	}
	return out, page, nil
}
