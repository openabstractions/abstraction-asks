package asks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestOrdinaryOwnDecisionPreservesOperatorEvidenceAndAckBytes(t *testing.T) {
	for _, dismiss := range []bool{false, true} {
		t.Run(map[bool]string{false: "answer", true: "dismiss"}[dismiss], func(t *testing.T) {
			b, clock := ordinaryBook(t)
			q := ordinaryRequest(*clock, "evidence")
			first := admitChoice(t, b, q)
			initial, err := os.ReadFile(b.path)
			if err != nil {
				t.Fatal(err)
			}
			operatorPath := filepath.Join(t.TempDir(), "operator-book.json")
			if err := os.WriteFile(operatorPath, initial, 0600); err != nil {
				t.Fatal(err)
			}
			opBook, err := LoadApplicationBook(operatorPath)
			if err != nil {
				t.Fatal(err)
			}
			opBook.now = b.now
			actor, option := choiceActorPath(t, "caller.exe"), "keep"
			if dismiss {
				option = ""
			}
			own, err := b.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, option, actor, dismiss, !dismiss, nil)
			if err != nil || own.Record == nil {
				t.Fatal(own, err)
			}
			op, err := opBook.DecideChoiceAuthorized(first.ID, option, actor, dismiss, !dismiss, nil)
			if err != nil || !reflect.DeepEqual(op, own) {
				t.Fatal(op, own, err)
			}
			// The two decisions differ only in their recorded route (ASK-O12)
			// and the operator's answer in the answer audit (ASK-O15).
			if raw, err := os.ReadFile(operatorPath); err != nil {
				t.Fatal(err)
			} else if ownRaw, err := os.ReadFile(b.path); err != nil || !bytes.Contains(raw, []byte(`"route":"operator"`)) ||
				!bytes.Equal(bytes.Replace(withoutAudit(t, raw, !dismiss), []byte(`"route":"operator"`), []byte(`"route":"requester"`), 1), withoutAudit(t, ownRaw, false)) {
				t.Fatal("decision evidence diverged", err)
			}
			*clock = clock.Add(time.Second)
			restarted, err := LoadApplicationBook(b.path)
			if err != nil {
				t.Fatal(err)
			}
			restarted.now = b.now
			if replay, err := restarted.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, option, choiceActorPath(t, "later.exe"), dismiss, false, nil); err != nil || !reflect.DeepEqual(replay, own) {
				t.Fatal("reconciliation changed actor/time", replay, err)
			}
			if !dismiss {
				if stale, err := restarted.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, option, actor, false, true, nil); err != nil || stale.Outcome != wire.ChoiceDecisionOutcomeConflict || stale.Record != nil {
					t.Fatal(stale, err)
				}
			}
			for _, candidate := range []*Book{restarted, opBook} {
				if ack, err := candidate.AcknowledgeChoice("app-scope", q.RequestRef, *own.Record.Answer, nil); err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
					t.Fatal(ack, err)
				}
			}
			if raw, err := os.ReadFile(operatorPath); err != nil {
				t.Fatal(err)
			} else if ownRaw, err := os.ReadFile(b.path); err != nil || !bytes.Equal(withoutAudit(t, raw, !dismiss), withoutAudit(t, ownRaw, false)) {
				t.Fatal("ack hash/tombstone bytes diverged", err)
			}
			if gone, err := restarted.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, option, actor, dismiss, false, nil); err != nil || gone.Outcome != wire.ChoiceDecisionOutcomeGone || gone.Record != nil {
				t.Fatal(gone, err)
			}
		})
	}
}

// withoutAudit is the encoded Book state with its answer audit removed after
// checking that the audit holds exactly one entry when audited is true and
// none otherwise.
func withoutAudit(t *testing.T, raw []byte, audited bool) []byte {
	t.Helper()
	f, err := decodeApplication(raw)
	if err != nil || (len(f.Answers) == 1) != audited || len(f.Answers) > 1 {
		t.Fatalf("answer audit: %+v %v", f.Answers, err)
	}
	f.Answers, f.AnswersSinceMs = nil, 0
	out, err := json.Marshal(&f)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestOrdinaryOwnDecisionCannotAddressPermissionsOrForeignRef(t *testing.T) {
	b, clock := ordinaryBook(t)
	permission, _, err := b.AskApplication("app-scope", "application", "permission-ref", FirstUseKey, map[string]string{"program": choiceActorPath(t, "app.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	q := ordinaryRequest(*clock, "ordinary-ref")
	admitChoice(t, b, q)
	before, err := b.readApplication()
	if err != nil {
		t.Fatal(err)
	}
	actor := choiceActorPath(t, "caller.exe")
	for _, target := range []struct {
		scope string
		ref   wire.RequestRef
	}{
		{"different-program", q.RequestRef},
		{"app-scope", wire.RequestRef{Key: "permission-ref", ReplayUntilMs: q.RequestRef.ReplayUntilMs}},
		{"app-scope", wire.RequestRef{Key: permission.ID, ReplayUntilMs: q.RequestRef.ReplayUntilMs}},
		{"app-scope", wire.RequestRef{Key: q.RequestRef.Key, ReplayUntilMs: q.RequestRef.ReplayUntilMs + 1}},
	} {
		for _, dismiss := range []bool{false, true} {
			option := "keep"
			if dismiss {
				option = ""
			}
			if result, err := b.DecideOwnChoiceAuthorized(target.scope, target.ref, option, actor, dismiss, false, nil); err != nil || result.Outcome != wire.ChoiceDecisionOutcomeUnknown || result.Record != nil {
				t.Fatal("foreign/absent identity leaked", result, err)
			}
		}
	}
	after, err := b.readApplication()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("refused own decision changed permissions or choices", err)
	}
	if invalid, err := b.DecideOwnChoiceAuthorized("app-scope", wire.RequestRef{}, "keep", actor, false, true, nil); err != nil || invalid.Outcome != wire.ChoiceDecisionOutcomeInvalid {
		t.Fatal(invalid, err)
	}
	if invalid, err := b.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, "unoffered", actor, false, true, nil); err != nil || invalid.Outcome != wire.ChoiceDecisionOutcomeInvalid {
		t.Fatal(invalid, err)
	}
}

func TestOrdinaryOwnAndOperatorRaceFirstTerminalWinner(t *testing.T) {
	for range 8 {
		b, _ := ordinaryBook(t)
		q := ordinaryRequest(b.clock(), "race")
		pending := admitChoice(t, b, q)
		other, err := LoadApplicationBook(b.path)
		if err != nil {
			t.Fatal(err)
		}
		other.now = b.now
		callerActor, operatorActor := choiceActorPath(t, "caller.exe"), choiceActorPath(t, "operator.exe")
		start := make(chan struct{})
		results := make(chan wire.ChoiceDecision, 2)
		errors := make(chan error, 2)
		go func() {
			<-start
			r, err := b.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, "keep", callerActor, false, true, nil)
			results <- r
			errors <- err
		}()
		go func() {
			<-start
			r, err := other.DecideChoiceAuthorized(pending.ID, "replace", operatorActor, false, true, nil)
			results <- r
			errors <- err
		}()
		close(start)
		a, z := <-results, <-results
		for range 2 {
			if err := <-errors; err != nil {
				t.Fatal(err)
			}
		}
		if a.Outcome == wire.ChoiceDecisionOutcomeConflict {
			a, z = z, a
		}
		if a.Outcome != wire.ChoiceDecisionOutcomeAnswered || z.Outcome != wire.ChoiceDecisionOutcomeConflict || z.Record != nil {
			t.Fatal("multiple terminal winners", a, z)
		}
		answer := a.Record.Answer
		if answer.Option == "keep" && answer.AnsweredBy != callerActor || answer.Option == "replace" && answer.AnsweredBy != operatorActor {
			t.Fatal("wrong winner attribution", answer)
		}
		if reconciled, err := b.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, answer.Option, callerActor, false, false, nil); err != nil || !reflect.DeepEqual(reconciled, a) {
			t.Fatal(reconciled, err)
		}
	}
}

func TestOrdinaryOwnDecisionExpiryFloorRestartAndStorageFailure(t *testing.T) {
	for _, replayDeadline := range []bool{false, true} {
		b, clock := ordinaryBook(t)
		original := *clock
		q := ordinaryRequest(*clock, "expiry")
		admitChoice(t, b, q)
		deadline := q.PendingUntilMs
		if replayDeadline {
			deadline = q.RequestRef.ReplayUntilMs
		}
		*clock = time.UnixMilli(deadline)
		actor := choiceActorPath(t, "caller.exe")
		if result, err := b.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, "keep", actor, false, true, nil); err != nil || result.Outcome != wire.ChoiceDecisionOutcomeExpired || result.Record != nil {
			t.Fatal(result, err)
		}
		*clock = original
		restarted, err := LoadApplicationBook(b.path)
		if err != nil {
			t.Fatal(err)
		}
		restarted.now = b.now
		if result, err := restarted.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, "", actor, true, false, nil); err != nil || result.Outcome != wire.ChoiceDecisionOutcomeExpired {
			t.Fatal("rollback reopened expired request", result, err)
		}
		if replayDeadline {
			absent := q.RequestRef
			absent.Key = "never-admitted"
			if result, err := restarted.DecideOwnChoiceAuthorized("other-scope", absent, "keep", actor, false, true, nil); err != nil || result.Outcome != wire.ChoiceDecisionOutcomeExpired {
				t.Fatal("elapsed absent ref lost expiry", result, err)
			}
		}
	}
	b, clock := ordinaryBook(t)
	q := ordinaryRequest(*clock, "storage-refusal")
	admitChoice(t, b, q)
	originalPath := b.path
	before, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	*clock = time.UnixMilli(q.RequestRef.ReplayUntilMs)
	b.path = t.TempDir()
	if result, err := b.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, "keep", choiceActorPath(t, "caller.exe"), false, true, nil); err == nil || result.Outcome == wire.ChoiceDecisionOutcomeExpired {
		t.Fatal("storage failure reported successful expiry", result, err)
	}
	if after, err := os.ReadFile(originalPath); err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed expiry changed original evidence", err)
	}
}

func TestOrdinaryOwnDecisionRechecksAuthorizationAfterBookWait(t *testing.T) {
	for _, failure := range []error{context.Canceled, errors.New("binding revoked")} {
		b, clock := ordinaryBook(t)
		q := ordinaryRequest(*clock, "wait-recheck")
		admitChoice(t, b, q)
		before, err := os.ReadFile(b.path)
		if err != nil {
			t.Fatal(err)
		}
		b.mu.Lock()
		started, done := make(chan struct{}), make(chan error, 1)
		authority := error(nil)
		actor := choiceActorPath(t, "caller.exe")
		go func() {
			close(started)
			_, err := b.DecideOwnChoiceAuthorized("app-scope", q.RequestRef, "keep", actor, false, true, func() error { return authority })
			done <- err
		}()
		<-started
		authority = failure
		b.mu.Unlock()
		if err := <-done; !errors.Is(err, failure) {
			t.Fatal("write did not recheck after lock", err)
		}
		if after, err := os.ReadFile(b.path); err != nil || !bytes.Equal(before, after) {
			t.Fatal("failed binding changed evidence", err)
		}
	}
}
