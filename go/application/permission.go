package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	rightsclient "github.com/openabstractions/abstraction-rights/go/client"
)

// PreparePermission is a trusted configured capability binding. It resolves
// native subject/owner target evidence before any Book lock. Pending means the
// supported effect needs consent. Explicit blocks and outage admit no request.
type PreparePermission func(context.Context, *identity.Peer, wire.PermissionIntent) (PreparedPermission, error)
type PreparedPermission struct {
	Permission asks.TypedPermission
	Outcome    wire.PermissionOutcome
	Revision   string
	// Recheck runs with the Book pinned immediately before admission. It reads
	// the same owning policy revision quietly and performs no Ask or rule edit.
	Recheck func() error
}

func (h *Host) EnablePermission(prepare PreparePermission) error {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	if h.serving || h.ctx.Err() != nil || prepare == nil {
		return errors.New("asks: configure permission preparation before Serve")
	}
	h.permission = prepare
	return nil
}

func permissionScope(subjectAccount, subjectProgram string) string {
	raw, _ := json.Marshal([]string{subjectAccount, subjectProgram})
	sum := sha256.Sum256(raw)
	return "typed-permission-owner@1:" + hex.EncodeToString(sum[:])
}

func validIntent(q wire.PermissionIntent) bool {
	if !asks.ValidRequestKey(q.RequestKey) || len(q.Purpose) > 800 {
		return false
	}
	if q.Selection != nil {
		return q.BundleRef == "" && q.BundleSelector == nil
	}
	return asks.ValidRequestKey(q.BundleRef) && q.BundleSelector != nil
}

func permissionObservation(record asks.Record, outcome string, err error) wire.PermissionObservation {
	if err != nil {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnavailable}
	}
	parsed, ok := wire.ParsePermissionOutcome(outcome)
	if !ok {
		parsed = wire.PermissionOutcomeUnavailable
	}
	result := wire.PermissionObservation{Outcome: parsed}
	if parsed == wire.PermissionOutcomePending || parsed == wire.PermissionOutcomeAnswered {
		if record.TypedPermission == nil {
			return wire.PermissionObservation{Outcome: wire.PermissionOutcomeInvalid}
		}
		result.Answer = &wire.Answer{ID: record.ID, Pending: record.Pending(), Option: record.Option, Yes: record.Yes, Kept: record.Kept}
	}
	return result
}

type permissionReceiver struct{ receiver }

func (r *permissionReceiver) Request(q wire.PermissionIntent) (wire.PermissionObservation, error) {
	if !validIntent(q) {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeInvalid}, nil
	}
	peer, err := r.call.Peer()
	if err != nil {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeForbidden}, nil
	}
	subject, err := rightsclient.SubjectFromPeer(peer)
	if err != nil || subject.Account != r.host.owner {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeForbidden}, nil
	}
	if r.host.permission == nil {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnavailable}, nil
	}
	ctx := r.call.WaitContext()
	prepared, err := r.host.permission(ctx, peer, q)
	if err != nil || ctx.Err() != nil {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnavailable}, nil
	}
	switch prepared.Outcome {
	case wire.PermissionOutcomeInvalid, wire.PermissionOutcomeForbidden, wire.PermissionOutcomeUnavailable:
		return wire.PermissionObservation{Outcome: prepared.Outcome}, nil
	case wire.PermissionOutcomePending, wire.PermissionOutcomePermitted, wire.PermissionOutcomeDenied:
	default:
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnavailable}, nil
	}
	p := prepared.Permission
	if p.Effect.Subject != subject || p.Origin != "application" || p.Purpose != q.Purpose || !asks.ValidTypedPermission(p) {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeInvalid}, nil
	}
	if r.call.Recheck() != nil {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeForbidden}, nil
	}
	scope := permissionScope(subject.Account, subject.Program)
	record, outcome, err := r.host.book.ReplayTypedPermission(scope, q.RequestKey, p)
	if err != nil || outcome != "unknown" {
		return permissionObservation(record, outcome, err), nil
	}
	if prepared.Outcome != wire.PermissionOutcomePending {
		return wire.PermissionObservation{Outcome: prepared.Outcome, PolicyRevision: prepared.Revision}, nil
	}
	publisher := asks.Publisher{}
	if code, proof := peer.Code.Get(); proof > identity.ProofNone {
		publisher = asks.CodePublisher(code)
	}
	record, outcome, err = r.host.book.AskTypedPermissionAuthorizedFrom(scope, subject.Program, q.RequestKey, p, r.call.Caller, publisher, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.call.Recheck(); err != nil {
			return err
		}
		if prepared.Recheck != nil {
			return prepared.Recheck()
		}
		return nil
	})
	return permissionObservation(record, outcome, err), nil
}

func (r *permissionReceiver) Observe(key string, waitMS int64) (wire.PermissionObservation, error) {
	if !asks.ValidRequestKey(key) || waitMS < 0 || waitMS > 30000 {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeInvalid}, nil
	}
	peer, err := r.call.Peer()
	if err != nil {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeForbidden}, nil
	}
	subject, err := rightsclient.SubjectFromPeer(peer)
	if err != nil || subject.Account != r.host.owner {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeForbidden}, nil
	}
	scope := permissionScope(subject.Account, subject.Program)
	record, outcome, err := r.host.book.ObserveApplication(scope, key)
	if err == nil && outcome == "pending" && waitMS > 0 {
		timer := time.NewTimer(time.Duration(waitMS) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-r.call.WaitContext().Done():
			return wire.PermissionObservation{}, r.call.WaitContext().Err()
		case <-timer.C:
		case <-r.host.book.Decided(record.ID):
		}
		record, outcome, err = r.host.book.ObserveApplication(scope, key)
	}
	if r.call.Recheck() != nil {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeForbidden}, nil
	}
	return permissionObservation(record, outcome, err), nil
}

type permissionOperatorReceiver struct{ operatorReceiver }

func (r *permissionOperatorReceiver) ListPermissions(cursor string, limit int64) (wire.OperatorPage, error) {
	return r.listQuestions(cursor, limit, true)
}
func (r *permissionOperatorReceiver) ReadPermissionDetails(id string) (wire.TypedPermissionDetails, error) {
	refusal := func(outcome wire.OperatorQuestionOutcome) wire.TypedPermissionDetails {
		return wire.TypedPermissionDetails{Question: wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: outcome}}}
	}
	if _, err := r.authorize(); err != nil {
		return refusal(operatorQuestionError(err)), nil
	}
	if !asks.ValidRequestKey(id) {
		return refusal(wire.OperatorQuestionOutcomeInvalid), nil
	}
	record, found, err := r.host.book.ApplicationRecord(id)
	if err != nil {
		return refusal(wire.OperatorQuestionOutcomeUnavailable), nil
	}
	if !found {
		return refusal(wire.OperatorQuestionOutcomeUnknown), nil
	}
	if record.TypedPermission == nil || record.Key != asks.TypedPermissionKey {
		return refusal(wire.OperatorQuestionOutcomeInvalid), nil
	}
	if _, err := r.authorize(); err != nil {
		return refusal(operatorQuestionError(err)), nil
	}
	p := asks.CloneTypedPermission(*record.TypedPermission)
	return wire.TypedPermissionDetails{Question: attributedRecord(record), Effect: &p.Effect, EffectDigest: p.Digest, Origin: p.Origin, Purpose: p.Purpose, Display: &p.Display}, nil
}
func (r *permissionOperatorReceiver) AnswerPermissionIfPending(id, option string) (wire.OperatorDecision, error) {
	return r.answer(id, option, func(id, option string, authorize func() error) (asks.Record, string, error) {
		return r.host.book.AnswerTypedPermissionIfPendingAuthorized(id, option, func(asks.Record, string) error { return authorize() })
	})
}
