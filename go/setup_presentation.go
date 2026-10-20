package asks

import "github.com/openabstractions/abstraction-asks/go/catalogue"

type SetupPresentation = catalogue.SetupPresentation

func SetupRequestSentence(app, use string) string { return catalogue.SetupRequestSentence(app, use) }
func SetupGigabytes(n int64) string               { return catalogue.SetupGigabytes(n) }
func SetupCannotRunSentence(pool int64, smallest string, need int64) string {
	return catalogue.SetupCannotRunSentence(pool, smallest, need)
}
func SetupQuestionPresentation(slots map[string]string) (SetupPresentation, error) {
	return catalogue.SetupQuestionPresentation(slots)
}
func SetupPresentationForFacts(facts SetupQuestionFacts) (SetupPresentation, error) {
	return catalogue.SetupPresentationForFacts(facts)
}

// QuestionView is catalogue.QuestionView.
type QuestionView = catalogue.QuestionView

// QuestionPresentation is catalogue.QuestionPresentation: a catalogue
// question's native popup words from its retained slots.
func QuestionPresentation(key, asker string, slots map[string]string) (QuestionView, bool) {
	return catalogue.QuestionPresentation(key, asker, slots)
}
