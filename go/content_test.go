package asks

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestCatalogueContentSurvivesTextChangeRetirementAndCallerMutation(t *testing.T) {
	b, clock := ordinaryBook(t)
	program := choiceActorPath(t, "content-app.exe")
	slots := map[string]string{"program": program, "title": "Original title"}
	registration, outcome, err := b.AskApplication("runtime", "runtime", "registration", SelfRegisteredKey, slots, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatal(registration, outcome, err)
	}
	slots["title"] = "caller changed its map"
	registration.Content.Slots["title"] = "caller changed returned content"
	permission, err := NewPermissionRequest(PermissionTarget{Account: b.owner, Program: program, Action: PermissionContentWrite, Resource: "output:image"})
	if err != nil {
		t.Fatal(err)
	}
	firstUse, _, err := b.AskPermissionFrom("runtime", "runtime", "permission", permission, listen.Seen{}, Publisher{})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.changeApplication(func(f *book) error {
		for _, record := range f.Asks {
			record.Text = "Different catalogue wording, without the former English grammar."
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, record := range []Record{registration, firstUse} {
		option := "keep"
		if record.Key == FirstUseKey {
			option = "allow"
		}
		if _, outcome, err := b.AnswerApplication(record.ID, option); err != nil || outcome != "answered" {
			t.Fatal(outcome, err)
		}
		if _, outcome, err := b.RetireApplicationAuthorized(record.ID, nil); err != nil || outcome != "retired" {
			t.Fatal(outcome, err)
		}
		*clock = clock.Add(time.Second)
	}
	answers, _, _, err := b.AuditedAnswerContents()
	if err != nil || len(answers) != 2 {
		t.Fatal(answers, err)
	}
	var registered, permitted *wire.CatalogueContent
	for _, answer := range answers {
		if answer.Answer.Key == SelfRegisteredKey {
			registered = answer.Content
		}
		if answer.Answer.Key == FirstUseKey {
			permitted = answer.Content
		}
	}
	if registered == nil || registered.Slots["title"] != "Original title" || permitted == nil || permitted.Permission == nil || len(permitted.Permission.Targets) != 2 || permitted.Permission.Targets[0] != permission.Targets[0] {
		t.Fatal(answers)
	}
	registered.Slots["title"] = "changed read"
	permitted.Permission.Targets[0].Resource = "foreign"
	again, _, _, err := b.AuditedAnswerContents()
	if err != nil || again[0].Content.Permission.Targets[0].Resource != "output:image" || again[1].Content.Slots["title"] != "Original title" {
		t.Fatal(again, err)
	}
	legacy, _, _, err := b.AuditedAnswers()
	if err != nil || legacy[0] != again[0].Answer {
		t.Fatal(legacy, err)
	}
}

func TestCatalogueContentLegacyReadsStayAbsentAndMigrateOnEdit(t *testing.T) {
	b, _ := ordinaryBook(t)
	program := choiceActorPath(t, "old-app.exe")
	r, _, err := b.AskApplication("runtime", "runtime", "old", SelfRegisteredKey, map[string]string{"program": program, "title": "Old title"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.AnswerApplication(r.ID, "keep"); err != nil {
		t.Fatal(err)
	}
	f, err := b.readApplication()
	if err != nil {
		t.Fatal(err)
	}
	f.Profile = nativeApplicationProfile
	for _, record := range f.Asks {
		record.Content = nil
	}
	for i := range f.Answers {
		f.Answers[i].Content = nil
	}
	raw, _ := json.Marshal(f)
	if err := os.WriteFile(b.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	read, found, err := b.ApplicationRecord(r.ID)
	if err != nil || !found || read.Content != nil {
		t.Fatal(read, found, err)
	}
	answers, _, _, err := b.AuditedAnswerContents()
	if err != nil || len(answers) != 1 || answers[0].Content != nil {
		t.Fatal(answers, err)
	}
	after, _ := os.ReadFile(b.path)
	if !bytes.Equal(raw, after) {
		t.Fatal("read migrated the private profile")
	}
	if _, outcome, err := b.AskApplication("runtime", "runtime", "old", SelfRegisteredKey, map[string]string{"program": program, "title": "Old title"}, listen.Seen{}); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	after, _ = os.ReadFile(b.path)
	if !bytes.Equal(raw, after) {
		t.Fatal("idempotent replay migrated without a real Book edit")
	}
	if _, _, err := b.AskApplication("app", "app", "new", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{}); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(b.path)
	if !bytes.Contains(after, []byte(`"profile":"`+ApplicationProfile+`"`)) {
		t.Fatal("real edit did not migrate")
	}
	if read, _, err := b.ApplicationRecord(r.ID); err != nil || read.Content != nil {
		t.Fatal("legacy content was fabricated", read, err)
	}
}

func TestCatalogueContentMalformedCurrentAndDowngradedProfilesRefuse(t *testing.T) {
	b, _ := ordinaryBook(t)
	program := choiceActorPath(t, "app.exe")
	if _, _, err := b.AskApplication("runtime", "runtime", "registration", SelfRegisteredKey, map[string]string{"program": program, "title": "Title"}, listen.Seen{}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(b.path)
	for _, change := range []func(*book){
		func(f *book) { f.Asks[0].Content.Slots = nil },
		func(f *book) { f.Asks[0].Content.Slots["title"] = strings.Repeat("x", 201) },
		func(f *book) { f.Asks[0].Content.Slots["program"] = "different" },
		func(f *book) { f.Profile = nativeApplicationProfile },
	} {
		f, err := decodeApplication(raw)
		if err != nil {
			t.Fatal(err)
		}
		change(&f)
		bad, _ := json.Marshal(f)
		if _, err := decodeApplication(bad); err == nil {
			t.Fatal("accepted malformed or downgraded content", string(bad))
		}
	}
}
