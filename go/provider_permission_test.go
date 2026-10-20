package asks

import (
	"github.com/openabstractions/abstraction-identity/listen"
	"path/filepath"
	"slices"
	"testing"
)

func TestProviderPermissionPairIsStructuralAndKeepsLegacyForms(t *testing.T) {
	primary := PermissionTarget{Account: "account", Program: filepath.Join(t.TempDir(), "app.exe"), Action: "abstraction.credentials/apply", Resource: "credential:removed-preset"}
	single, err := NewPermissionRequest(primary)
	if err != nil || len(single.Targets) != 1 || !ValidPermissionRequest(single) {
		t.Fatal(single, err)
	}
	companion := single.Targets[0]
	companion.Action = "abstraction.inference/complete"
	companion.Resource = "host:removed-preset"
	pair := PermissionRequest{Targets: append(slices.Clone(single.Targets), companion)}
	if name, ok := ProviderPermissionName(pair); !ok || name != "removed-preset" {
		t.Fatal(pair, name, ok)
	}
	for _, kind := range []string{"name", "subject", "account", "action", "reversed", "three"} {
		bad := PermissionRequest{Targets: slices.Clone(pair.Targets)}
		switch kind {
		case "name":
			bad.Targets[1].Resource = "host:other"
		case "subject":
			bad.Targets[1].Program = filepath.Join(t.TempDir(), "other.exe")
		case "account":
			bad.Targets[1].Account = "other"
		case "action":
			bad.Targets[1].Action = "abstraction.inference/host.manage"
		case "reversed":
			slices.Reverse(bad.Targets)
		case "three":
			bad.Targets = append(bad.Targets, companion)
		}
		if ValidPermissionRequest(bad) {
			t.Fatal(kind, bad)
		}
	}
	content := primary
	content.Action = PermissionContentWrite
	content.Resource = "input:image"
	paired, err := NewPermissionRequest(content)
	if err != nil || len(paired.Targets) != 2 || !ValidPermissionRequest(paired) {
		t.Fatal(paired, err)
	}
}

func TestProviderPermissionBookPreservesPairAcrossReloadWithoutCatalogueMembership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asks.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	primary := PermissionTarget{Account: b.owner, Program: filepath.Join(t.TempDir(), "app.exe"), Action: "abstraction.credentials/apply", Resource: "credential:removed-preset"}
	permission, err := NewPermissionRequest(primary)
	if err != nil {
		t.Fatal(err)
	}
	companion := permission.Targets[0]
	companion.Action, companion.Resource = "abstraction.inference/complete", "host:removed-preset"
	permission.Targets = append(permission.Targets, companion)
	record, outcome, err := b.AskPermissionFrom("runtime", "runtime", "provider-pair", permission, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" || record.Permission == nil || !slices.Equal(record.Permission.Targets, permission.Targets) {
		t.Fatal(record, outcome, err)
	}
	reloaded, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	got, outcome, err := reloaded.ObserveApplication("runtime", "provider-pair")
	if err != nil || outcome != "pending" || got.ID != record.ID || got.Permission == nil || !slices.Equal(got.Permission.Targets, permission.Targets) {
		t.Fatal(got, outcome, err)
	}
}
