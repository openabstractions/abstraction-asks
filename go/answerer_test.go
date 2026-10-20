package asks

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/openabstractions/abstraction-identity/listen"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

// ASK-R7: a script host is refused as an answerer by its signed original
// file name or by its own file name; another program passes.
func TestScriptHostSubjectNamesAHostByItsSignatureOrItsName(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		program   string
		publisher Publisher
		host      bool
	}{
		{filepath.Join(dir, "python.exe"), Publisher{}, true},
		{filepath.Join(dir, "node.exe"), Publisher{}, true},
		{filepath.Join(dir, "opencode.exe"), Publisher{}, false},
		// A signed copy renamed after a client is still its signer's host.
		{filepath.Join(dir, "opencode.exe"), Publisher{Subject: "OpenJS Foundation", Trusted: true, OriginalName: "node.exe"}, true},
		// An untrusted signature names nothing.
		{filepath.Join(dir, "opencode.exe"), Publisher{Subject: "Someone", OriginalName: "node.exe"}, false},
	}
	if runtime.GOOS == "windows" {
		// The file system compares names without case on Windows.
		cases = append(cases, struct {
			program   string
			publisher Publisher
			host      bool
		}{filepath.Join(dir, "PowerShell.EXE"), Publisher{}, true})
	}
	for _, c := range cases {
		if got := ScriptHostSubject(rwire.Subject{Program: c.program}, c.publisher); got != c.host {
			t.Errorf("ScriptHostSubject(%q, %+v) = %v, want %v", c.program, c.publisher, got, c.host)
		}
	}
}

// ASK-R6: only a routine question is answerable by an answerer.
func TestOnlyARoutineTierIsAnswerable(t *testing.T) {
	for tier, want := range map[Tier]bool{TierRoutine: true, TierSensitive: false, TierAdministrative: false, "": false, "later": false} {
		if tier.Answerable() != want {
			t.Errorf("%q answerable = %v", tier, tier.Answerable())
		}
	}
}

// ASK-O7: a responder's answer records its actor; the state file refuses an
// actor beside an operator's answer or a pending question.
func TestTheBookRecordsTheAnswererOfAResponderAnswer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ask := func(key string) Record {
		r, _, err := b.AskApplication("scope", "app", key, FirstUseKey,
			map[string]string{"program": filepath.Join(t.TempDir(), "comfyui.exe"), "action": "abstraction.storage/content.write", "resource": "output:image"}, listen.Seen{})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	first := ask("first")
	if _, outcome, _ := b.AnswerApplicationAsResponder(first.ID, "allow", "relative.exe", nil); outcome != "invalid" {
		t.Fatalf("a relative answerer: %s", outcome)
	}
	answered, outcome, err := b.AnswerApplicationAsResponder(first.ID, "allow", exe, nil)
	if err != nil || outcome != "answered" || answered.Answerer != exe || answered.AnsweredBy != AnsweredByResponder {
		t.Fatalf("%+v %s %v", answered, outcome, err)
	}
	reopened, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	if r, ok, err := reopened.ApplicationRecord(first.ID); err != nil || !ok || r.Answerer != exe {
		t.Fatalf("after a restart %+v %v %v", r, ok, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	forged := strings.Replace(string(raw), `"answered_by":"responder"`, `"answered_by":"operator"`, 1)
	if forged == string(raw) {
		t.Fatal("the fixture did not record a responder answer")
	}
	if err := os.WriteFile(path, []byte(forged), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApplicationBook(path); err == nil {
		t.Fatal("an answerer beside an operator's answer was accepted")
	}
}
