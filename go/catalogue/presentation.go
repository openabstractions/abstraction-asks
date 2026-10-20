package catalogue

// PresentationFamily is native-presenter metadata. It changes no admission,
// responder eligibility, answer meaning, wire record or permission effect.
type PresentationFamily uint8

const (
	PresentationInvalid PresentationFamily = iota
	PresentationQuestion
	PresentationApplicationChoice
	PresentationConsent
	PresentationPairingConsent
)

// Every catalogue row declares its semantic presentation here. Question keeps
// its published struct layout; native metadata is not persisted in the Book.
var questionPresentations = map[string]PresentationFamily{
	ProgramReviewKey:          PresentationConsent,
	SetupModelsKey:            PresentationConsent,
	ApplicationCommandOnceKey: PresentationConsent,
	AgentEnrollmentKey:        PresentationConsent,
	AgentUpdateKey:            PresentationConsent,
	PairingKey:                PresentationPairingConsent,
	"content.replace":         PresentationQuestion,
	"rights.register":         PresentationQuestion,
	projectEditQuestionKey:    PresentationQuestion,
	"download.reach":          PresentationQuestion,
	FirstUseKey:               PresentationConsent,
	SelfRegisteredKey:         PresentationConsent,
	SystemSecurityLogKey:      PresentationConsent,
	RightsChangeKey:           PresentationConsent,
	TypedPermissionKey:        PresentationConsent,
}

// PresentationForQuestion classifies retained questions read from the owning
// service. Runtime-only receiver rejection supplies the origin boundary;
// this display helper authenticates no caller. Unknown legacy/free prose stays
// ordinary; a known undeclared key is invalid rather than selecting a renderer.
func PresentationForQuestion(key string) PresentationFamily {
	if family, ok := questionPresentations[key]; ok {
		if (family == PresentationConsent || family == PresentationPairingConsent) && !RuntimeOnlyKey(key) {
			return PresentationInvalid
		}
		return family
	}
	if _, known := Find(key); known || RuntimeOnlyKey(key) {
		return PresentationInvalid
	}
	return PresentationQuestion
}

// AutomaticNativeQuestion is limited to service-reserved occurrences. The
// application receiver rejects these keys before admission. Catalogue wording
// and approval options do not make an application request a security prompt.
func AutomaticNativeQuestion(key string) bool {
	// Setup's session-claimed native step presents this exact Book occurrence.
	if key == SetupModelsKey {
		return false
	}
	family := PresentationForQuestion(key)
	return RuntimeOnlyKey(key) && (family == PresentationConsent || family == PresentationPairingConsent)
}

// Ordinary application@2 choices retain application-owned presentation policy.
func PresentationForApplicationChoice() PresentationFamily { return PresentationApplicationChoice }
