package typedpermission

import (
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	rights "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
	"strings"
	"testing"
)

func TestPermissionBundleKeepsCanonicalSHA256Only(t *testing.T) {
	selection := rights.PermissionSelection{Permission: rights.PermissionKey{Action: "owner/read", ResourceProfile: "named@1"}, Selector: rights.ResourceSelector{Kind: rights.ResourceSelectorKindOne, Target: []byte("target")}}
	p := Permission{Effect: rights.PermissionEffect{Subject: rights.Subject{Account: "account", Program: "/program"}, Selections: []rights.PermissionSelection{selection}, Bundle: &rights.BundleExpansion{BundleRef: "owner/read@1", Selector: selection.Selector, Selections: []rights.PermissionSelection{selection}}}, Origin: "application", Display: wire.PermissionDisplay{BundleLabel: "Read bundle", BundleDisclosure: "Read the named target", Selections: []wire.PermissionDisplaySelection{{Operation: "Read", Target: "target"}}}}
	validators := Validators{NativeProgram: func(program string) bool { return program == "/program" }, RemoteProgram: func(string) bool { return false }}
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"sha256:" + strings.Repeat("a", 64), true},
		{"sha256:" + strings.Repeat("A", 64), false},
		{"SHA256:" + strings.Repeat("a", 64), false},
		{strings.Repeat("a", 64), false},
		{"sha256:" + strings.Repeat("a", 63), false},
		{"sha256:" + strings.Repeat("a", 65), false},
		{"sha512:" + strings.Repeat("a", 128), false},
	} {
		p.Effect.Bundle.DefinitionDigest = tc.value
		p.Digest = PermissionEffectDigest(p.Effect)
		if got := ValidPermission(p, validators); got != tc.valid {
			t.Fatalf("bundle digest %q: %v", tc.value, got)
		}
	}
}

func TestPermissionRequiresOwningNativeValidator(t *testing.T) {
	effect := rights.PermissionEffect{Subject: rights.Subject{Account: "account", Program: "/program"}, Selections: []rights.PermissionSelection{{Permission: rights.PermissionKey{Action: "owner/read", ResourceProfile: "named@1"}, Selector: rights.ResourceSelector{Kind: rights.ResourceSelectorKindOne, Target: []byte("target")}}}}
	p := Permission{Effect: effect, Digest: PermissionEffectDigest(effect), Origin: "application", Display: wire.PermissionDisplay{Selections: []wire.PermissionDisplaySelection{{Operation: "Read", Target: "target"}}}}
	if ValidPermission(p, Validators{}) || ValidPermission(p, Validators{NativeProgram: func(string) bool { return false }, RemoteProgram: func(string) bool { return false }}) {
		t.Fatal("missing/unproved native syntax accepted")
	}
	if !ValidPermission(p, Validators{NativeProgram: func(program string) bool { return program == "/program" }, RemoteProgram: func(string) bool { return false }}) {
		t.Fatal("owning validator refused")
	}
	p.Effect.Selections[0].Selector.Target[0] ^= 1
	if ValidPermission(p, Validators{NativeProgram: func(string) bool { return true }, RemoteProgram: func(string) bool { return false }}) {
		t.Fatal("changed effect accepted")
	}
}

func TestPermissionDetailsRequireBothNativeOwners(t *testing.T) {
	for _, validators := range []Validators{{}, {RemoteProgram: func(string) bool { return true }}, {NativeProgram: func(string) bool { return true }}} {
		if _, err := CheckDetails(wire.TypedPermissionDetails{}, "account", "question", validators); err == nil {
			t.Fatal("missing native owner accepted")
		}
	}
}
