//go:build oa_identitytest

package application

import (
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/identitytest"
	"os"
	"path/filepath"
	"testing"
)

func TestSubjectIdentityChoiceActorAcrossPackageVersions(t *testing.T) {
	root := t.TempDir()
	facts := []identitytest.Facts{}
	for _, version := range []string{"1.0.0.0", "2.0.0.0"} {
		folder := filepath.Join(root, version)
		if err := os.MkdirAll(folder, 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(folder, "app.exe")
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, identitytest.Facts{User: identity.User{Kind: "windows", SID: "owner"}, Path: path, Package: "SubjectFixture_" + version + "_x64__pzs8sxrjxfjjc", PackageRoot: folder})
	}
	peers := identitytest.New(t, facts...)
	first, err := choiceActorFromPeer(peers[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := choiceActorFromPeer(peers[1])
	if err != nil || first != second || first != "msix:SubjectFixture_pzs8sxrjxfjjc" {
		t.Fatal(first, second, err)
	}
}

func TestSubjectIdentityChoiceActorRefusesUnknownAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.exe")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	peer := identitytest.New(t, identitytest.Facts{User: identity.User{Kind: "posix", UID: -1}, Path: path})[0]
	if _, err := choiceActorFromPeer(peer); err == nil {
		t.Fatal("negative uid actor accepted")
	}
}

func TestSubjectIdentityUnknownUIDScopeRefused(t *testing.T) {
	peer := identitytest.New(t, identitytest.Facts{User: identity.User{Kind: "posix", UID: -1}})[0]
	if scope, _, err := callerScope(peer, "-1"); err == nil || scope != "" {
		t.Fatal(scope, err)
	}
}
