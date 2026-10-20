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
	switch o.Name {
	case "keep":
		return "`keep`: the app stays registered, confirmed"
	case "remove":
		return "`remove`: the app's entry is removed and it may not register itself again"
	}
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
	var meanings []string
	for _, name := range q.OptionNames() {
		o, _ := q.Option(name)
		meanings = append(meanings, optionMeaning(o))
	}
	return fmt.Sprintf("| `%s` | `%s` | %s |", q.Key, q.Text, strings.Join(meanings, " · "))
}

func TestContentReplaceQuestionHasExactServiceOwnedPromptAndOneTimeChoice(t *testing.T) {
	q, ok := Find("content.replace")
	if !ok {
		t.Fatal("content.replace missing from the fixed catalogue")
	}
	if q.Text != "{asker} requests a one-time replacement of {target} with {replacement}. Request {code} expires {until}." || q.About != "target" {
		t.Fatalf("unexpected service-owned question: %+v", q)
	}
	if len(q.Options) != 1 || q.Options[0] != (Option{Name: "once", Yes: true}) {
		t.Fatalf("content.replace must offer only a non-kept allow-once option: %+v", q.Options)
	}
	if option, ok := q.Option("never"); !ok || option != Never {
		t.Fatalf("the standard never refusal must remain available: %+v ok=%v", option, ok)
	}
	text, err := q.Render(Ask{Asker: "ComfyUI", Key: q.Key, Slots: map[string]string{
		"target": "model.safetensors", "replacement": "model-klein.safetensors", "code": "R7KD", "until": "2026-09-29 14:30 UTC",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "ComfyUI requests a one-time replacement of model.safetensors with model-klein.safetensors. Request R7KD expires 2026-09-29 14:30 UTC."
	if text != want {
		t.Fatalf("rendered prompt %q, want %q", text, want)
	}
}

func TestOpenCodeProjectQuestionIsOneTimeFolderAccess(t *testing.T) {
	q, ok := Find("opencode.project.edit")
	if !ok || q.About != "folder" || !strings.Contains(q.Text, "edit and delete") || !strings.Contains(q.Text, "including its subfolders") {
		t.Fatalf("folder disclosure missing: %+v", q)
	}
	once, ok := q.Option("once")
	if !ok || !once.Yes || once.Kept {
		t.Fatalf("folder permission must be one-time: %+v", once)
	}
	if _, ok := q.Option("allow"); ok {
		t.Fatal("folder grant offers a persistent allow")
	}
}

// TestSystemAccessQuestionIsOnceAndRuntimeOnly is ASK-S1: one-time, no
// persistent allow, names the administrator prompt, and only a runtime
// admits it.
func TestSystemAccessQuestionIsOnceAndRuntimeOnly(t *testing.T) {
	q, ok := Find(SystemSecurityLogKey)
	if !ok || q.About != "code" || !strings.Contains(q.Text, "Windows asks for administrator approval next") {
		t.Fatalf("system-access question: %+v", q)
	}
	once, ok := q.Option("once")
	if !ok || !once.Yes || once.Kept {
		t.Fatalf("once must be one-time: %+v", once)
	}
	if _, ok := q.Option("allow"); ok {
		t.Fatal("the system-access question offers a persistent allow")
	}
	if refuse, ok := q.Option("refuse"); !ok || refuse.Yes {
		t.Fatalf("refuse: %+v", refuse)
	}
	for _, key := range []string{FirstUseKey, TypedPermissionKey, SystemSecurityLogKey} {
		if !RuntimeOnlyKey(key) {
			t.Fatalf("%s is admitted by applications", key)
		}
	}
	if RuntimeOnlyKey("download.reach") {
		t.Fatal("an application question became runtime-only")
	}
	text, err := q.Render(Ask{Asker: "reader.exe", Key: q.Key, Slots: map[string]string{"code": "a1b2c3d4", "until": "2026-10-07T12:15:00Z"}})
	if err != nil || text != "reader.exe wants to read one page of the Windows Security log. Windows asks for administrator approval next. Request a1b2c3d4 expires 2026-10-07T12:15:00Z." {
		t.Fatalf("%q %v", text, err)
	}
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
