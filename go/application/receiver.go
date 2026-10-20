package application

import (
	"errors"
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	"path/filepath"
)

type receiver struct {
	host *Host
	call *listen.FramedCall
}

// callerScope derives the admission subject from the receiving account and
// the bound program (ASK-A1). It returns the scope and that program, the
// clean absolute path the platform proved; records keep the program and
// derive their display name from it (asks.AskerDisplay).
func callerScope(p *identity.Peer, owner string) (string, string, error) {
	if p == nil {
		return "", "", errors.New("asks: caller required")
	}
	s, verdict := identity.AdmitOwner(p, identity.Need{User: listen.Program.User}, owner)
	if verdict == identity.OwnerOtherAccount {
		return "", "", errors.New("asks: caller account refused")
	}
	if verdict != identity.OwnerAdmitted {
		return "", "", errors.New("asks: caller identity required")
	}
	path, e := p.Path.AtLeast(listen.Program.Path)
	if e != nil || !filepath.IsAbs(path) {
		return "", "", errors.New("asks: bound program required")
	}
	path = filepath.Clean(path)
	return asks.ApplicationOwnerProgramScope(s.AccountKind, s.Account, path), path, nil
}
func (r *receiver) scope() (string, string, error) {
	p, e := r.call.Peer()
	if e != nil {
		return "", "", e
	}
	return callerScope(p, r.host.owner)
}
func observation(record asks.Record, outcome string, e error) wire.QuestionObservation {
	if e != nil {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeUnavailable}
	}
	parsed, ok := wire.ParseObservationOutcome(outcome)
	if !ok {
		parsed = wire.ObservationOutcomeUnavailable
	}
	result := wire.QuestionObservation{Outcome: parsed}
	if parsed == wire.ObservationOutcomePending || parsed == wire.ObservationOutcomeAnswered {
		result.Answer = &wire.Answer{ID: record.ID, Pending: record.Pending(), Option: record.Option, Yes: record.Yes, Kept: record.Kept}
	}
	return result
}
func (r *receiver) Ask(q wire.ApplicationQuestion) (wire.QuestionObservation, error) {
	if asks.RuntimeOnlyKey(q.Key) {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeForbidden}, nil
	}
	scope, program, e := r.scope()
	if e != nil {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeForbidden}, nil
	}
	r.host.lifecycle.Lock()
	policy := r.host.askPolicy
	r.host.lifecycle.Unlock()
	if policy != nil {
		peer, err := r.call.Peer()
		if err != nil {
			return wire.QuestionObservation{Outcome: wire.ObservationOutcomeForbidden}, nil
		}
		ctx := r.call.WaitContext()
		err = policy(ctx, peer, q.Key)
		if errors.Is(err, ErrAskPolicyUnavailable) || (err == nil && ctx.Err() != nil) {
			return wire.QuestionObservation{Outcome: wire.ObservationOutcomeUnavailable}, nil
		}
		if err != nil || r.call.Recheck() != nil {
			return wire.QuestionObservation{Outcome: wire.ObservationOutcomeForbidden}, nil
		}
	}
	// The publisher is the code signer of the peer this call bound (ASK-A6).
	publisher := asks.Publisher{}
	if peer, err := r.call.Peer(); err == nil {
		if code, proof := peer.Code.Get(); proof > identity.ProofNone {
			publisher = asks.CodePublisher(code)
		}
	}
	record, outcome, e := r.nativeBookAskApplicationAs(scope, program, q.RequestKey, q.Key, q.Slots, r.call.Caller, publisher)
	return observation(record, outcome, e), nil
}
func (r *receiver) Observe(key string, waitMS int64) (wire.QuestionObservation, error) {
	scope, _, e := r.scope()
	if e != nil {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeForbidden}, nil
	}
	if waitMS < 0 || waitMS > 30000 || !asks.ValidRequestKey(key) {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeInvalid}, nil
	}
	var record asks.Record
	var outcome string
	var readErr error
	// Recheck binding and storage after every read; forgetting is an explicit gap.
	waited, e := r.host.observe(r.call.WaitContext(), waitMS, func() bool {
		record, outcome, readErr = r.nativeBookObserveApplication(scope, key)
		return readErr != nil || outcome != wire.ObservationOutcomePending.String()
	}, func() error { _, _, err := r.scope(); return err }, func() string { return record.ID })
	switch waited {
	case waitForbidden:
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeForbidden}, nil
	case waitCancelled:
		return wire.QuestionObservation{}, e
	case waitUnavailable:
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeUnavailable}, nil
	}
	return observation(record, outcome, readErr), nil
}
