package client

// The generated contract types this package's API reaches, re-exported so an
// application names them through this package and never imports the generated
// one. scripts/idiom_check.py refuses a reachable type this file leaves out.

import (
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
)

type Answer = wire.Answer

type ObservationOutcome = wire.ObservationOutcome

const (
	ObservationOutcomePending     = wire.ObservationOutcomePending
	ObservationOutcomeAnswered    = wire.ObservationOutcomeAnswered
	ObservationOutcomeUnknown     = wire.ObservationOutcomeUnknown
	ObservationOutcomeGone        = wire.ObservationOutcomeGone
	ObservationOutcomeInvalid     = wire.ObservationOutcomeInvalid
	ObservationOutcomeConflict    = wire.ObservationOutcomeConflict
	ObservationOutcomeForbidden   = wire.ObservationOutcomeForbidden
	ObservationOutcomeUnavailable = wire.ObservationOutcomeUnavailable
)

// ObservationOutcomeValues returns every member of ObservationOutcome in declaration order, in a new slice.
func ObservationOutcomeValues() []ObservationOutcome { return wire.ObservationOutcomeValues() }

type OperatorDecision = wire.OperatorDecision

type OperatorDecisionOutcome = wire.OperatorDecisionOutcome

const (
	OperatorDecisionOutcomeAnswered    = wire.OperatorDecisionOutcomeAnswered
	OperatorDecisionOutcomeConflict    = wire.OperatorDecisionOutcomeConflict
	OperatorDecisionOutcomeUnknown     = wire.OperatorDecisionOutcomeUnknown
	OperatorDecisionOutcomeInvalid     = wire.OperatorDecisionOutcomeInvalid
	OperatorDecisionOutcomeForbidden   = wire.OperatorDecisionOutcomeForbidden
	OperatorDecisionOutcomeUnavailable = wire.OperatorDecisionOutcomeUnavailable
)

// OperatorDecisionOutcomeValues returns every member of OperatorDecisionOutcome in declaration order, in a new slice.
func OperatorDecisionOutcomeValues() []OperatorDecisionOutcome {
	return wire.OperatorDecisionOutcomeValues()
}

type OperatorPage = wire.OperatorPage

type OperatorPageOutcome = wire.OperatorPageOutcome

const (
	OperatorPageOutcomePage        = wire.OperatorPageOutcomePage
	OperatorPageOutcomeGap         = wire.OperatorPageOutcomeGap
	OperatorPageOutcomeInvalid     = wire.OperatorPageOutcomeInvalid
	OperatorPageOutcomeForbidden   = wire.OperatorPageOutcomeForbidden
	OperatorPageOutcomeUnavailable = wire.OperatorPageOutcomeUnavailable
)

// OperatorPageOutcomeValues returns every member of OperatorPageOutcome in declaration order, in a new slice.
func OperatorPageOutcomeValues() []OperatorPageOutcome { return wire.OperatorPageOutcomeValues() }

type OperatorRetirement = wire.OperatorRetirement

type OperatorRetirementOutcome = wire.OperatorRetirementOutcome

const (
	OperatorRetirementOutcomeRetired     = wire.OperatorRetirementOutcomeRetired
	OperatorRetirementOutcomeUnknown     = wire.OperatorRetirementOutcomeUnknown
	OperatorRetirementOutcomeInvalid     = wire.OperatorRetirementOutcomeInvalid
	OperatorRetirementOutcomeForbidden   = wire.OperatorRetirementOutcomeForbidden
	OperatorRetirementOutcomeUnavailable = wire.OperatorRetirementOutcomeUnavailable
)

// OperatorRetirementOutcomeValues returns every member of OperatorRetirementOutcome in declaration order, in a new slice.
func OperatorRetirementOutcomeValues() []OperatorRetirementOutcome {
	return wire.OperatorRetirementOutcomeValues()
}

type RecordMetadata = wire.RecordMetadata

type ServiceError = wire.ServiceError

type ServiceErrorCode = wire.ServiceErrorCode

const (
	ServiceErrorCodeHandlerError   = wire.ServiceErrorCodeHandlerError
	ServiceErrorCodeInvalidResult  = wire.ServiceErrorCodeInvalidResult
	ServiceErrorCodeUnknownVersion = wire.ServiceErrorCodeUnknownVersion
	ServiceErrorCodeUnknownService = wire.ServiceErrorCodeUnknownService
	ServiceErrorCodeUnknownMethod  = wire.ServiceErrorCodeUnknownMethod
	ServiceErrorCodeWrongMode      = wire.ServiceErrorCodeWrongMode
)

// ServiceErrorCodeValues returns every member of ServiceErrorCode in declaration order, in a new slice.
func ServiceErrorCodeValues() []ServiceErrorCode { return wire.ServiceErrorCodeValues() }
