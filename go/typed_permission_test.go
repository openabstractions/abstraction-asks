package asks

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

func typedFixture(b *Book, all bool, bundle bool) TypedPermission {
	selector := rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindOne, Target: []byte("{\"name\":\"test-secret\"}\n")}
	if all {
		selector = rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindAll}
	}
	p := TypedPermission{Effect: rwire.PermissionEffect{Subject: rwire.Subject{Account: b.owner, Program: filepath.Join(os.TempDir(), "typed-test.exe")}, Selections: []rwire.PermissionSelection{{Permission: rwire.PermissionKey{Action: "abstraction.credentials/read", ResourceProfile: "abstraction.credentials/named@1"}, Selector: selector}}}, Origin: "application", Purpose: "Read the selected test secret", Display: wire.PermissionDisplay{Selections: []wire.PermissionDisplaySelection{{Operation: "Read a saved secret value", Target: "test-secret", Collection: "saved application credentials in this account", AllEffect: "present and future saved application credentials", Disclosure: "Returns the secret value to this application"}}}}
	if bundle {
		p.Effect.Selections = append(p.Effect.Selections, rwire.PermissionSelection{Permission: rwire.PermissionKey{Action: "abstraction.credentials/rotate", ResourceProfile: "abstraction.credentials/named@1"}, Selector: selector})
		p.Effect.Bundle = &rwire.BundleExpansion{BundleRef: "abstraction.credentials/full-control@1", DefinitionDigest: "sha256:" + strings.Repeat("a", 64), Selector: selector, Selections: append([]rwire.PermissionSelection(nil), p.Effect.Selections...)}
		p.Display.Selections = append(p.Display.Selections, wire.PermissionDisplaySelection{Operation: "Rotate a saved secret", Target: "test-secret", Collection: "saved application credentials in this account", AllEffect: "present and future saved application credentials"})
		p.Display.BundleLabel = "Full control version 1"
		p.Display.BundleDisclosure = "Read and rotate only; no later operation is included"
	}
	p.Digest = PermissionEffectDigest(p.Effect)
	return p
}

func TestTypedPermissionAllowOnceStaysWithItsServiceCall(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := typedFixture(b, false, false)
	check := func() error { return nil }
	if _, outcome, _ := b.AskTypedPermissionWithOnceAuthorizedFrom("owner", "app", "first", p, listen.Seen{}, Publisher{}, check); outcome != "invalid" {
		t.Fatal("an application-origin request acquired a once choice", outcome)
	}
	p.Origin = "service"
	first, outcome, err := b.AskTypedPermissionWithOnceAuthorizedFrom("owner", "service", "first", p, listen.Seen{}, Publisher{}, check)
	if outcome != "pending" || err != nil || !slices.Equal(first.Options, []string{"once", "allow", "refuse"}) {
		t.Fatal(first, outcome, err)
	}
	second, outcome, err := b.AskTypedPermissionWithOnceAuthorizedFrom("owner", "service", "second", p, listen.Seen{}, Publisher{}, check)
	if outcome != "pending" || err != nil || second.ID == first.ID {
		t.Fatal("once choices coalesced across calls", second, outcome, err)
	}
	ordinary, outcome, err := b.AskTypedPermissionFrom("owner", "service", "ordinary", p, listen.Seen{}, Publisher{})
	if outcome != "pending" || err != nil || !slices.Equal(ordinary.Options, []string{"allow", "refuse"}) {
		t.Fatal("ordinary choices changed", ordinary, outcome, err)
	}
	guard := func(Record, string) error { return nil }
	if _, outcome, err := b.AnswerTypedPermissionAsResponderAuthorized(first.ID, "allow", filepath.Join(t.TempDir(), "assistant.exe"), guard); err != nil || outcome != "invalid" {
		t.Fatal("responder answered a person-only occurrence", outcome, err)
	}
	if _, outcome, err := b.AnswerTypedPermissionIfPendingAuthorized(ordinary.ID, "once", guard); err != nil || outcome != "invalid" {
		t.Fatal("ordinary occurrence accepted an unoffered answer", outcome, err)
	}
	if _, outcome, err := b.AnswerTypedPermissionIfPendingAuthorized(first.ID, "once", guard); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	for _, test := range []struct{ key, status, option string }{{"first", "answered", "once"}, {"second", "pending", ""}} {
		row, status, err := b.ReplayTypedPermission("owner", test.key, p)
		if err != nil || status != test.status || row.Option != test.option {
			t.Fatal(test.key, status, row.Option, err)
		}
	}
}

func TestRefusedTypedAdmissionLogsOneBoundedLine(t *testing.T) {
	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := typedFixture(b, false, false)
	if _, outcome, err := b.AskTypedPermissionFrom("owner", "app", "key", p, listen.Seen{}, Publisher{}); err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	if buf.Len() != 0 {
		t.Fatalf("admitted request logged: %q", buf.String())
	}
	other := typedFixture(b, true, false)
	if _, outcome, err := b.AskTypedPermissionFrom("owner", "app", "key", other, listen.Seen{}, Publisher{}); err != nil || outcome != "conflict" {
		t.Fatal(outcome, err)
	}
	line := buf.String()
	if strings.Count(line, "\n") != 1 || !strings.Contains(line, "program=app") || !strings.Contains(line, "key=key") || !strings.Contains(line, "outcome=conflict") {
		t.Fatalf("log = %q", line)
	}
	for _, text := range []string{p.Purpose, "test-secret", "Read a saved secret value", "OA permission request"} {
		if strings.Contains(line, text) {
			t.Fatalf("log contains %q: %q", text, line)
		}
	}
	buf.Reset()
	if _, outcome, _ := b.AskTypedPermissionFrom("owner", "app", "", p, listen.Seen{}, Publisher{}); outcome != "invalid" || strings.Count(buf.String(), "\n") != 1 || !strings.Contains(buf.String(), "outcome=invalid") {
		t.Fatalf("invalid: %q %q", outcome, buf.String())
	}
}

func TestTypedPermissionImmutableReplayAndLegacyIsolation(t *testing.T) {
	for _, mode := range []struct {
		name        string
		all, bundle bool
	}{{"one", false, false}, {"all", true, false}, {"bundle", true, true}} {
		t.Run(mode.name, func(t *testing.T) {
			b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
			if err != nil {
				t.Fatal(err)
			}
			p := typedFixture(b, mode.all, mode.bundle)
			if !ValidTypedPermission(p) {
				t.Fatal("invalid fixture")
			}
			record, outcome, err := b.AskTypedPermissionFrom("typed-owner", "app", "key", p, listen.Seen{}, Publisher{})
			if err != nil || outcome != "pending" {
				t.Fatal(outcome, err)
			}
			// Mutating the caller's target/display after admission cannot alter the Book.
			p.Display.Selections[0].Operation = "mutated"
			if len(p.Effect.Selections[0].Selector.Target) > 0 {
				p.Effect.Selections[0].Selector.Target[0] = 'x'
			}
			saved, found, err := b.ApplicationRecord(record.ID)
			if err != nil || !found || saved.TypedPermission.Display.Selections[0].Operation == "mutated" {
				t.Fatal(saved, err)
			}
			if _, ok := PermissionForRecord(b.owner, saved); ok {
				t.Fatal("typed request became an exact legacy permission")
			}
			if _, _, err := b.AnswerLegacyApplicationAuthorized(record.ID, "allow", true, func() error { return nil }); !errors.Is(err, ErrTypedPermissionRequired) {
				t.Fatal("legacy answer accepted typed request", err)
			}
			if _, outcome, err := b.AnswerTypedPermissionIfPendingAuthorized(record.ID, "allow", func(Record, string) error { return nil }); err != nil || outcome != "answered" {
				t.Fatal(outcome, err)
			}
			if _, outcome, err := b.AnswerTypedPermissionIfPendingAuthorized(record.ID, "allow", func(Record, string) error { return nil }); err != nil || outcome != "conflict" {
				t.Fatal("repeated pending decision", outcome, err)
			}
			reopened, err := LoadApplicationBook(b.path)
			if err != nil {
				t.Fatal(err)
			}
			frozen := CloneTypedPermission(*saved.TypedPermission)
			frozen.Display.Selections[0].Operation = "new wording"
			replay, outcome, err := reopened.ReplayTypedPermission("typed-owner", "key", frozen)
			if err != nil || outcome != "answered" || replay.ID != record.ID || replay.TypedPermission.Display.Selections[0].Operation == "new wording" {
				t.Fatal("replay rewrote display", outcome, err)
			}
			changed := CloneTypedPermission(frozen)
			changed.Effect.Selections[0].Permission.Action = "abstraction.credentials/revoke"
			if changed.Effect.Bundle != nil {
				changed.Effect.Bundle.Selections = append([]rwire.PermissionSelection(nil), changed.Effect.Selections...)
			}
			changed.Digest = PermissionEffectDigest(changed.Effect)
			if _, outcome, err := reopened.ReplayTypedPermission("typed-owner", "key", changed); err != nil || outcome != "conflict" {
				t.Fatal("changed effect replay", outcome, err)
			}
		})
	}
}

func TestTypedPermissionRefusePacingAndInLockAdmission(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := typedFixture(b, true, true)
	r, _, err := b.AskTypedPermissionFrom("owner", "app", "first", p, listen.Seen{}, Publisher{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.AnswerTypedPermissionIfPendingAuthorized(r.ID, "refuse", func(Record, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	p.Purpose = "another purpose"
	shared, outcome, err := b.AskTypedPermissionFrom("owner", "app", "different-key", p, listen.Seen{}, Publisher{})
	if err != nil || outcome != "answered" || shared.ID != r.ID {
		t.Fatal("new key bypassed Refuse pacing", outcome, err)
	}
	b.now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	fresh, outcome, err := b.AskTypedPermissionFrom("owner", "app", "after-interval", p, listen.Seen{}, Publisher{})
	if err != nil || outcome != "pending" || fresh.ID == r.ID {
		t.Fatal(outcome, err)
	}
	denied := errors.New("caller stopped")
	if _, _, err := b.AskTypedPermissionAuthorizedFrom("owner", "app", "stopped", p, listen.Seen{}, Publisher{}, func() error { return denied }); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	if _, outcome, err := b.ObserveApplication("owner", "stopped"); err != nil || outcome != "unknown" {
		t.Fatal("stopped admission persisted", outcome, err)
	}
}

func TestTypedPermissionOccurrenceFreshRefusalAndPendingCoalescing(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := typedFixture(b, false, false)
	p.Origin, p.Purpose = "service", ""
	ask := func(key string) (Record, string, error) {
		return b.AskTypedPermissionOccurrenceAuthorizedFrom("owner", "app", key, p, listen.Seen{}, Publisher{}, func() error { return nil })
	}
	r, outcome, err := ask("call-1")
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	shared, outcome, err := ask("call-2")
	if err != nil || outcome != "pending" || shared.ID != r.ID {
		t.Fatal("pending calls did not coalesce", outcome, err)
	}
	if _, _, err := b.AnswerTypedPermissionIfPendingAuthorized(r.ID, "refuse", func(Record, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"call-1", "call-2"} {
		replay, outcome, err := ask(key)
		if err != nil || outcome != "answered" || replay.ID != r.ID || replay.Option != "refuse" {
			t.Fatal("lost occurrence replay", key, outcome, err)
		}
	}
	fresh, outcome, err := ask("call-3")
	if err != nil || outcome != "pending" || fresh.ID == r.ID {
		t.Fatal("fresh native call paced by old Refuse", outcome, err)
	}
	if _, outcome, _ := b.AskTypedPermissionOccurrenceAuthorizedFrom("owner", "app", "no-auth", p, listen.Seen{}, Publisher{}, nil); outcome != "invalid" {
		t.Fatal("missing trusted admission accepted", outcome)
	}
}

func TestTypedPermissionResponderProfileAndLegacyIsolation(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := typedFixture(b, false, false)
	r, _, err := b.AskTypedPermissionFrom("owner", "app", "responder", p, listen.Seen{}, Publisher{})
	if err != nil {
		t.Fatal(err)
	}
	actor := filepath.Join(t.TempDir(), "assistant.exe")
	if _, _, err := b.AnswerApplicationAsResponder(r.ID, "allow", actor, func(Record, string) error { return nil }); !errors.Is(err, ErrTypedPermissionRequired) {
		t.Fatal("legacy typed bypass", err)
	}
	answered, outcome, err := b.AnswerTypedPermissionAsResponderAuthorized(r.ID, "allow", actor, func(Record, string) error { return nil })
	if err != nil || outcome != "answered" || answered.AnsweredBy != AnsweredByResponder || answered.Answerer != actor {
		t.Fatal(answered, outcome, err)
	}
	raw, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeApplication(raw); err != nil {
		t.Fatal(err)
	}
	old := strings.Replace(string(raw), ApplicationProfile, shareApplicationProfile, 1)
	if _, err := decodeApplication([]byte(old)); err == nil {
		t.Fatal("old profile accepted typed responder")
	}
	var stored book
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	stored.Asks[0].Answerer = ""
	broken, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeApplication(broken); err == nil {
		t.Fatal("typed responder lost actor")
	}
}

func TestTypedPermissionVersion10OperatorMigratesWithoutActorChange(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := typedFixture(b, false, false)
	r, _, err := b.AskTypedPermissionFrom("owner", "runtime", "old-operator", p, listen.Seen{}, Publisher{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.AnswerTypedPermissionIfPendingAuthorized(r.ID, "allow", func(Record, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	old := strings.Replace(string(raw), ApplicationProfile, shareApplicationProfile, 1)
	if err := os.WriteFile(b.path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatal(err)
	}
	if _, outcome, err := reopened.AskTypedPermissionFrom("owner", "runtime", "new-call", p, listen.Seen{}, Publisher{}); err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	saved, found, err := reopened.ApplicationRecord(r.ID)
	if err != nil || !found || saved.AnsweredBy != AnsweredByOperator || saved.Answerer != "" || saved.Option != "allow" {
		t.Fatal(saved, err)
	}
	raw, err = os.ReadFile(b.path)
	if err != nil || !strings.Contains(string(raw), ApplicationProfile) {
		t.Fatal(string(raw), err)
	}
}

func TestTypedPermissionRefusesMalformedRetainedAuthority(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := typedFixture(b, true, true)
	for _, mutate := range []func(*TypedPermission){func(p *TypedPermission) { p.Display.Selections[0].AllEffect = "" }, func(p *TypedPermission) { p.Display.BundleDisclosure = "" }, func(p *TypedPermission) { p.Effect.Bundle.DefinitionDigest = "bad" }, func(p *TypedPermission) { p.Effect.Selections[0].Selector.Target = []byte{} }, func(p *TypedPermission) { p.Digest = "bad" }} {
		bad := CloneTypedPermission(p)
		mutate(&bad)
		if ValidTypedPermission(bad) {
			t.Fatal("malformed authority accepted")
		}
	}
	r, _, err := b.AskTypedPermissionFrom("owner", "app", "record", p, listen.Seen{}, Publisher{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := decodeApplication(raw)
	if err != nil {
		t.Fatal(err)
	}
	f.Asks[0].TypedPermission.Effect.Subject.Account = "foreign"
	f.Asks[0].TypedPermission.Digest = PermissionEffectDigest(f.Asks[0].TypedPermission.Effect)
	raw, _ = json.Marshal(f)
	if err := os.WriteFile(b.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	_, err = b.WithApplicationRecordContext(context.Background(), r.ID, func(Record, string, []string) error { called = true; return nil })
	if err == nil || called {
		t.Fatal("foreign target reached trusted callback", err)
	}
}
