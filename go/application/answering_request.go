package application

import (
	"context"
	"errors"

	identity "github.com/openabstractions/abstraction-identity"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

// RequestAnswering is trusted host composition for responder@1's
// RequestAnswering (ASK-R10): it decides the bound peer's enrollment and, on
// not_granted, admits the runtime's first-use question for that right. A
// script host, an operator program or another account reads forbidden.
type RequestAnswering func(context.Context, *identity.Peer) rwire.Decision

// EnableAnsweringRequest configures RequestAnswering before Serve. Without it
// the method reads forbidden and a refused answer asks nothing.
func (h *Host) EnableAnsweringRequest(request RequestAnswering) error {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	if h.serving || h.ctx.Err() != nil || request == nil {
		return errors.New("asks: configure the answering request before Serve")
	}
	h.answeringRequest = request
	return nil
}

// RequestAnswering asks for the caller's right to answer for the person.
func (r *responderReceiver) RequestAnswering() (rwire.Decision, error) {
	if _, _, err := r.scope(); err != nil || r.host.answeringRequest == nil {
		return rwire.Decision{Outcome: rwire.DecisionOutcomeForbidden}, nil
	}
	peer, err := r.call.Peer()
	if err != nil {
		return rwire.Decision{Outcome: rwire.DecisionOutcomeForbidden}, nil
	}
	return r.host.answeringRequest(r.ctx, peer), nil
}

// askAnswering is a refused answer's request for the right (ASK-R10), made
// after the Book edit released its lock. Its outcome changes no reply.
func (r *responderReceiver) askAnswering() {
	if r.host.answeringRequest == nil {
		return
	}
	if peer, err := r.call.Peer(); err == nil {
		r.host.answeringRequest(r.ctx, peer)
	}
}
