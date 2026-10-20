package asks

import "github.com/openabstractions/abstraction-asks/go/catalogue"

type PresentationFamily = catalogue.PresentationFamily

const PresentationInvalid = catalogue.PresentationInvalid
const PresentationQuestion = catalogue.PresentationQuestion
const PresentationApplicationChoice = catalogue.PresentationApplicationChoice
const PresentationConsent = catalogue.PresentationConsent
const PresentationPairingConsent = catalogue.PresentationPairingConsent

func PresentationForQuestion(key string) PresentationFamily {
	return catalogue.PresentationForQuestion(key)
}
func AutomaticNativeQuestion(key string) bool { return catalogue.AutomaticNativeQuestion(key) }
func PresentationForApplicationChoice() PresentationFamily {
	return catalogue.PresentationForApplicationChoice()
}
