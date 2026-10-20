package asks

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openabstractions/abstraction-identity/listen"
)

func TestPermissionAdmissionRetainsLongTargetIndependentOfWording(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "asks.json"))
	if err != nil {
		t.Fatal(err)
	}
	primary := PermissionTarget{Account: b.owner, Program: filepath.Join(t.TempDir(), strings.Repeat("p", 240), "app.exe"), Action: "abstraction.model/lookup", Resource: strings.Repeat("r", 1024)}
	p, err := NewPermissionRequest(primary)
	if err != nil {
		t.Fatal(err)
	}
	r, outcome, err := b.AskPermissionFrom("runtime", "runtime", "long-target", p, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatalf("admit %+v %s %v", r, outcome, err)
	}
	if err := b.changeApplication(func(f *book) error {
		f.Asks[0].Text = "Allow this application to continue?"
		f.Asks[0].About = "a display label"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatal(err)
	}
	stored, found, err := reopened.ApplicationRecord(r.ID)
	if err != nil || !found {
		t.Fatal(found, err)
	}
	got, ok := PermissionForRecord(b.owner, stored)
	if !ok || got.Targets[0] != p.Targets[0] {
		t.Fatalf("display changed authority: %+v", got)
	}
	again, outcome, err := reopened.AskPermissionFrom("runtime", "runtime", "long-target", p, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" || again.ID != r.ID {
		t.Fatalf("replay %+v %s %v", again, outcome, err)
	}
	p.Targets[0].Resource = "different"
	if _, outcome, err := reopened.AskPermissionFrom("runtime", "runtime", "long-target", p, listen.Seen{}, Publisher{}); err != nil || outcome != "conflict" {
		t.Fatalf("changed target %s %v", outcome, err)
	}
}

func TestPinnedPermissionReadRefusesForeignCanonicalStateBeforeCallback(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "asks.json"))
	if err != nil {
		t.Fatal(err)
	}
	permission, err := NewPermissionRequest(PermissionTarget{Account: b.owner, Program: filepath.Join(t.TempDir(), "app.exe"), Action: "abstraction.model/lookup", Resource: "hf"})
	if err != nil {
		t.Fatal(err)
	}
	record, outcome, err := b.AskPermissionFrom("runtime", "runtime", "pin-account", permission, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	data, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := decodeApplication(data)
	if err != nil {
		t.Fatal(err)
	}
	f.Asks[0].Permission.Targets[0].Account = "foreign"
	foreign, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeApplication(foreign); err != nil {
		t.Fatalf("fixture is not canonical valid syntax: %v", err)
	}
	if err := os.WriteFile(b.path, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	found, err := b.WithApplicationRecordContext(context.Background(), record.ID, func(Record, string, []string) error { called = true; return nil })
	if err == nil || found || called {
		t.Fatalf("foreign record reached pin callback: found=%v called=%v err=%v", found, called, err)
	}
}

func TestPermissionRequestAllowsOnlyTheExistingFixedPair(t *testing.T) {
	primary := PermissionTarget{Account: "account", Program: filepath.Join(t.TempDir(), "app.exe"), Action: PermissionContentWrite, Resource: "input:audio"}
	p, err := NewPermissionRequest(primary)
	if err != nil || len(p.Targets) != 2 {
		t.Fatal(p, err)
	}
	for _, mutate := range []func(*PermissionRequest){
		func(p *PermissionRequest) { p.Targets = p.Targets[:1] },
		func(p *PermissionRequest) { p.Targets[1].Account = "another" },
		func(p *PermissionRequest) { p.Targets[1].Program = filepath.Join(t.TempDir(), "other.exe") },
		func(p *PermissionRequest) { p.Targets[1].Resource = "input:image" },
		func(p *PermissionRequest) { p.Targets[1].Action = PermissionContentWrite },
		func(p *PermissionRequest) { p.Targets = append(p.Targets, p.Targets[0]) },
	} {
		altered := PermissionRequest{Targets: append([]PermissionTarget(nil), p.Targets...)}
		mutate(&altered)
		if ValidPermissionRequest(altered) {
			t.Fatalf("independent/missing effect accepted: %+v", altered)
		}
	}
}

func TestPermissionProfilesReadLegacyWithoutRewritingThenMigrate(t *testing.T) {
	for _, profile := range []string{previousApplicationProfile, answererApplicationProfile} {
		t.Run(profile, func(t *testing.T) {
			b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "asks.json"))
			if err != nil {
				t.Fatal(err)
			}
			program := filepath.Join(t.TempDir(), "app.exe")
			r, _, err := b.AskApplication("runtime", "runtime", "legacy", FirstUseKey, map[string]string{"program": program, "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(b.path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := decodeApplication(data)
			if err != nil {
				t.Fatal(err)
			}
			f.Profile = profile
			f.ChoiceTimeMs = 0
			f.Asks[0].Permission = nil
			legacy, err := json.Marshal(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(b.path, legacy, 0600); err != nil {
				t.Fatal(err)
			}
			reopened, err := LoadApplicationBook(b.path)
			if err != nil {
				t.Fatal(err)
			}
			stored, _, err := reopened.ApplicationRecord(r.ID)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := PermissionForRecord(b.owner, stored)
			if !ok || got.Targets[0].Program != program {
				t.Fatalf("legacy %+v", got)
			}
			after, _ := os.ReadFile(b.path)
			if !bytes.Equal(after, legacy) {
				t.Fatal("read rewrote legacy state")
			}
			if _, outcome, err := reopened.AnswerApplicationIfPendingAuthorized(r.ID, "allow", nil); err != nil || outcome != "answered" {
				t.Fatal(outcome, err)
			}
			migrated, _ := os.ReadFile(b.path)
			next, err := decodeApplication(migrated)
			if err != nil || next.Profile != ApplicationProfile || next.Asks[0].Permission == nil || next.Asks[0].Permission.Targets[0].Program != program {
				t.Fatalf("migration %+v %v", next, err)
			}
		})
	}
}

func TestPermissionProfileRefusesMissingMalformedAndForeignTargets(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "asks.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewPermissionRequest(PermissionTarget{Account: b.owner, Program: filepath.Join(t.TempDir(), "app.exe"), Action: PermissionContentWrite, Resource: "input:audio"})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = b.AskPermissionFrom("runtime", "runtime", "target", p, listen.Seen{}, Publisher{})
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []func(*book){func(f *book) { f.Asks[0].Permission = nil }, func(f *book) { f.Asks[0].Permission.Targets[1].Resource = "input:image" }, func(f *book) {
		f.Asks[0].Permission.Targets[0].Account = "foreign"
		f.Asks[0].Permission.Targets[1].Account = "foreign"
	}} {
		f, err := decodeApplication(original)
		if err != nil {
			t.Fatal(err)
		}
		mutation(&f)
		data, _ := json.Marshal(f)
		if err := os.WriteFile(b.path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadApplicationBook(b.path); err == nil {
			t.Fatalf("accepted invalid @3: %s", data)
		}
	}
	if _, ok := PermissionForRecord(b.owner, Record{Key: FirstUseKey, About: p.Targets[0].Program, Text: p.Targets[0].Program + " wants to " + PermissionContentWrite + " on input:audio"}); ok {
		t.Fatal("unmarked current record fell back to prose")
	}
}
