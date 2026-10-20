package asks

import (
	"github.com/openabstractions/abstraction-identity/listen"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestFacts() SetupQuestionFacts {
	return SetupQuestionFacts{Targets: []SetupTargetFact{{Name: "editor", Title: "Editor", Program: "C:/Apps/editor.exe", Use: "code"}}, Code: &SetupCodeFact{Title: "Model", License: "Apache-2.0", State: "download"}}
}

func TestSetupFactsRetainedByBookAcrossNewOccurrenceAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "questions.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	f := setupTestFacts()
	f.Occurrence = strings.Repeat("a", 48)
	slots, err := SetupFactSlots(f)
	if err != nil {
		t.Fatal(err)
	}
	first, out, err := b.AskApplicationFrom("runtime", "runtime", "first", SetupModelsKey, slots, listen.Seen{}, Publisher{})
	if err != nil || out != "pending" {
		t.Fatal(out, err)
	}
	expected, err := SetupQuestionPresentation(slots)
	if err != nil {
		t.Fatal(err)
	}
	if _, out, err := b.AnswerApplicationIfPendingAuthorized(first.ID, "not_now", func() error { return nil }); err != nil || out != "answered" {
		t.Fatal(out, err)
	}
	f.Occurrence = strings.Repeat("b", 48)
	f.Code.Title = "New model"
	slots, err = SetupFactSlots(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.AskApplicationFrom("runtime", "runtime", "settings", SetupModelsKey, slots, listen.Seen{}, Publisher{}); err != nil {
		t.Fatal(err)
	}
	b, err = LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	old, found, err := b.ApplicationRecord(first.ID)
	if err != nil || !found || old.Content == nil {
		t.Fatal(found, err)
	}
	got, err := SetupQuestionPresentation(old.Content.Slots)
	if err != nil || got != expected {
		t.Fatal(got, err)
	}
	answers, _, _, err := b.AuditedAnswerContents()
	if err != nil || len(answers) != 1 || answers[0].Content == nil {
		t.Fatal(answers, err)
	}
	got, err = SetupQuestionPresentation(answers[0].Content.Slots)
	if err != nil || got != expected {
		t.Fatal(got, err)
	}
	// Returned maps are copies of the Book's immutable occurrence content.
	old.Content.Slots["code"] = "mutated"
	again, _, err := b.ApplicationRecord(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err = SetupQuestionPresentation(again.Content.Slots)
	if err != nil || got != expected {
		t.Fatal(got, err)
	}
}

func TestSetupFactsStrictDecodedRows(t *testing.T) {
	f := setupTestFacts()
	slots, err := SetupFactSlots(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetupFactsFromSlots(slots); err != nil {
		t.Fatal(err)
	}
	original := slots["target_01"]
	for _, control := range []string{`\n`, `\r`, `\u0000`, `\u001f`, `\u007f`, `\u0085`, `\u009f`} {
		t.Run(control, func(t *testing.T) {
			slots["target_01"] = strings.Replace(original, `"title":"Editor"`, `"title":"X`+control+`Y"`, 1)
			if _, err := SetupFactsFromSlots(slots); err == nil {
				t.Fatal("accepted decoded control")
			}
		})
	}
	for name, row := range map[string]string{
		"unknown":      strings.Replace(original, `"chat":false`, `"chat":false,"instruction":"run code"`, 1),
		"duplicate":    strings.Replace(original, `"title":"Editor"`, `"title":"Editor","title":"Other"`, 1),
		"wrong type":   strings.Replace(original, `"chat":false`, `"chat":"false"`, 1),
		"over carrier": strings.Replace(original, `"program":"C:/Apps/editor.exe"`, `"program":"`+strings.Repeat("x", 801)+`"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			slots["target_01"] = row
			if _, err := SetupFactsFromSlots(slots); err == nil {
				t.Fatal("accepted invalid row")
			}
		})
	}
	slots["target_01"] = original
	slots["target_17"] = original
	if _, err := SetupFactsFromSlots(slots); err == nil {
		t.Fatal("accepted seventeenth row")
	}
}

func TestSetupFactsDecodedFieldsAndSixteenRows(t *testing.T) {
	f := setupTestFacts()
	f.Targets[0].Program = "C:/Apps/" + strings.Repeat("x", 180) + "/editor.exe"
	longSlots, err := SetupFactSlots(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(longSlots["target_01"]) <= 200 || len(longSlots["target_01"]) > 800 {
		t.Fatal(len(longSlots["target_01"]))
	}
	if _, err := SetupFactsFromSlots(longSlots); err != nil {
		t.Fatal(err)
	}
	if validQuestionSlot(longSlots["target_01"]) {
		t.Fatal("generic catalogue limit widened")
	}
	for i := 1; i < 16; i++ {
		target := f.Targets[0]
		target.Name = strings.Repeat("x", i)
		f.Targets = append(f.Targets, target)
	}
	slots, err := SetupFactSlots(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 29 {
		t.Fatalf("slots=%d", len(slots))
	}
	if _, err := SetupFactsFromSlots(slots); err != nil {
		t.Fatal(err)
	}
	f.Targets = append(f.Targets, SetupTargetFact{Name: "extra", Title: "Extra", Use: "code"})
	if _, err := SetupFactSlots(f); err == nil {
		t.Fatal("accepted seventeen targets")
	}
	f = setupTestFacts()
	f.Code.Title = "X\nY"
	if _, err := SetupFactSlots(f); err == nil {
		t.Fatal("accepted code control")
	}
	f = setupTestFacts()
	f.Code.Ref.Family = "X\u0085Y"
	if _, err := SetupFactSlots(f); err == nil {
		t.Fatal("accepted reference control")
	}
}
