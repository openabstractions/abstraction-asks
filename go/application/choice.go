package application

import (
	"errors"
	"path/filepath"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

const OrdinaryChoicePolicyKey = "ordinary.choice"

type choiceReceiver struct{ receiver }

// Request text never supplies admission authority or publisher information.
func (r *choiceReceiver) authorizeAsk() error {
	if err := r.call.WaitContext().Err(); err != nil {
		return err
	}
	if _, _, err := r.scope(); err != nil {
		return ErrOperatorForbidden
	}
	if r.host.askPolicy != nil {
		peer, err := r.call.Peer()
		if err != nil {
			return err
		}
		err = r.host.askPolicy(r.call.WaitContext(), peer, OrdinaryChoicePolicyKey)
		if errors.Is(err, ErrAskPolicyUnavailable) {
			return err
		}
		if err != nil {
			return ErrOperatorForbidden
		}
	}
	return r.call.Recheck()
}
func choiceFailure(err error) wire.ChoiceOutcome {
	if errors.Is(err, ErrOperatorForbidden) {
		return wire.ChoiceOutcomeForbidden
	}
	return wire.ChoiceOutcomeUnavailable
}
func (r *choiceReceiver) Ask(q wire.ChoiceRequest) (wire.ChoiceObservation, error) {
	return r.ask(q, nil)
}

// ShareChoice is Ask that shares the choice with an enrolled assistant,
// which may choose only the shared options (ASK-A12).
func (r *choiceReceiver) ShareChoice(q wire.ChoiceRequest, shared []string) (wire.ChoiceObservation, error) {
	if !asks.ValidChoiceRequest(q) || !asks.ValidAssistantOptions(q, shared) {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeInvalid}, nil
	}
	return r.ask(q, shared)
}

func (r *choiceReceiver) ask(q wire.ChoiceRequest, shared []string) (wire.ChoiceObservation, error) {
	scope, program, err := r.scope()
	if err != nil {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeForbidden}, nil
	}
	// The record shows the display name derived from the bound program; the
	// Book keeps the program beside it (ASK-O12).
	program = identity.CanonicalProgramPath(program)
	if err = r.authorizeAsk(); err != nil {
		return wire.ChoiceObservation{Outcome: choiceFailure(err)}, nil
	}
	publisher := asks.Publisher{}
	if peer, e := r.call.Peer(); e == nil {
		if code, proof := peer.Code.Get(); proof > identity.ProofNone {
			publisher = asks.CodePublisher(code)
		}
	}
	result, err := r.host.book.AskChoiceAs(scope, program, q, shared, r.call.Caller, publisher, r.authorizeAsk)
	if err != nil {
		result = wire.ChoiceObservation{Outcome: choiceFailure(err)}
	}
	return result, nil
}
func (r *choiceReceiver) Observe(ref wire.RequestRef, waitMS int64) (wire.ChoiceObservation, error) {
	scope, _, err := r.scope()
	if err != nil {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeForbidden}, nil
	}
	if waitMS < 0 || waitMS > 30000 || !asks.ValidChoiceReference(ref) {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeInvalid}, nil
	}
	result, err := r.host.book.ObserveChoice(scope, ref)
	if err != nil {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeUnavailable}, nil
	}
	if result.Outcome != wire.ChoiceOutcomePending || waitMS == 0 {
		return result, nil
	}
	wait := time.Duration(waitMS) * time.Millisecond
	if record, found, e := r.host.book.ChoiceRecord(result.ID); e != nil {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeUnavailable}, nil
	} else if found {
		until := time.Until(time.UnixMilli(record.Request.PendingUntilMs))
		if until < wait {
			wait = max(until, 0)
		}
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-r.call.WaitContext().Done():
		return wire.ChoiceObservation{}, r.call.WaitContext().Err()
	case <-timer.C:
	case <-r.host.book.Decided(result.ID):
	}
	if _, _, err := r.scope(); err != nil {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeForbidden}, nil
	}
	result, err = r.host.book.ObserveChoice(scope, ref)
	if err != nil {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeUnavailable}, nil
	}
	return result, nil
}
func (r *choiceReceiver) Acknowledge(ref wire.RequestRef, answer wire.ChoiceAnswer) (wire.ChoiceAcknowledgement, error) {
	scope, _, err := r.scope()
	if err != nil {
		return wire.ChoiceAcknowledgement{Outcome: wire.ChoiceAckOutcomeForbidden}, nil
	}
	result, err := r.host.book.AcknowledgeChoice(scope, ref, answer, func() error {
		if err := r.call.WaitContext().Err(); err != nil {
			return err
		}
		if _, _, err := r.scope(); err != nil {
			return ErrOperatorForbidden
		}
		return r.call.Recheck()
	})
	if err != nil {
		outcome := wire.ChoiceAckOutcomeUnavailable
		if errors.Is(err, ErrOperatorForbidden) {
			outcome = wire.ChoiceAckOutcomeForbidden
		}
		return wire.ChoiceAcknowledgement{Outcome: outcome}, nil
	}
	return result, nil
}

func (r *choiceReceiver) Answer(ref wire.RequestRef, option string, ifPending bool) (wire.ChoiceDecision, error) {
	return r.decideOwnChoice(ref, option, false, ifPending)
}
func (r *choiceReceiver) Dismiss(ref wire.RequestRef) (wire.ChoiceDecision, error) {
	return r.decideOwnChoice(ref, "", true, false)
}
func (r *choiceReceiver) decideOwnChoice(ref wire.RequestRef, option string, dismiss, ifPending bool) (wire.ChoiceDecision, error) {
	failed := func(err error) wire.ChoiceDecision {
		outcome, _ := wire.ParseChoiceDecisionOutcome(operatorDecisionError(err).String())
		return wire.ChoiceDecision{Outcome: outcome}
	}
	scope, _, err := r.scope()
	if err != nil {
		return failed(ErrOperatorForbidden), nil
	}
	actor, err := r.choiceActor()
	if err != nil {
		return failed(err), nil
	}
	result, err := r.host.book.DecideOwnChoiceAuthorized(scope, ref, option, actor, dismiss, ifPending, func() error {
		if err := r.call.WaitContext().Err(); err != nil {
			return err
		}
		currentScope, _, err := r.scope()
		if err != nil || currentScope != scope {
			return ErrOperatorForbidden
		}
		currentActor, err := r.choiceActor()
		if err != nil || currentActor != actor {
			return ErrOperatorForbidden
		}
		return r.call.Recheck()
	})
	if err != nil {
		return failed(err), nil
	}
	return result, nil
}

func (r *receiver) choiceActor() (string, error) {
	peer, err := r.call.Peer()
	if err != nil {
		return "", err
	}
	program, err := peer.Path.AtLeast(listen.Program.Path)
	if err != nil || !filepath.IsAbs(program) {
		return "", ErrOperatorForbidden
	}
	return identity.CanonicalProgramPath(filepath.Clean(program)), nil
}
