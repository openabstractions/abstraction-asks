package asks

import "github.com/openabstractions/abstraction-asks/go/catalogue"

type SetupQuestionFacts = catalogue.SetupQuestionFacts
type SetupTargetFact = catalogue.SetupTargetFact
type SetupModelFact = catalogue.SetupModelFact
type SetupCodeFact = catalogue.SetupCodeFact
type SetupPictureFact = catalogue.SetupPictureFact
type SetupVideoFact = catalogue.SetupVideoFact
type SetupVideoSource = catalogue.SetupVideoSource
type SetupVideoPreset = catalogue.SetupVideoPreset

func SetupFactSlots(f SetupQuestionFacts) (map[string]string, error) {
	return catalogue.SetupFactSlots(f)
}
func SetupFactsFromSlots(slots map[string]string) (SetupQuestionFacts, error) {
	return catalogue.SetupFactsFromSlots(slots)
}
func SetupPermissionWords(kind string) string { return catalogue.SetupPermissionWords(kind) }

const SetupFallbackNoPackage = catalogue.SetupFallbackNoPackage
const SetupFallbackNoFeature = catalogue.SetupFallbackNoFeature

func SetupFallbackWords(kind string) string          { return catalogue.SetupFallbackWords(kind) }
func SetupFallbackFact(words string) (string, error) { return catalogue.SetupFallbackFact(words) }
