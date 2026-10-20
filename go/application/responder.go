package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

// ResponderEffect completes the separate resource-rule effect of a recorded
// choice. The effect is invoked after the Book edit releases its lock.
type ResponderEffect func(context.Context, func() error, bool) (wire.ResponderEffectOutcome, string)

// AuthorizeResponder is trusted host composition. It examines the current
// service-owned record and admission scope, and may return a guarded effect.
// The receiving peer is rebound on each call and before the Book edit.
type AuthorizeResponder func(context.Context, *identity.Peer, asks.Record, string, string) (ResponderEffect, error)

var ErrResponderForbidden = errors.New("asks: responder forbidden")

func (h *Host) EnableResponder(authorize AuthorizeResponder) error {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	if h.serving || h.ctx.Err() != nil || authorize == nil {
		return errors.New("asks: configure responder before Serve with authorization")
	}
	h.responder = authorize
	return nil
}

type responderReceiver struct {
	receiver
	ctx context.Context
}

func (r *responderReceiver) authorize(record asks.Record, scope, option string) (ResponderEffect, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	if _, _, err := r.scope(); err != nil || r.call.Recheck() != nil {
		return nil, ErrResponderForbidden
	}
	if r.host.responder == nil {
		return nil, ErrResponderForbidden
	}
	peer, err := r.call.Peer()
	if err != nil {
		return nil, ErrResponderForbidden
	}
	return r.host.responder(r.ctx, peer, record, scope, option)
}

func responderPageRefusal(outcome wire.OperatorPageOutcome) wire.OperatorPage {
	return wire.OperatorPage{Outcome: outcome, Records: []wire.RecordMetadata{}}
}

func (r *responderReceiver) ListAnswerable(cursor string, limit int64) (wire.OperatorPage, error) {
	caller, _, err := r.scope()
	if err != nil || r.host.responder == nil {
		return responderPageRefusal(wire.OperatorPageOutcomeForbidden), nil
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return responderPageRefusal(wire.OperatorPageOutcomeInvalid), nil
	}
	entries, revision, err := r.host.book.ApplicationHistoryScoped()
	if err != nil {
		return responderPageRefusal(wire.OperatorPageOutcomeUnavailable), nil
	}
	sum := sha256.Sum256([]byte(caller))
	prefix := r.host.operatorEpoch + ":" + hex.EncodeToString(sum[:]) + ":" + revision + ":"
	offset := 0
	if cursor != "" {
		word, ok := strings.CutPrefix(cursor, prefix)
		n, parseErr := strconv.Atoi(word)
		if !ok || parseErr != nil || n < 0 || n > len(entries) || strconv.Itoa(n) != word {
			return responderPageRefusal(wire.OperatorPageOutcomeGap), nil
		}
		offset = n
	}
	page := wire.OperatorPage{Outcome: wire.OperatorPageOutcomePage, Records: []wire.RecordMetadata{}}
	used := 2048
	for offset < len(entries) && int64(len(page.Records)) < limit {
		entry := entries[offset]
		offset++
		if !entry.Record.Pending() || entry.Scope == "" {
			continue
		}
		meta := operatorMetadata(entry.Record)
		meta.Options = nil
		for _, option := range entry.Record.Options {
			if _, e := r.authorize(entry.Record, entry.Scope, option); e == nil {
				meta.Options = append(meta.Options, option)
			} else if !errors.Is(e, ErrResponderForbidden) {
				return responderPageRefusal(wire.OperatorPageOutcomeUnavailable), nil
			}
		}
		if len(meta.Options) == 0 {
			continue
		}
		data, e := json.Marshal(meta)
		cost := len(data) + 1024 + 32*len(meta.Options)
		if e != nil || cost > (256<<10)-2048 {
			return responderPageRefusal(wire.OperatorPageOutcomeUnavailable), nil
		}
		if used+cost > 256<<10 {
			offset--
			break
		}
		used += cost
		page.Records = append(page.Records, meta)
	}
	page.Complete = offset == len(entries)
	if !page.Complete {
		page.Next = prefix + strconv.Itoa(offset)
	}
	// A revocation while composing a page must not disclose a now-unanswerable
	// record as actionable. Recheck every returned option before reply.
	for i := range page.Records {
		entry := entries[0]
		for _, candidate := range entries {
			if candidate.Record.ID == page.Records[i].ID {
				entry = candidate
				break
			}
		}
		for _, option := range page.Records[i].Options {
			if _, err := r.authorize(entry.Record, entry.Scope, option); err != nil {
				return responderPageRefusal(wire.OperatorPageOutcomeGap), nil
			}
		}
	}
	return page, nil
}

func (r *responderReceiver) Answer(id, option string) (wire.ResponderDecision, error) {
	result := wire.ResponderDecision{Outcome: wire.OperatorDecisionOutcomeUnavailable, Effect: wire.ResponderEffectOutcomeNone}
	if _, _, err := r.scope(); err != nil || r.host.responder == nil {
		result.Outcome = wire.OperatorDecisionOutcomeForbidden
		return result, nil
	}
	// The answer records its actor, the answering program the call bound
	// (ASK-O7, ASK-R5).
	peer, err := r.call.Peer()
	if err != nil {
		result.Outcome = wire.OperatorDecisionOutcomeForbidden
		return result, nil
	}
	answerer, err := identity.SubjectProgram(peer, listen.Program.Path)
	if err != nil {
		result.Outcome = wire.OperatorDecisionOutcomeForbidden
		return result, nil
	}
	var effect ResponderEffect
	wasPending := false
	record, outcome, err := r.host.book.AnswerApplicationAsResponder(id, option, answerer, func(record asks.Record, scope string) error {
		wasPending = record.Pending()
		var e error
		effect, e = r.authorize(record, scope, option)
		return e
	})
	if err != nil {
		if errors.Is(err, ErrResponderForbidden) {
			result.Outcome = wire.OperatorDecisionOutcomeForbidden
			r.askAnswering()
		}
		return result, nil
	}
	parsed, ok := wire.ParseOperatorDecisionOutcome(outcome)
	if !ok {
		return result, nil
	}
	result.Outcome = parsed
	if parsed == wire.OperatorDecisionOutcomeAnswered {
		meta := operatorMetadata(record)
		result.Record = &meta
		if effect != nil {
			result.Effect, result.TargetOutcome = effect(r.ctx, r.call.Recheck, wasPending)
		}
	}
	return result, nil
}
