package asks

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/openabstractions/abstraction-identity/listen"
)

// TestApplicationRecordKeepsBoundAskerProgram: two programs with one file
// name ask; each record keeps its own bound program and shows the file name
// derived from it (ASK-A1, ASK-A2), across a restart.
func TestApplicationRecordKeepsBoundAskerProgram(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(t.TempDir(), "opencode.exe")
	second := filepath.Join(t.TempDir(), "opencode.exe")
	slots := map[string]string{"host": "example.com"}
	a, outcome, err := b.AskApplicationAs("scope-a", first, "same", "download.reach", slots, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	c, outcome, err := b.AskApplicationAs("scope-b", second, "same", "download.reach", slots, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	if a.Asker != "opencode.exe" || c.Asker != "opencode.exe" || a.AskerProgram != first || c.AskerProgram != second {
		t.Fatalf("records must keep their bound programs: %+v %+v", a, c)
	}
	restarted, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []Record{a, c} {
		got, found, err := restarted.ApplicationRecord(want.ID)
		if err != nil || !found || got.AskerProgram != want.AskerProgram || got.Asker != AskerDisplay(want.AskerProgram) {
			t.Fatalf("restart lost the asker program: %+v %v", got, err)
		}
	}
	replay, outcome, err := restarted.AskApplicationAs("scope-a", first, "same", "download.reach", slots, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" || replay.ID != a.ID || replay.AskerProgram != first {
		t.Fatalf("replay must return the admitted record: %+v %s %v", replay, outcome, err)
	}
	for _, program := range []string{"", "opencode.exe", filepath.Join(t.TempDir(), "a") + "\n", "msix:Example.App_8wekyb3d8bbwe"} {
		if _, outcome, err := b.AskApplicationAs("scope-c", program, "bad", "download.reach", slots, listen.Seen{}, Publisher{}); err != nil || outcome != "invalid" {
			t.Fatalf("program %q must be invalid: %s %v", program, outcome, err)
		}
	}
}

// TestApplicationAskerProgramProfile: version 6 and later carry an asker
// program, and its display name must be the one derived from it. A version 5
// file reads and migrates on its next edit (ASK-P1).
func TestApplicationAskerProgramProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "editor.exe")
	if _, _, err := b.AskApplicationAs("scope", program, "one", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{}, Publisher{}); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(current, []byte(`"profile":"asks-application@9"`)) {
		t.Fatalf("expected version 7: %s %v", current, err)
	}
	older := bytes.Replace(current, []byte(`"asks-application@9"`), []byte(`"asks-application@5"`), 1)
	if _, err := decodeApplication(older); err == nil {
		t.Fatal("version 5 must refuse an asker program")
	}
	renamed := bytes.Replace(current, []byte(`"asker":"editor.exe"`), []byte(`"asker":"other.exe"`), 1)
	if bytes.Equal(renamed, current) {
		t.Fatal("fixture did not change the asker")
	}
	if _, err := decodeApplication(renamed); err == nil {
		t.Fatal("an asker that is not derived from its program must be refused")
	}

	legacyPath := filepath.Join(t.TempDir(), "application.json")
	legacy, err := LoadApplicationBook(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	old, _, err := legacy.AskApplication("scope", "editor.exe", "old", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, bytes.Replace(data, []byte(`"asks-application@9"`), []byte(`"asks-application@5"`), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadApplicationBook(legacyPath)
	if err != nil {
		t.Fatalf("version 5 must read: %v", err)
	}
	if _, _, err := reopened.AskApplicationAs("scope-2", program, "new", "download.reach", map[string]string{"host": "example.org"}, listen.Seen{}, Publisher{}); err != nil {
		t.Fatal(err)
	}
	migrated, err := os.ReadFile(legacyPath)
	if err != nil || !bytes.Contains(migrated, []byte(`"profile":"asks-application@9"`)) {
		t.Fatalf("version 5 must migrate on edit: %s %v", migrated, err)
	}
	kept, found, err := reopened.ApplicationRecord(old.ID)
	if err != nil || !found || kept.AskerProgram != "" || kept.Asker != "editor.exe" {
		t.Fatalf("an earlier record keeps its display name and no program: %+v %v", kept, err)
	}
}
