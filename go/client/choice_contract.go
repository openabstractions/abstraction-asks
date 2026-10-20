package client

import wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"

type RequestRef = wire.RequestRef
type ChoiceOption = wire.ChoiceOption
type ChoiceRequest = wire.ChoiceRequest
type ChoiceAnswer = wire.ChoiceAnswer
type ChoiceObservation = wire.ChoiceObservation
type ChoiceAcknowledgement = wire.ChoiceAcknowledgement
type ChoiceRecord = wire.ChoiceRecord
type ChoiceRead = wire.ChoiceRead
type ChoicePage = wire.ChoicePage
type ChoiceDecision = wire.ChoiceDecision
type ChoiceState = wire.ChoiceState
type ChoiceOutcome = wire.ChoiceOutcome
type ChoiceAckOutcome = wire.ChoiceAckOutcome
type ChoiceDecisionOutcome = wire.ChoiceDecisionOutcome
type ChoiceRoute = wire.ChoiceRoute
type ChoiceDetails = wire.ChoiceDetails
type AnswerKind = wire.AnswerKind
type AnswerEntry = wire.AnswerEntry
type AnswerEntryPage = wire.AnswerEntryPage

const (
	AnswerKindQuestion = wire.AnswerKindQuestion
	AnswerKindChoice   = wire.AnswerKindChoice
)

// AnswerKindValues returns every member of AnswerKind in declaration order, in a new slice.
func AnswerKindValues() []AnswerKind { return wire.AnswerKindValues() }

type AnswerableChoice = wire.AnswerableChoice
type AnswerableChoicePage = wire.AnswerableChoicePage

const (
	ChoiceStatePending               = wire.ChoiceStatePending
	ChoiceStateAnswered              = wire.ChoiceStateAnswered
	ChoiceStateDismissed             = wire.ChoiceStateDismissed
	ChoiceStateExpired               = wire.ChoiceStateExpired
	ChoiceOutcomePending             = wire.ChoiceOutcomePending
	ChoiceOutcomeAnswered            = wire.ChoiceOutcomeAnswered
	ChoiceOutcomeDismissed           = wire.ChoiceOutcomeDismissed
	ChoiceOutcomeExpired             = wire.ChoiceOutcomeExpired
	ChoiceOutcomeGone                = wire.ChoiceOutcomeGone
	ChoiceOutcomeUnknown             = wire.ChoiceOutcomeUnknown
	ChoiceOutcomeConflict            = wire.ChoiceOutcomeConflict
	ChoiceOutcomeInvalid             = wire.ChoiceOutcomeInvalid
	ChoiceOutcomeExhausted           = wire.ChoiceOutcomeExhausted
	ChoiceOutcomeForbidden           = wire.ChoiceOutcomeForbidden
	ChoiceOutcomeUnavailable         = wire.ChoiceOutcomeUnavailable
	ChoiceAckOutcomeAcknowledged     = wire.ChoiceAckOutcomeAcknowledged
	ChoiceAckOutcomeConflict         = wire.ChoiceAckOutcomeConflict
	ChoiceAckOutcomeUnknown          = wire.ChoiceAckOutcomeUnknown
	ChoiceAckOutcomeExpired          = wire.ChoiceAckOutcomeExpired
	ChoiceAckOutcomeInvalid          = wire.ChoiceAckOutcomeInvalid
	ChoiceAckOutcomeForbidden        = wire.ChoiceAckOutcomeForbidden
	ChoiceAckOutcomeUnavailable      = wire.ChoiceAckOutcomeUnavailable
	ChoiceDecisionOutcomeAnswered    = wire.ChoiceDecisionOutcomeAnswered
	ChoiceDecisionOutcomeDismissed   = wire.ChoiceDecisionOutcomeDismissed
	ChoiceDecisionOutcomeExpired     = wire.ChoiceDecisionOutcomeExpired
	ChoiceDecisionOutcomeGone        = wire.ChoiceDecisionOutcomeGone
	ChoiceDecisionOutcomeUnknown     = wire.ChoiceDecisionOutcomeUnknown
	ChoiceDecisionOutcomeConflict    = wire.ChoiceDecisionOutcomeConflict
	ChoiceDecisionOutcomeInvalid     = wire.ChoiceDecisionOutcomeInvalid
	ChoiceDecisionOutcomeForbidden   = wire.ChoiceDecisionOutcomeForbidden
	ChoiceDecisionOutcomeUnavailable = wire.ChoiceDecisionOutcomeUnavailable
	ChoiceRouteRequester             = wire.ChoiceRouteRequester
	ChoiceRouteOperator              = wire.ChoiceRouteOperator
	ChoiceRouteResponder             = wire.ChoiceRouteResponder
)

func ChoiceRouteValues() []ChoiceRoute { return wire.ChoiceRouteValues() }

func ChoiceStateValues() []ChoiceState                     { return wire.ChoiceStateValues() }
func ChoiceOutcomeValues() []ChoiceOutcome                 { return wire.ChoiceOutcomeValues() }
func ChoiceAckOutcomeValues() []ChoiceAckOutcome           { return wire.ChoiceAckOutcomeValues() }
func ChoiceDecisionOutcomeValues() []ChoiceDecisionOutcome { return wire.ChoiceDecisionOutcomeValues() }
