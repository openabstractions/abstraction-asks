package application

import (
	"context"
	"errors"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	waitwire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/responderwait"
	identity "github.com/openabstractions/abstraction-identity"
)

// ResponderEnrollment checks the native caller's current general answering enrollment.
type ResponderEnrollment func(context.Context, *identity.Peer) (bool, error)

func (h *Host) EnableResponderWait(enrolled ResponderEnrollment) error {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	if h.serving || h.ctx.Err() != nil || h.responder == nil || enrolled == nil {
		return errors.New("asks: configure responder wait before Serve with enrollment")
	}
	h.responderWait = enrolled
	return nil
}

func (r *responderReceiver) waitEnrollment() wire.OperatorPageOutcome {
	if _, _, err := r.scope(); err != nil || r.call.Recheck() != nil {
		return wire.OperatorPageOutcomeForbidden
	}
	peer, err := r.call.Peer()
	if err != nil {
		return wire.OperatorPageOutcomeForbidden
	}
	enrolled, err := r.host.responderWait(r.ctx, peer)
	if errors.Is(err, ErrResponderForbidden) || err == nil && !enrolled {
		return wire.OperatorPageOutcomeForbidden
	}
	if err != nil {
		return wire.OperatorPageOutcomeUnavailable
	}
	return wire.OperatorPageOutcomePage
}

type responderWaitReceiver struct{ responderReceiver }

func (r *responderWaitReceiver) ListWaiting(cursor string, limit int64) (waitwire.WaitingPage, error) {
	page, err := r.listAnswerable(cursor, limit, true)
	result := waitwire.WaitingPage{IDs: []string{}, Next: page.Next, Complete: page.Complete}
	switch page.Outcome {
	case wire.OperatorPageOutcomePage:
		result.Outcome = waitwire.OutcomePage
		for _, record := range page.Records {
			if r.host.book.LiveQuestionWaiters(record.ID) > 0 {
				result.IDs = append(result.IDs, record.ID)
			}
		}
	case wire.OperatorPageOutcomeGap:
		result.Outcome = waitwire.OutcomeGap
	case wire.OperatorPageOutcomeInvalid:
		result.Outcome = waitwire.OutcomeInvalid
	case wire.OperatorPageOutcomeForbidden:
		result.Outcome = waitwire.OutcomeForbidden
	default:
		result.Outcome = waitwire.OutcomeUnavailable
	}
	return result, err
}
