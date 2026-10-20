package asks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/openabstractions/abstraction-identity/listen"
)

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
