package asks

// Tier is a question's tier: who may answer it (ASK-R6;
// research/rights-model/PROPOSAL.md §7.1, VISION 2026-10-03). A first-use
// question takes the tier of the permission it decides, from the one table
// grants.FirstUseTier keeps; a question that table does not name is
// sensitive.
type Tier string

const (
	// TierRoutine is answered by the person, or by an answerer the person
	// enrolled (ASK-R5).
	TierRoutine Tier = "routine"
	// TierSensitive is answered by the person only; an answerer may pass it on.
	TierSensitive Tier = "sensitive"
	// TierAdministrative is answered by the person only: money, credentials,
	// new apps, network exposure, deleting data, and OpenAbstractions itself.
	TierAdministrative Tier = "administrative"
)

// Answerable reports whether an answerer, under any grant, may answer a
// question of tier t. Only a routine question is; every other is the
// person's (ASK-R6).
func (t Tier) Answerable() bool { return t == TierRoutine }
