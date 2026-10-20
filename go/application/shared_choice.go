package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
)

// ChoiceResponderEffect runs after an enrolled assistant's answer to a shared
// choice is recorded, outside the Book edit; the runtime renews the
// enrollment there (ASK-R9). A failure leaves the recorded answer standing.
type ChoiceResponderEffect func(context.Context)

// AuthorizeChoiceResponder is trusted host composition: it decides whether
// the bound peer may answer this shared choice now, from its enrollment, its
// program and the parties to the choice (ASK-R9). It is called for every
// listed choice and inside the Book edit of every answer. A refusal returns
// ErrResponderForbidden; any other error is unavailable.
type AuthorizeChoiceResponder func(context.Context, *identity.Peer, asks.ChoiceCandidate) (ChoiceResponderEffect, error)

// EnableChoiceResponder configures the answerer check for shared ordinary
// choices before Serve. Without it, responder@1's choice methods refuse.
func (h *Host) EnableChoiceResponder(authorize AuthorizeChoiceResponder) error {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	if h.serving || h.ctx.Err() != nil || authorize == nil {
		return errors.New("asks: configure the choice answerer before Serve with authorization")
	}
	h.choiceResponder = authorize
	return nil
}

func (r *responderReceiver) authorizeChoice(c asks.ChoiceCandidate) (ChoiceResponderEffect, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	if _, _, err := r.scope(); err != nil || r.call.Recheck() != nil {
		return nil, ErrResponderForbidden
	}
	if r.host.choiceResponder == nil {
		return nil, ErrResponderForbidden
	}
	peer, err := r.call.Peer()
	if err != nil {
		return nil, ErrResponderForbidden
	}
	return r.host.choiceResponder(r.ctx, peer, c)
}

// answerableChoice is a pending shared choice with only its shared options,
// in the order the application offered them.
func answerableChoice(c asks.ChoiceCandidate) wire.AnswerableChoice {
	out := wire.AnswerableChoice{ID: c.Record.ID, Asker: c.Record.Asker, Title: c.Record.Request.Title, Options: []wire.ChoiceOption{},
		PendingUntilMs: c.Record.Request.PendingUntilMs, Publisher: c.Record.Publisher, PublisherTrusted: c.Record.PublisherTrusted}
	for _, o := range c.Record.Request.Options {
		if slices.Contains(c.Shared, o.ID) {
			out.Options = append(out.Options, o)
		}
	}
	return out
}

func answerableRefusal(outcome wire.OperatorPageOutcome) wire.AnswerableChoicePage {
	return wire.AnswerableChoicePage{Outcome: outcome, Choices: []wire.AnswerableChoice{}}
}

// ListChoices lists the pending shared choices this caller may answer now
// (ASK-R9). Each listed choice is checked again before the page is returned.
func (r *responderReceiver) ListChoices(cursor string, limit int64) (wire.AnswerableChoicePage, error) {
	caller, _, err := r.scope()
	if err != nil || r.host.choiceResponder == nil {
		return answerableRefusal(wire.OperatorPageOutcomeForbidden), nil
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return answerableRefusal(wire.OperatorPageOutcomeInvalid), nil
	}
	candidates, revision, err := r.host.book.SharedChoices()
	if err != nil {
		return answerableRefusal(wire.OperatorPageOutcomeUnavailable), nil
	}
	sum := sha256.Sum256([]byte(caller))
	prefix := r.host.operatorEpoch + ":" + hex.EncodeToString(sum[:]) + ":choices:" + revision + ":"
	offset := 0
	if cursor != "" {
		word, ok := strings.CutPrefix(cursor, prefix)
		n, parseErr := strconv.Atoi(word)
		if !ok || parseErr != nil || n < 0 || n > len(candidates) || strconv.Itoa(n) != word {
			return answerableRefusal(wire.OperatorPageOutcomeGap), nil
		}
		offset = n
	}
	page := wire.AnswerableChoicePage{Outcome: wire.OperatorPageOutcomePage, Choices: []wire.AnswerableChoice{}}
	listed := []asks.ChoiceCandidate{}
	used := 2048
	for offset < len(candidates) && int64(len(page.Choices)) < limit {
		c := candidates[offset]
		offset++
		if _, e := r.authorizeChoice(c); e != nil {
			if errors.Is(e, ErrResponderForbidden) {
				continue
			}
			return answerableRefusal(wire.OperatorPageOutcomeUnavailable), nil
		}
		choice := answerableChoice(c)
		data, e := json.Marshal(choice)
		cost := len(data) + 1024
		if e != nil || cost > (256<<10)-2048 {
			return answerableRefusal(wire.OperatorPageOutcomeUnavailable), nil
		}
		if used+cost > 256<<10 {
			offset--
			break
		}
		used += cost
		page.Choices = append(page.Choices, choice)
		listed = append(listed, c)
	}
	page.Complete = offset == len(candidates)
	if !page.Complete {
		page.Next = prefix + strconv.Itoa(offset)
	}
	// A revocation while composing the page must not disclose a choice as
	// answerable.
	for _, c := range listed {
		if _, err := r.authorizeChoice(c); err != nil {
			return answerableRefusal(wire.OperatorPageOutcomeGap), nil
		}
	}
	return page, nil
}

// AnswerChoice records a shared option for a pending shared choice under the
// caller's current enrollment (ASK-R9). The answer records the bound
// answering program as its actor.
func (r *responderReceiver) AnswerChoice(id, option string) (wire.ChoiceDecision, error) {
	refused := func(outcome wire.ChoiceDecisionOutcome) (wire.ChoiceDecision, error) {
		return wire.ChoiceDecision{Outcome: outcome}, nil
	}
	if _, _, err := r.scope(); err != nil || r.host.choiceResponder == nil {
		return refused(wire.ChoiceDecisionOutcomeForbidden)
	}
	if !asks.ValidRequestKey(id) || !asks.ValidChoiceOptionID(option) {
		return refused(wire.ChoiceDecisionOutcomeInvalid)
	}
	// The same actor derivation as the requester's and the operator's.
	answerer, err := r.choiceActor()
	if err != nil {
		return refused(wire.ChoiceDecisionOutcomeForbidden)
	}
	var effect ChoiceResponderEffect
	result, recorded, err := r.host.book.AnswerSharedChoice(id, option, answerer, func(c asks.ChoiceCandidate) error {
		var e error
		effect, e = r.authorizeChoice(c)
		return e
	})
	switch {
	case errors.Is(err, ErrResponderForbidden):
		r.askAnswering()
		return refused(wire.ChoiceDecisionOutcomeForbidden)
	case errors.Is(err, asks.ErrChoiceNotShared):
		return refused(wire.ChoiceDecisionOutcomeForbidden)
	case err != nil:
		return refused(wire.ChoiceDecisionOutcomeUnavailable)
	}
	if recorded && effect != nil {
		effect(r.ctx)
	}
	return result, nil
}
