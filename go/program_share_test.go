package asks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

// ASK-P5 (asks P0-3): program A admits ordinary choices with distinct
// references until it reads exhausted; program B's next choice is admitted.
func TestOneProgramCannotExhaustOrdinaryChoices(t *testing.T) {
	b, clock := ordinaryBook(t)
	dir := t.TempDir()
	a, other := filepath.Join(dir, "nagware.exe"), filepath.Join(dir, "editor.exe")
	admitted := 0
	for i := 0; i <= MaxOrdinaryAdmissions; i++ {
		r, err := b.AskChoiceAs("scope-a", a, ordinaryRequest(*clock, fmt.Sprint(i)), nil, listen.Seen{}, Publisher{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if r.Outcome == wire.ChoiceOutcomeExhausted {
			break
		}
		if r.Outcome != wire.ChoiceOutcomePending {
			t.Fatal(r)
		}
		admitted++
	}
	if admitted == 0 || admitted > programOrdinaryShare {
		t.Fatalf("program A admitted %d choices; its share is %d", admitted, programOrdinaryShare)
	}
	r, err := b.AskChoiceAs("scope-b", other, ordinaryRequest(*clock, "b"), nil, listen.Seen{}, Publisher{}, nil)
	if err != nil || r.Outcome != wire.ChoiceOutcomePending {
		t.Fatalf("program B was refused after A's share filled: %+v %v", r, err)
	}
	if replay, err := b.AskChoiceAs("scope-a", a, ordinaryRequest(*clock, "0"), nil, listen.Seen{}, Publisher{}, nil); err != nil || replay.Outcome != wire.ChoiceOutcomePending {
		t.Fatalf("program A's admitted choice no longer replays: %+v %v", replay, err)
	}
}

// withoutProgramHashes removes every version 10 program fingerprint, so a
// test can read the same state under an earlier version.
func withoutProgramHashes(data []byte) []byte {
	return regexp.MustCompile(`,"program_hash":"[0-9a-f]{64}"`).ReplaceAll(withoutCatalogueContent(data), nil)
}

// Older-profile fixtures omit the content introduced in version 13.
func withoutCatalogueContent(data []byte) []byte {
	var f book
	if err := json.Unmarshal(data, &f); err != nil {
		panic(err)
	}
	dropCatalogueContent(&f)
	out, err := json.Marshal(f)
	if err != nil {
		panic(err)
	}
	return out
}
func dropCatalogueContent(f *book) {
	for _, r := range f.Asks {
		r.Content = nil
	}
	for i := range f.Answers {
		f.Answers[i].Content = nil
	}
}

// dropProgramHashes is withoutProgramHashes for a decoded book.
func dropProgramHashes(f *book) {
	dropCatalogueContent(f)
	for i := range f.Admissions {
		f.Admissions[i].ProgramHash = ""
	}
	for i := range f.Choices {
		f.Choices[i].ProgramHash = ""
	}
}

// ASK-P5: version 10 alone carries program fingerprints; a version 9 file
// reads, and its next edit gives each retained record's admission its
// program's fingerprint, which stays after the record is retired.
func TestProgramHashesMigrateFromVersion9(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "editor.exe")
	r, _, err := b.AskApplicationAs("scope", program, "one", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{}, Publisher{})
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	want := []byte(`"program_hash":"` + programHash(program) + `"`)
	if err != nil || !bytes.Contains(current, want) {
		t.Fatalf("the admission must keep its program's fingerprint: %s %v", current, err)
	}
	if _, err := decodeApplication(bytes.Replace(current, []byte(`"`+ApplicationProfile+`"`), []byte(`"asks-application@9"`), 1)); err == nil {
		t.Fatal("version 9 must refuse a program fingerprint")
	}
	other := bytes.Replace(current, want, []byte(`"program_hash":"`+programHash(program+"x")+`"`), 1)
	if _, err := decodeApplication(other); err == nil {
		t.Fatal("a retained record's admission must carry its own program's fingerprint")
	}
	v9 := withoutProgramHashes(bytes.Replace(current, []byte(`"`+ApplicationProfile+`"`), []byte(`"asks-application@9"`), 1))
	if err := os.WriteFile(path, v9, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, outcome, err := b.RetireApplicationAuthorized(r.ID, nil); err != nil || outcome != "retired" {
		t.Fatal(outcome, err)
	}
	migrated, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(migrated, []byte(`"profile":"`+ApplicationProfile+`"`)) || !bytes.Contains(migrated, want) {
		t.Fatalf("version 9 must migrate with its admission's fingerprint: %s %v", migrated, err)
	}
}

// ASK-P5: an acknowledged choice's tombstone stays charged to the program
// that asked. Program A decides and acknowledges each of its own choices
// until it reads exhausted, within its share; program B's next choice is
// admitted, and a reopened Book reads the same.
func TestAcknowledgedChoicesStayInTheAskersShare(t *testing.T) {
	b, clock := ordinaryBook(t)
	dir := t.TempDir()
	a, other := filepath.Join(dir, "nagware.exe"), filepath.Join(dir, "editor.exe")
	admitted := 0
	// Twice the share is enough to tell: a tombstone charged elsewhere lets A
	// go on past its share.
	for i := 0; i < 2*programOrdinaryShare; i++ {
		q := ordinaryRequest(*clock, fmt.Sprint(i))
		r, err := b.AskChoiceAs("scope-a", a, q, nil, listen.Seen{}, Publisher{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if r.Outcome == wire.ChoiceOutcomeExhausted {
			break
		}
		if r.Outcome != wire.ChoiceOutcomePending {
			t.Fatal(r)
		}
		admitted++
		d, err := b.DecideOwnChoiceAuthorized("scope-a", q.RequestRef, "keep", a, false, true, nil)
		if err != nil || d.Outcome != wire.ChoiceDecisionOutcomeAnswered || d.Record == nil || d.Record.Answer == nil {
			t.Fatalf("decide %d: %+v %v", i, d, err)
		}
		if ack, err := b.AcknowledgeChoice("scope-a", q.RequestRef, *d.Record.Answer, nil); err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
			t.Fatalf("acknowledge %d: %+v %v", i, ack, err)
		}
	}
	if admitted == 0 || admitted > programOrdinaryShare {
		t.Fatalf("program A admitted %d acknowledged choices; its share is %d", admitted, programOrdinaryShare)
	}
	r, err := b.AskChoiceAs("scope-b", other, ordinaryRequest(*clock, "b"), nil, listen.Seen{}, Publisher{}, nil)
	if err != nil || r.Outcome != wire.ChoiceOutcomePending {
		t.Fatalf("program B was refused after A's acknowledged choices: %+v %v", r, err)
	}
	reopened, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatal(err)
	}
	reopened.now = b.now
	if r, err := reopened.AskChoiceAs("scope-a", a, ordinaryRequest(*clock, "after-restart"), nil, listen.Seen{}, Publisher{}, nil); err != nil || r.Outcome != wire.ChoiceOutcomeExhausted {
		t.Fatalf("program A after restart: %+v %v", r, err)
	}
}

// ASK-P5: a retired question's admission stays charged to the program the
// question concerned, so retiring program A's questions does not move them
// into the admitting scope's count.
func TestRetiredQuestionsStayInTheProgramsShare(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "nagware.exe")
	if _, outcome, err := b.AskApplicationAs("runtime", a, "seed", "download.reach", map[string]string{"host": "seed"}, listen.Seen{}, Publisher{}); err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	// Fill A's share but a few with retained questions through the provider
	// edit; the public ask and retire below decide at the bound.
	const room = 16
	b.mu.Lock()
	err = b.changeApplication(func(f *book) error {
		base := *f.Asks[0]
		for i := 1; i < programApplicationShare-room; i++ {
			r := base
			r.ID = fmt.Sprintf("filled-%d", i)
			f.Asks = append(f.Asks, &r)
			f.Admissions = append(f.Admissions, applicationAdmission{Scope: "runtime", Key: r.ID, Fingerprint: fmt.Sprintf("%064d", i), ID: r.ID})
		}
		return nil
	})
	b.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	admitted := 0
	for i := 0; i < 2*room; i++ {
		key := fmt.Sprintf("q-%d", i)
		r, outcome, err := b.AskApplicationAs("runtime", a, key, "download.reach", map[string]string{"host": key}, listen.Seen{}, Publisher{})
		if errors.Is(err, ErrApplicationCapacity) {
			break
		}
		if err != nil || outcome != "pending" {
			t.Fatal(outcome, err)
		}
		admitted++
		if _, outcome, err := b.RetireApplicationAuthorized(r.ID, nil); err != nil || outcome != "retired" {
			t.Fatal(outcome, err)
		}
	}
	if admitted == 0 || admitted > room {
		t.Fatalf("program A admitted %d retired questions with room for %d in its share", admitted, room)
	}
	if _, outcome, err := b.AskApplicationAs("runtime", filepath.Join(dir, "editor.exe"), "b", "download.reach", map[string]string{"host": "b"}, listen.Seen{}, Publisher{}); err != nil || outcome != "pending" {
		t.Fatalf("program B in the same scope after A's retired questions: %s %v", outcome, err)
	}
}

// ASK-P5: the same for questions (application@1) and typed permission
// requests (permission@1): program A reads exhausted at its share of the
// legacy and typed partition, and program B is admitted.
func TestOneProgramCannotExhaustQuestionsOrTypedRequests(t *testing.T) {
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a, other := filepath.Join(dir, "nagware.exe"), filepath.Join(dir, "editor.exe")
	ask := func(program, scope, key string) (string, error) {
		_, outcome, err := b.AskApplicationAs(scope, program, key, "download.reach", map[string]string{"host": key}, listen.Seen{}, Publisher{})
		if errors.Is(err, ErrApplicationCapacity) {
			return "exhausted", nil
		}
		return outcome, err
	}
	if outcome, err := ask(a, "scope-a", "seed"); err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	// Fill A's share but one through the provider edit, as the capacity
	// tests do; the public admissions below decide at the bound.
	b.mu.Lock()
	err = b.changeApplication(func(f *book) error {
		base := *f.Asks[0]
		for i := 1; i < programApplicationShare-1; i++ {
			r := base
			r.ID = fmt.Sprintf("filled-%d", i)
			f.Asks = append(f.Asks, &r)
			f.Admissions = append(f.Admissions, applicationAdmission{Scope: "scope-a", Key: r.ID, Fingerprint: fmt.Sprintf("%064d", i), ID: r.ID})
		}
		return nil
	})
	b.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := ask(a, "scope-a", "last"); err != nil || outcome != "pending" {
		t.Fatalf("program A's last question within its share: %s %v", outcome, err)
	}
	if outcome, err := ask(a, "scope-a", "over"); err != nil || outcome != "exhausted" {
		t.Fatalf("program A past its share: %s %v", outcome, err)
	}
	if outcome, err := ask(other, "scope-b", "b"); err != nil || outcome != "pending" {
		t.Fatalf("program B after A's share filled: %s %v", outcome, err)
	}

	typed := func(program, key string) (string, error) {
		p := typedFixture(b, false, false)
		p.Effect.Subject.Program = program
		p.Effect.Selections[0].Selector = rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindOne, Target: []byte(fmt.Sprintf("{\"name\":%q}\n", key))}
		p.Display.Selections[0].Target = key
		p.Digest = PermissionEffectDigest(p.Effect)
		_, outcome, err := b.AskTypedPermissionFrom("typed-"+filepath.Base(program), "app", key, p, listen.Seen{}, Publisher{})
		if errors.Is(err, ErrApplicationCapacity) {
			return "exhausted", nil
		}
		return outcome, err
	}
	if outcome, err := typed(a, "secret-a"); err != nil || outcome != "exhausted" {
		t.Fatalf("program A's typed request past its share: %s %v", outcome, err)
	}
	if outcome, err := typed(other, "secret-b"); err != nil || outcome != "pending" {
		t.Fatalf("program B's typed request after A's share filled: %s %v", outcome, err)
	}
}
