//go:build oa_identitytest

package application

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/identitytest"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestNativeSubjectReceiverCanonicalReplayAndRefusals(t *testing.T) {
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	b, err := asks.LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := &receiver{host: &Host{book: b, owner: account.Uid}}
	var facts []identitytest.Facts
	for _, version := range []string{"1.0.0.0", "2.0.0.0"} {
		root := filepath.Join(t.TempDir(), version)
		if err = os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "app.exe")
		if err = os.WriteFile(path, []byte("inert"), 0600); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, identitytest.Facts{User: identity.User{Kind: "windows", SID: account.Uid}, Path: path, Package: "SubjectFixture_" + version + "_x64__pzs8sxrjxfjjc", PackageRoot: root})
	}
	facts = append(facts, identitytest.Facts{User: identity.User{Kind: "windows", SID: account.Uid}}, identitytest.Facts{User: identity.User{Kind: "windows", SID: "foreign"}, Path: facts[0].Path})
	peers := identitytest.New(t, facts...)
	legacyScope, observed, err := callerScope(peers[0], account.Uid)
	if err != nil {
		t.Fatal(err)
	}
	legacy, outcome, err := b.AskApplicationAs(legacyScope, observed, "legacy-key", "download.reach", map[string]string{"host": "legacy.example"}, listen.Seen{}, asks.Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	if _, err = b.Answer(legacy.ID, "once"); err != nil {
		t.Fatal(err)
	}
	view, err := r.nativeBookForPeer(peers[0])
	if err != nil {
		t.Fatal(err)
	}
	old, outcome, err := view.AskApplicationAs(observed, "legacy-key", "download.reach", map[string]string{"host": "legacy.example"}, listen.Seen{}, asks.Publisher{})
	if err != nil || outcome != "answered" || old.ID != legacy.ID {
		t.Fatal("legacy scope fingerprint changed", old, outcome, err)
	}
	var first string
	for i, peer := range peers[:2] {
		view, e := r.nativeBookForPeer(peer)
		if e != nil {
			t.Fatal(e)
		}
		record, outcome, e := view.AskApplicationAs(facts[i].Path, "key", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{}, asks.Publisher{})
		if e != nil || outcome != "pending" {
			t.Fatal(outcome, e)
		}
		if i == 0 {
			first = record.ID
		} else if record.ID != first {
			t.Fatal("native package version opened another occurrence")
		}
	}
	for _, peer := range peers[2:] {
		if _, e := r.nativeBookForPeer(peer); e == nil {
			t.Fatal("unproven native provenance fell back to legacy admission")
		}
	}
}
