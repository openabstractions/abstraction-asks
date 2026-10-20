package asks

import (
	"path/filepath"
	"testing"

	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

// TestApplicationAdmissionRecordsPublisher: the admitting service records the
// requester's publisher with the question (ASK-A6). A replay keeps the
// evidence of the first admission, the file keeps it across a reload, and a
// subject a person could be misled by is not recorded.
func TestApplicationAdmissionRecordsPublisher(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	slots := map[string]string{"host": "example.com"}
	signer := Publisher{Subject: "Test Signer", Trusted: true, Description: "Test Program", OriginalName: "test.exe"}
	first, outcome, err := b.AskApplicationFrom("scope", "app", "first", "download.reach", slots, listen.Seen{}, signer)
	if err != nil || outcome != "pending" || first.Publisher != "Test Signer" || !first.PublisherTrusted ||
		first.ProgramDescription != "Test Program" || first.ProgramOriginalName != "test.exe" {
		t.Fatalf("admission must record the publisher: %+v %q %v", first, outcome, err)
	}
	replay, outcome, err := b.AskApplicationFrom("scope", "app", "first", "download.reach", slots, listen.Seen{}, Publisher{Subject: "Other Signer"})
	if err != nil || outcome != "pending" || replay.ID != first.ID || replay.Publisher != "Test Signer" || !replay.PublisherTrusted ||
		replay.ProgramDescription != "Test Program" {
		t.Fatalf("a replay must keep the first admission's publisher: %+v %q %v", replay, outcome, err)
	}
	reopened, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	stored, found, err := reopened.ApplicationRecord(first.ID)
	if err != nil || !found || stored.Publisher != "Test Signer" || !stored.PublisherTrusted ||
		stored.ProgramDescription != "Test Program" || stored.ProgramOriginalName != "test.exe" {
		t.Fatalf("the recorded publisher must survive a reload: %+v %v %v", stored, found, err)
	}
	misleading, _, err := b.AskApplicationFrom("scope", "app", "second", "download.reach", map[string]string{"host": "second.example"}, listen.Seen{},
		Publisher{Subject: "Good‮Signer", Trusted: true, Description: "Good‮Program", OriginalName: "good.exe"})
	if err != nil || misleading.Publisher != "" || !misleading.PublisherTrusted || misleading.ProgramDescription != "" || misleading.ProgramOriginalName != "good.exe" {
		t.Fatalf("a subject or description with a direction override must not be recorded: %+v %v", misleading, err)
	}
	untrusted, _, err := b.AskApplicationFrom("scope", "app", "third", "download.reach", map[string]string{"host": "third.example"}, listen.Seen{},
		Publisher{Subject: "Claimed", Description: "Windows PowerShell", OriginalName: "PowerShell.EXE"})
	if err != nil || untrusted.ProgramDescription != "" || untrusted.ProgramOriginalName != "" {
		t.Fatalf("a program named without a trusted signature must not be recorded: %+v %v", untrusted, err)
	}
	if _, found, err := b.ApplicationRecord("0123456789abcdef"); err != nil || found {
		t.Fatalf("an absent id must read as not found: %v %v", found, err)
	}
}

func TestCodePublisher(t *testing.T) {
	for _, tc := range []struct {
		code identity.Code
		want Publisher
	}{
		{identity.Code{Subject: "Johannes Schindelin", Trusted: true}, Publisher{Subject: "Johannes Schindelin", Trusted: true}},
		{identity.Code{Trusted: true, Catalog: true}, Publisher{Trusted: true}},
		{identity.Code{Status: "no signature"}, Publisher{}},
		{identity.Code{Subject: "line\nbreak", Trusted: true}, Publisher{Trusted: true}},
		{identity.Code{Subject: "Microsoft Windows", Trusted: true, Catalog: true, Description: "Windows PowerShell", OriginalName: "PowerShell.EXE"},
			Publisher{Subject: "Microsoft Windows", Trusted: true, Description: "Windows PowerShell", OriginalName: "PowerShell.EXE"}},
		{identity.Code{Status: "no signature", Description: "Windows PowerShell", OriginalName: "PowerShell.EXE"}, Publisher{}},
		{identity.Code{Trusted: true, Description: "tab\there", OriginalName: "x.exe"}, Publisher{Trusted: true, OriginalName: "x.exe"}},
	} {
		if got := CodePublisher(tc.code); got != tc.want {
			t.Errorf("CodePublisher(%+v) = %+v, want %+v", tc.code, got, tc.want)
		}
	}
}
