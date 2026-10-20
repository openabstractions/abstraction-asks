package asks

import (
	"slices"
	"strings"
)

// QuestionsPending reads the INF-W11 / ASK-A14 refusal words naming pending
// first-use questions. It preserves first occurrence order. These words name
// observations to make; they do not convey a permission decision.
func QuestionsPending(reason string) []string {
	var ids []string
	for _, word := range strings.Fields(reason) {
		if id, ok := strings.CutPrefix(word, "rights:question_pending:"); ok && id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}
