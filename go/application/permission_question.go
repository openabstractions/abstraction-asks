package application

import (
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
	rightsclient "github.com/openabstractions/abstraction-rights/go/client"
)

// questionAbout reports whether record is a first-use question whose
// permission is for subject's own program on owner's account: the runtime's
// own first-use question (ASK-R2) or a typed occurrence (ASK-A11).
func questionAbout(record asks.Record, owner string, subject rwire.Subject) bool {
	if subject.Account != owner {
		return false
	}
	if record.TypedPermission != nil {
		s := record.TypedPermission.Effect.Subject
		return s.Account == owner && identity.SameSubjectProgram(s.Program, subject.Program)
	}
	permission, ok := asks.PermissionForRecord(owner, record)
	return ok && identity.SameSubjectProgram(permission.Targets[0].Program, subject.Program)
}

// joinedObservation projects a question about subject: pending, or answered
// with the person's option. Any other record reads unknown, so nothing about
// another program's question is disclosed.
func joinedObservation(record asks.Record, owner string, subject rwire.Subject) wire.PermissionObservation {
	if !questionAbout(record, owner, subject) {
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnknown}
	}
	outcome := wire.PermissionOutcomeAnswered
	if record.Pending() {
		outcome = wire.PermissionOutcomePending
	}
	return wire.PermissionObservation{Outcome: outcome, Answer: &wire.Answer{ID: record.ID, Pending: record.Pending(), Option: record.Option, Yes: record.Yes, Kept: record.Kept}}
}

// ObserveQuestion waits up to waitMS for the answer to one first-use question
// about this proven caller's own program (ASK-A14). It grants, answers and
// admits nothing.
func (r *permissionReceiver) ObserveQuestion(id string, waitMS int64) (wire.PendingQuestion, error) {
	refuse := func(outcome wire.PermissionOutcome) (wire.PendingQuestion, error) {
		return wire.PendingQuestion{Observation: wire.PermissionObservation{Outcome: outcome}}, nil
	}
	if !asks.ValidRequestKey(id) || waitMS < 0 || waitMS > 30000 {
		return refuse(wire.PermissionOutcomeInvalid)
	}
	peer, err := r.call.Peer()
	if err != nil {
		return refuse(wire.PermissionOutcomeForbidden)
	}
	subject, err := rightsclient.SubjectFromPeer(peer)
	if err != nil || subject.Account != r.host.owner {
		return refuse(wire.PermissionOutcomeForbidden)
	}
	var record asks.Record
	var found bool
	var readErr error
	var observation wire.PermissionObservation
	waited, err := r.host.observe(r.call.WaitContext(), waitMS, func() bool {
		record, found, readErr = r.host.book.ApplicationRecord(id)
		if readErr != nil || !found {
			return true
		}
		if !questionAbout(record, r.host.owner, subject) {
			observation = wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnknown}
			return true
		}
		observation = joinedObservation(record, r.host.owner, subject)
		if r.host.permissionObservation != nil {
			current, e := r.host.permissionObservation(r.call.WaitContext(), peer, record)
			if e != nil {
				observation = wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnavailable}
				return true
			}
			if (current.Outcome == wire.PermissionOutcomePermitted || current.Outcome == wire.PermissionOutcomeDenied) && asks.ValidRequestKey(current.PolicyRevision) {
				observation = wire.PermissionObservation{Outcome: current.Outcome, PolicyRevision: current.PolicyRevision}
				return true
			}
			if !record.Pending() && record.Option == "allow" {
				observation = wire.PermissionObservation{Outcome: wire.PermissionOutcomePending, Answer: &wire.Answer{ID: record.ID, Pending: true}}
				return false
			}
		}
		return !record.Pending()
	}, r.call.Recheck, func() string { return record.ID })
	switch waited {
	case waitForbidden:
		return refuse(wire.PermissionOutcomeForbidden)
	case waitCancelled:
		return wire.PendingQuestion{}, err
	case waitUnavailable:
		return refuse(wire.PermissionOutcomeUnavailable)
	}
	if readErr != nil {
		return refuse(wire.PermissionOutcomeUnavailable)
	}
	if !found {
		return refuse(wire.PermissionOutcomeUnknown)
	}
	result := wire.PendingQuestion{Observation: observation}
	if result.Observation.Outcome == wire.PermissionOutcomePending {
		result.PendingUntilMs = r.host.book.PendingUntil(record).UnixMilli()
	}
	return result, nil
}

// exactObservation answers a request for an exact first-use action
// (ASK-A14) from its preparation.
func exactObservation(prepared PreparedPermission, owner string, subject rwire.Subject) wire.PermissionObservation {
	switch prepared.Outcome {
	case wire.PermissionOutcomePermitted, wire.PermissionOutcomeDenied:
		if !asks.ValidRequestKey(prepared.Revision) {
			return wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnavailable}
		}
		return wire.PermissionObservation{Outcome: prepared.Outcome, PolicyRevision: prepared.Revision}
	case wire.PermissionOutcomePending, wire.PermissionOutcomeAnswered:
		if prepared.Joined == nil {
			return wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnavailable}
		}
		return joinedObservation(*prepared.Joined, owner, subject)
	case wire.PermissionOutcomeInvalid, wire.PermissionOutcomeForbidden:
		return wire.PermissionObservation{Outcome: prepared.Outcome}
	}
	return wire.PermissionObservation{Outcome: wire.PermissionOutcomeUnavailable}
}
