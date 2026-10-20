package asks

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

func shareChoice(t *testing.T, b *Book, requester string, q wire.ChoiceRequest, shared ...string) wire.ChoiceObservation {
	t.Helper()
	r, err := b.AskChoiceAs("app-scope", requester, q, shared, listen.Seen{}, Publisher{}, nil)
	if err != nil || r.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(r, err)
	}
	return r
}

func allow(ChoiceCandidate) error { return nil }

// An assistant's answer keeps an audit entry beyond acknowledgement and
// replay expiry, for seven days after the answer (ASK-O13).
func TestDelegatedAnswerSurvivesAcknowledgementAndAgesOut(t *testing.T) {
	b, clock := ordinaryBook(t)
	requester, answerer := choiceActorPath(t, "app.exe"), choiceActorPath(t, "assistant.exe")
	q := ordinaryRequest(*clock, "audit")
	pending := shareChoice(t, b, requester, q, "keep")
	var seen ChoiceCandidate
	decision, recorded, err := b.AnswerSharedChoice(pending.ID, "keep", answerer, func(c ChoiceCandidate) error { seen = c; return nil })
	if err != nil || !recorded || decision.Outcome != wire.ChoiceDecisionOutcomeAnswered || seen.AskerProgram != requester || len(seen.Shared) != 1 {
		t.Fatal(decision, recorded, err, seen)
	}
	if _, recorded, err := b.AnswerSharedChoice(pending.ID, "keep", answerer, allow); err != nil || recorded {
		t.Fatal("a replay must not record again", err)
	}
	if conflict, _, err := b.AnswerSharedChoice(pending.ID, "keep", choiceActorPath(t, "other.exe"), allow); err != nil || conflict.Outcome != wire.ChoiceDecisionOutcomeConflict {
		t.Fatal("another answerer must conflict", conflict, err)
	}
	attributed, found, err := b.ChoiceAttributed(pending.ID)
	if err != nil || !found || attributed.Route != ChoiceRouteResponder || attributed.AskerProgram != requester {
		t.Fatal(attributed, found, err)
	}
	if ack, err := b.AcknowledgeChoice("app-scope", q.RequestRef, *decision.Record.Answer, nil); err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
		t.Fatal(ack, err)
	}
	if _, found, err := b.ChoiceAttributed(pending.ID); err != nil || found {
		t.Fatal("an acknowledged choice keeps its record", found, err)
	}
	*clock = clock.Add(2 * time.Hour) // past the replay deadline
	answers, since, _, err := b.AuditedAnswers()
	if err != nil || len(answers) != 1 || since != 0 || answers[0].ID != pending.ID || answers[0].Label != "Keep" || answers[0].Answerer != answerer || answers[0].AskerProgram != requester {
		t.Fatal(answers, since, err)
	}
	// A real edit after seven days drops the entry.
	*clock = clock.Add(7 * 24 * time.Hour)
	shareChoice(t, b, requester, ordinaryRequest(*clock, "later"), "keep")
	if answers, _, _, err := b.AuditedAnswers(); err != nil || len(answers) != 0 {
		t.Fatal("an entry older than seven days stays", answers, err)
	}
}

// The audit keeps at most MaxDelegatedAnswers entries; eviction records the
// time up to which the list is incomplete.
func TestDelegatedAnswerBoundEvictsOldestAndRecordsSince(t *testing.T) {
	b, clock := ordinaryBook(t)
	requester, answerer := choiceActorPath(t, "app.exe"), choiceActorPath(t, "assistant.exe")
	var first int64
	for i := range MaxDelegatedAnswers + 2 {
		*clock = clock.Add(time.Second)
		q := ordinaryRequest(*clock, fmt.Sprintf("bound-%d", i))
		pending := shareChoice(t, b, requester, q, "keep")
		d, _, err := b.AnswerSharedChoice(pending.ID, "keep", answerer, allow)
		if err != nil || d.Outcome != wire.ChoiceDecisionOutcomeAnswered {
			t.Fatal(i, d, err)
		}
		if i == 1 {
			first = d.Record.Answer.AnsweredMs
		}
		if _, err := b.AcknowledgeChoice("app-scope", q.RequestRef, *d.Record.Answer, nil); err != nil {
			t.Fatal(err)
		}
	}
	answers, since, _, err := b.AuditedAnswers()
	if err != nil || len(answers) != MaxDelegatedAnswers || since != first || answers[len(answers)-1].AnsweredMs <= since {
		t.Fatalf("len=%d since=%d first=%d %v", len(answers), since, first, err)
	}
	if answers[0].AnsweredMs < answers[1].AnsweredMs {
		t.Fatal("audit must read newest first")
	}
}

// Sharing is checked by the Book: an unshared choice or option is refused
// before the answerer check runs, and a refusal records nothing.
func TestSharedChoiceRefusalsRecordNothing(t *testing.T) {
	b, clock := ordinaryBook(t)
	requester, answerer := choiceActorPath(t, "app.exe"), choiceActorPath(t, "assistant.exe")
	unshared := admitChoice(t, b, ordinaryRequest(*clock, "unshared"))
	shared := shareChoice(t, b, requester, ordinaryRequest(*clock, "shared"), "keep", "cancel")
	called := false
	check := func(ChoiceCandidate) error { called = true; return nil }
	for _, c := range []struct{ id, option string }{{unshared.ID, "keep"}, {shared.ID, "replace"}} {
		if _, _, err := b.AnswerSharedChoice(c.id, c.option, answerer, check); !errors.Is(err, ErrChoiceNotShared) || called {
			t.Fatal(c, err, called)
		}
	}
	refused := errors.New("not enrolled")
	if _, _, err := b.AnswerSharedChoice(shared.ID, "keep", answerer, func(ChoiceCandidate) error { return refused }); !errors.Is(err, refused) {
		t.Fatal(err)
	}
	if candidates, _, err := b.SharedChoices(); err != nil || len(candidates) != 1 || candidates[0].Record.ID != shared.ID {
		t.Fatal("a refused answer changed the pending choice", candidates, err)
	}
	if answers, _, _, err := b.AuditedAnswers(); err != nil || len(answers) != 0 {
		t.Fatal(answers, err)
	}
	for _, bad := range [][]string{{}, {"absent"}, {"keep", "keep"}} {
		if r, err := b.AskChoiceAs("app-scope", requester, ordinaryRequest(*clock, "bad"), bad, listen.Seen{}, Publisher{}, nil); err != nil || r.Outcome != wire.ChoiceOutcomeInvalid {
			t.Fatal(bad, r, err)
		}
	}
}

// A version 6 file with ordinary choices reads and migrates on its next
// edit; a version 6 file cannot carry the version 7 fields.
func TestApplicationVersion7Migration(t *testing.T) {
	b, clock := ordinaryBook(t)
	old := admitChoice(t, b, ordinaryRequest(*clock, "old"))
	current, err := os.ReadFile(b.path)
	if err != nil || !bytes.Contains(current, []byte(`"profile":"asks-application@9"`)) {
		t.Fatalf("%s %v", current, err)
	}
	v6 := bytes.Replace(current, []byte(`"asks-application@9"`), []byte(`"asks-application@6"`), 1)
	if err := os.WriteFile(b.path, v6, 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatalf("version 6 must read: %v", err)
	}
	reopened.now = b.now
	shareChoice(t, reopened, choiceActorPath(t, "app.exe"), ordinaryRequest(*clock, "new"), "keep")
	migrated, err := os.ReadFile(b.path)
	if err != nil || !bytes.Contains(migrated, []byte(`"profile":"asks-application@9"`)) {
		t.Fatalf("version 6 must migrate on edit: %s %v", migrated, err)
	}
	if attributed, found, err := reopened.ChoiceAttributed(old.ID); err != nil || !found || attributed.AskerProgram != "" || attributed.Shared != nil || attributed.Route != "" {
		t.Fatal("an earlier choice gains no attribution", attributed, err)
	}
	downgraded := bytes.Replace(migrated, []byte(`"asks-application@9"`), []byte(`"asks-application@6"`), 1)
	if _, err := decodeApplication(downgraded); err == nil {
		t.Fatal("version 6 must refuse shared-choice fields")
	}
}
