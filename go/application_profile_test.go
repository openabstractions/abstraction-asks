package asks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openabstractions/abstraction-identity/listen"
)

func TestApplicationAdmissionRefusesInvalidScopeBeforeWriting(t *testing.T) {
	book, _, program := notifyBook(t)
	before, beforeErr := os.ReadFile(book.path)
	for _, scope := range []string{strings.Repeat("a", 129), strings.Repeat("é", 65), string([]byte{0xff}), "bad\x00scope", ""} {
		r, out, err := book.AskApplicationFrom(scope, "runtime", "key", AgentEnrollmentKey, map[string]string{"program": program, "title": "Fixture agent"}, listen.Seen{}, Publisher{})
		if err != nil || out != "invalid" || r.ID != "" {
			t.Fatal(out, err)
		}
		after, afterErr := os.ReadFile(book.path)
		if !bytes.Equal(before, after) || (beforeErr == nil) != (afterErr == nil) {
			t.Fatal("invalid scope changed Book bytes")
		}
	}
	for _, scope := range []string{strings.Repeat("a", 128), strings.Repeat("é", 64)} {
		r, out, err := book.AskApplicationFrom(scope, "runtime", "boundary", AgentEnrollmentKey, map[string]string{"program": program, "title": "Fixture agent"}, listen.Seen{}, Publisher{})
		if err != nil || out != "pending" {
			t.Fatal("valid scope boundary refused", out, err)
		}
		if _, found, err := book.ApplicationRecord(r.ID); err != nil || !found {
			t.Fatal("valid scope boundary unreadable", found, err)
		}
	}
	r, out, err := book.AskApplicationFrom("valid", "runtime", "valid", AgentEnrollmentKey, map[string]string{"program": program, "title": "Fixture agent"}, listen.Seen{}, Publisher{})
	if err != nil || out != "pending" {
		t.Fatal(out, err)
	}
	if _, found, err := book.ApplicationRecord(r.ID); err != nil || !found {
		t.Fatal("invalid scope poisoned Book", found, err)
	}
}

func TestApplicationVersionOneReadsAndMigratesOnWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.json")
	data, err := json.Marshal(&book{Profile: previousApplicationProfile})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.AskApplication("runtime", "runtime", "use-1", FirstUseKey,
		map[string]string{"program": filepath.Join(t.TempDir(), "example.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{}); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal(data, &profile); err != nil || profile.Profile != ApplicationProfile {
		t.Fatalf("not migrated: %s %v", data, err)
	}
	if _, err := LoadApplicationBook(path); err != nil {
		t.Fatalf("version 2 restart: %v", err)
	}
}

func TestApplicationVersionOneRefusesNewFieldAndMalformedFile(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"profile":"asks-application@1","admissions":null,"asks":null,"unknown":true}`),
		[]byte(`{"profile":"asks-application@1","admissions":null,"asks":null} {}`),
		[]byte(`{"profile":"asks-application@5","admissions":null,"asks":null}`),
	} {
		if _, err := decodeApplication(data); err == nil {
			t.Fatalf("accepted malformed/unknown profile: %s", data)
		}
	}
}
