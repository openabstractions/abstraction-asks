package client

import wireRights "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"

// These owner contract types are reached through the typed Asks records. The
// service-error vocabulary remains the Asks service's existing wire vocabulary.
type PermissionKey = wireRights.PermissionKey
type ResourceSelector = wireRights.ResourceSelector
type ResourceSelectorKind = wireRights.ResourceSelectorKind
type PermissionSelection = wireRights.PermissionSelection
type PermissionEffect = wireRights.PermissionEffect
type BundleExpansion = wireRights.BundleExpansion
type Subject = wireRights.Subject

// Decision and DecisionOutcome are what RequestAnsweringContext returns (ASK-R10).
type Decision = wireRights.Decision
type DecisionOutcome = wireRights.DecisionOutcome

func DecisionOutcomeValues() []DecisionOutcome { return wireRights.DecisionOutcomeValues() }

const (
	ResourceSelectorKindOne = wireRights.ResourceSelectorKindOne
	ResourceSelectorKindAll = wireRights.ResourceSelectorKindAll
)

func ResourceSelectorKindValues() []ResourceSelectorKind {
	return wireRights.ResourceSelectorKindValues()
}
