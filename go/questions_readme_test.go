package asks

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

// update rewrites README.md's question catalogue from Questions instead of
// failing on a mismatch: go test ./... -run TestReadmeQuestionCatalogue -update
var update = flag.Bool("update", false, "rewrite the README question catalogue from questions.go")

const (
	readmePath     = "../README.md"
	tableStartMark = "<!-- questions-table:start -->"
	tableEndMark   = "<!-- questions-table:end -->"
)

// optionMeaning states what choosing one option allows, from Option.Yes and
// Option.Kept alone: whether the choice permits or refuses the asker, and
// whether the service remembers it or asks again next time.
func optionMeaning(o Option) string {
	verb := "refused"
	if o.Yes {
		verb = "permitted"
	}
	span := "now, asked again next time"
	if o.Kept {
		span = "permanently, remembered"
	}
	return fmt.Sprintf("`%s`: %s %s", o.Name, verb, span)
}

func questionRow(q Question) string {
	options := append(append([]Option{}, q.Options...), Never)
	meanings := make([]string, len(options))
	for i, o := range options {
		meanings[i] = optionMeaning(o)
	}
	return fmt.Sprintf("| `%s` | `%s` | %s |", q.Key, q.Text, strings.Join(meanings, " · "))
}

// expectedTable renders the catalogue block README.md must carry between
// tableStartMark and tableEndMark, generated from Questions.
func expectedTable() string {
	var b bytes.Buffer
	fmt.Fprintln(&b, tableStartMark)
	fmt.Fprintln(&b, "| id | wording shown to the person | what answering allows |")
	fmt.Fprintln(&b, "|---|---|---|")
	for _, q := range Questions {
		fmt.Fprintln(&b, questionRow(q))
	}
	fmt.Fprint(&b, tableEndMark)
	return b.String()
}

// TestReadmeQuestionCatalogueMatchesQuestionsGo fails when README.md's
// published question catalogue disagrees with Questions. Run with -update to
// regenerate the table in place instead of failing.
func TestReadmeQuestionCatalogueMatchesQuestionsGo(t *testing.T) {
	want := expectedTable()
	current, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(current, []byte("\r\n")) {
		want = strings.ReplaceAll(want, "\n", "\r\n")
	}
	start := bytes.Index(current, []byte(tableStartMark))
	end := bytes.Index(current, []byte(tableEndMark))
	if start < 0 || end < 0 || end < start {
		t.Fatalf("%s: missing %s .. %s markers", readmePath, tableStartMark, tableEndMark)
	}
	end += len(tableEndMark)
	got := string(current[start:end])
	if got == want {
		return
	}
	if *update {
		updated := string(current[:start]) + want + string(current[end:])
		if err := os.WriteFile(readmePath, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: question catalogue updated", readmePath)
		return
	}
	t.Errorf("%s: question catalogue is stale; rerun with -update, or paste:\n%s", readmePath, want)
}
