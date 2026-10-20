package asks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

// Every answer recorded through operator@1 or responder@1 joins the answer
// audit, and its entry outlives the question's record (ASK-O15): the person's
// own answers to catalogue questions and choices, an assistant's answers, and
// the self-registration Keep. A dismissal and an application's own decision
// record nothing; a replay records nothing again.
func TestAnswerAuditKeepsEveryRecordedAnswerPastItsRecord(t *testing.T) {
	b, clock := ordinaryBook(t)
	app, assistant := choiceActorPath(t, "comfyui.exe"), choiceActorPath(t, "opencode.exe")
	firstUse := func(key string) Record {
		r, _, err := b.AskApplication("scope", "comfyui.exe", key, FirstUseKey,
			map[string]string{"program": app, "action": "abstraction.storage/content.write", "resource": "output:image"}, listen.Seen{})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	refused := firstUse("first")
	*clock = clock.Add(time.Second)
	if _, outcome, err := b.AnswerApplication(refused.ID, "refuse"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	if _, outcome, err := b.AnswerApplication(refused.ID, "refuse"); err != nil || outcome != "answered" {
		t.Fatal("replay", outcome, err)
	}
	if _, outcome, err := b.RetireApplicationAuthorized(refused.ID, nil); err != nil || outcome != "retired" {
		t.Fatal(outcome, err)
	}
	*clock = clock.Add(time.Second)
	allowed := firstUse("second")
	if _, outcome, err := b.AnswerApplicationAsResponder(allowed.ID, "allow", assistant, nil); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	*clock = clock.Add(time.Second)
	kept, _, err := b.AskApplication("runtime", "openabstractions", "self", SelfRegisteredKey, map[string]string{"program": app, "title": "ComfyUI"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if _, outcome, err := b.AnswerApplicationIfPendingAuthorized(kept.ID, "keep", nil); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	*clock = clock.Add(time.Second)
	chosen := admitChoice(t, b, ordinaryRequest(*clock, "chosen"))
	if d, err := b.DecideChoiceAuthorized(chosen.ID, "keep", assistant, false, true, nil); err != nil || d.Outcome != wire.ChoiceDecisionOutcomeAnswered {
		t.Fatal(d, err)
	}
	dismissed := admitChoice(t, b, ordinaryRequest(*clock, "dismissed"))
	if d, err := b.DecideChoiceAuthorized(dismissed.ID, "", assistant, true, true, nil); err != nil || d.Outcome != wire.ChoiceDecisionOutcomeDismissed {
		t.Fatal(d, err)
	}
	own := ordinaryRequest(*clock, "own")
	admitChoice(t, b, own)
	if d, err := b.DecideOwnChoiceAuthorized("app-scope", own.RequestRef, "replace", app, false, true, nil); err != nil || d.Outcome != wire.ChoiceDecisionOutcomeAnswered {
		t.Fatal(d, err)
	}
	*clock = clock.Add(time.Second)
	shared := shareChoice(t, b, app, ordinaryRequest(*clock, "shared"), "keep")
	if _, recorded, err := b.AnswerSharedChoice(shared.ID, "keep", assistant, allow); err != nil || !recorded {
		t.Fatal(recorded, err)
	}

	answers, since, _, err := b.AuditedAnswers()
	if err != nil || since != 0 || len(answers) != 5 {
		t.Fatalf("%+v %d %v", answers, since, err)
	}
	want := []struct {
		id     string
		kind   wire.AnswerKind
		key    string
		option string
		label  string
		by     wire.Answerer
		answer string
	}{
		{shared.ID, wire.AnswerKindChoice, "", "keep", "Keep", wire.AnswererResponder, assistant},
		{chosen.ID, wire.AnswerKindChoice, "", "keep", "Keep", wire.AnswererOperator, ""},
		{kept.ID, wire.AnswerKindQuestion, SelfRegisteredKey, "keep", "keep", wire.AnswererOperator, ""},
		{allowed.ID, wire.AnswerKindQuestion, FirstUseKey, "allow", "allow", wire.AnswererResponder, assistant},
		{refused.ID, wire.AnswerKindQuestion, FirstUseKey, "refuse", "refuse", wire.AnswererOperator, ""},
	}
	for i, w := range want {
		a := answers[i]
		if a.ID != w.id || a.Kind != w.kind || a.Key != w.key || a.Option != w.option || a.Label != w.label || a.AnsweredBy != w.by || a.Answerer != w.answer || !ValidAnswerEntry(a) {
			t.Fatalf("entry %d: %+v, want %+v", i, a, w)
		}
		if i > 0 && a.AnsweredMs > answers[i-1].AnsweredMs {
			t.Fatal("the audit must read newest first")
		}
	}
	if program, action, resource, ok := FirstUse(answers[4].Key, answers[4].About, answers[4].Title); !ok || program != app || action != "abstraction.storage/content.write" || resource != "output:image" {
		t.Fatalf("a first-use entry must read back as its question: %+v", answers[4])
	}
	if program, title, ok := SelfRegistered(answers[2].Key, answers[2].About, answers[2].Title); !ok || program != app || title != "ComfyUI" {
		t.Fatalf("a self-registration entry must read back as its question: %+v", answers[2])
	}
	if answers[0].AskerProgram != app || answers[1].Title != "Replace this existing output for this delivery?" {
		t.Fatalf("%+v", answers[:2])
	}
	var delegated []wire.AnswerEntry
	for _, entry := range answers {
		if entry.Kind == wire.AnswerKindChoice && entry.AnsweredBy == wire.AnswererResponder {
			delegated = append(delegated, entry)
		}
	}
	if len(delegated) != 1 || delegated[0].ID != shared.ID || delegated[0].Answerer != assistant {
		t.Fatal("the delegated answers are the audit's responder choices", delegated)
	}
	// Seven days after an answer, a real edit drops its entry.
	*clock = clock.Add(7*24*time.Hour - 3*time.Second)
	admitChoice(t, b, ordinaryRequest(*clock, "later"))
	if answers, _, _, err := b.AuditedAnswers(); err != nil || len(answers) != 3 || answers[2].ID != kept.ID {
		t.Fatal("entries older than seven days stay", answers, err)
	}
}

// A version 8 file's delegated list reads as the answer audit and is written
// as version 9 on the next edit; only version 9 carries the audit, and it
// carries no delegated list.
func TestVersion8DelegatedAuditMovesIntoTheAnswerAudit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.json")
	answerer := choiceActorPath(t, "assistant.exe")
	at := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	old := book{Asks: []*Record{}, Profile: notificationApplicationProfile, ChoiceTimeMs: at.UnixMilli(), DelegatedSinceMs: at.Add(-time.Hour).UnixMilli(),
		Delegated: []legacyDelegatedAnswer{{ID: "d1", Asker: "app.exe", Title: "Which model?", Option: "small", Label: "Small", Answerer: answerer, AnsweredMs: at.UnixMilli()}}}
	raw, err := json.Marshal(&old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatalf("version 8 must read: %v", err)
	}
	b.now = func() time.Time { return at }
	answers, since, _, err := b.AuditedAnswers()
	if err != nil || len(answers) != 1 || since != old.DelegatedSinceMs || answers[0].Kind != wire.AnswerKindChoice || answers[0].AnsweredBy != wire.AnswererResponder || answers[0].Label != "Small" {
		t.Fatal(answers, since, err)
	}
	if delegated, _, _, err := b.AuditedAnswers(); err != nil || len(delegated) != 1 || delegated[0] != delegatedEntry(old.Delegated[0]) {
		t.Fatal(delegated, err)
	}
	admitChoice(t, b, ordinaryRequest(at, "edit"))
	migrated, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(migrated, []byte(`"profile":"asks-application@9"`)) || !bytes.Contains(migrated, []byte(`"answers":[`)) || bytes.Contains(migrated, []byte(`"delegated"`)) {
		t.Fatalf("version 8 must migrate on edit: %s %v", migrated, err)
	}
	if _, err := decodeApplication(bytes.Replace(migrated, []byte(`"asks-application@9"`), []byte(`"asks-application@8"`), 1)); err == nil {
		t.Fatal("version 8 must refuse the answer audit")
	}
	if _, err := decodeApplication(bytes.Replace(raw, []byte(`"asks-application@8"`), []byte(`"asks-application@9"`), 1)); err == nil {
		t.Fatal("version 9 must refuse a delegated list")
	}
}
