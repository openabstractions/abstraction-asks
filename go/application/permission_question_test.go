package application

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	rights "github.com/openabstractions/abstraction-rights/go"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
	rightsclient "github.com/openabstractions/abstraction-rights/go/client"
)

// runtimeQuestion admits the runtime's own first-use question for program,
// as a refused call does (ASK-R2).
func runtimeQuestion(t *testing.T, b *asks.Book, account, program, key string) asks.Record {
	t.Helper()
	permission, err := asks.NewPermissionRequest(asks.PermissionTarget{Account: account, Program: program, Action: "abstraction.inference/complete", Resource: "host:fixture"})
	if err != nil {
		t.Fatal(err)
	}
	record, outcome, err := b.AskPermissionFrom("runtime-fixture-scope", "OpenAbstractions", key, permission, listen.Seen{Why: "fixture refusal"}, asks.Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatalf("runtime question %s %v", outcome, err)
	}
	return record
}

func TestPermissionObservationWaitsForCommittedCurrentPolicy(t *testing.T) {
	b, _ := book(t)
	var current atomic.Int32
	var calls atomic.Int32
	observe := func(ctx context.Context, peer *identity.Peer, record asks.Record) (wire.PermissionObservation, error) {
		calls.Add(1)
		subject, err := rightsclient.SubjectFromPeer(peer)
		if err != nil {
			return wire.PermissionObservation{}, err
		}
		if !questionAbout(record, subject.Account, subject) {
			t.Fatal("foreign callback")
		}
		switch current.Load() {
		case 1:
			return wire.PermissionObservation{Outcome: wire.PermissionOutcomePermitted, PolicyRevision: "committed-1"}, nil
		case 2:
			return wire.PermissionObservation{Outcome: wire.PermissionOutcomeDenied, PolicyRevision: "revoked-2"}, nil
		case 3:
			return wire.PermissionObservation{}, errors.New("quiet decision unavailable")
		case 4:
			return wire.PermissionObservation{Outcome: wire.PermissionOutcomePermitted}, nil
		}
		return wire.PermissionObservation{Outcome: wire.PermissionOutcomePending}, nil
	}
	_, app, operator, _, _ := liveTyped(t, b, prepareFixture, observe)
	selection := rwire.PermissionSelection{Permission: rwire.PermissionKey{Action: "abstraction.credentials/read", ResourceProfile: "abstraction.credentials/named@1"}, Selector: rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindOne, Target: []byte(`{"name":"fixture"}`)}}
	request, err := app.RequestContext(t.Context(), wire.PermissionIntent{RequestKey: "commit-observe", Selection: &selection})
	if err != nil || request.Answer == nil {
		t.Fatal(request, err)
	}
	id := request.Answer.ID
	if _, err := operator.AnswerPermissionIfPendingContext(t.Context(), id, "allow"); err != nil {
		t.Fatal(err)
	}
	before, err := app.ObserveQuestionContext(t.Context(), id, 0)
	if err != nil || before.Observation.Outcome != wire.PermissionOutcomePending {
		t.Fatal("recorded Allow became authority", before, err)
	}
	result := make(chan wire.PendingQuestion, 1)
	go func() {
		out, err := app.ObserveQuestionContext(t.Context(), id, 1000)
		if err != nil {
			t.Error(err)
		}
		result <- out
	}()
	deadline := time.Now().Add(time.Second)
	for b.Gate().Waiting() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("observer did not wait")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case out := <-result:
		t.Fatal("returned before commit", out)
	default:
	}
	current.Store(1)
	b.Gate().Advance()
	select {
	case out := <-result:
		if out.Observation.Outcome != wire.PermissionOutcomePermitted || out.Observation.PolicyRevision != "committed-1" {
			t.Fatal(out)
		}
	case <-time.After(time.Second):
		t.Fatal("commit failed to wake")
	}
	// An unread/lost observation reply repeats only the same quiet read.
	again, err := app.ObserveQuestionContext(t.Context(), id, 0)
	if err != nil || again.Observation.Outcome != wire.PermissionOutcomePermitted {
		t.Fatal(again, err)
	}
	current.Store(2)
	b.Gate().Advance()
	revoked, err := app.ObserveQuestionContext(t.Context(), id, 0)
	if err != nil || revoked.Observation.Outcome != wire.PermissionOutcomeDenied || revoked.Observation.PolicyRevision != "revoked-2" {
		t.Fatal(revoked, err)
	}
	current.Store(3)
	failed, err := app.ObserveQuestionContext(t.Context(), id, 0)
	if err != nil || failed.Observation.Outcome != wire.PermissionOutcomeUnavailable {
		t.Fatal(failed, err)
	}
	current.Store(4)
	unwitnessed, err := app.ObserveQuestionContext(t.Context(), id, 0)
	if err != nil || unwitnessed.Observation.Outcome != wire.PermissionOutcomePending {
		t.Fatal(unwitnessed, err)
	}
	stored, _, _ := b.ApplicationRecord(id)
	foreign := runtimeQuestion(t, b, stored.TypedPermission.Effect.Subject.Account, stored.TypedPermission.Effect.Subject.Program+".other.exe", "foreign-commit")
	count := calls.Load()
	hidden, err := app.ObserveQuestionContext(t.Context(), foreign.ID, 0)
	if err != nil || hidden.Observation.Outcome != wire.PermissionOutcomeUnknown || calls.Load() != count {
		t.Fatal(hidden, err, calls.Load(), count)
	}
}

// A request for an exact first-use action joins the runtime's own question
// and ObserveQuestion waits for the person's answer to it; another program's
// question and an unknown id read unknown (ASK-A14).
func TestExactFirstUseRequestJoinsAndObservesTheRuntimeQuestion(t *testing.T) {
	b, _ := book(t)
	var other asks.Record
	joined := func(ctx context.Context, peer *identity.Peer, q wire.PermissionIntent) (PreparedPermission, error) {
		subject, err := rightsclient.SubjectFromPeer(peer)
		if err != nil {
			return PreparedPermission{}, err
		}
		if q.Selection == nil || q.Selection.Permission.ResourceProfile != rights.LegacyExactResourceProfile {
			return PreparedPermission{Outcome: wire.PermissionOutcomeInvalid}, nil
		}
		record := runtimeQuestion(t, b, subject.Account, subject.Program, "first-use:fixture")
		other = runtimeQuestion(t, b, subject.Account, subject.Program+".other.exe", "first-use:other")
		return PreparedPermission{Exact: true, Outcome: wire.PermissionOutcomePending, Joined: &record}, nil
	}
	_, app, _, _, _ := liveTyped(t, b, joined)
	ctx := context.Background()
	selection := rwire.PermissionSelection{Permission: rwire.PermissionKey{Action: "abstraction.inference/complete", ResourceProfile: rights.LegacyExactResourceProfile}, Selector: rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindOne, Target: []byte("host:fixture")}}
	admitted, err := app.RequestContext(ctx, wire.PermissionIntent{RequestKey: "ask-before-use", Selection: &selection})
	if err != nil || admitted.Outcome != wire.PermissionOutcomePending || admitted.Answer == nil {
		t.Fatalf("request %+v %v", admitted, err)
	}
	id := admitted.Answer.ID
	before := time.Now()
	pending, err := app.ObserveQuestionContext(ctx, id, 0)
	if err != nil || pending.Observation.Outcome != wire.PermissionOutcomePending || pending.PendingUntilMs <= before.UnixMilli() {
		t.Fatalf("pending %+v %v", pending, err)
	}
	if got := b.LiveQuestionWaiters(id); got != 0 {
		t.Fatalf("nonblocking Observe registered %d waiters", got)
	}
	for name, unseen := range map[string]string{"another program's question": other.ID, "unknown id": "no-such-question"} {
		got, err := app.ObserveQuestionContext(ctx, unseen, 0)
		if err != nil || got.Observation.Outcome != wire.PermissionOutcomeUnknown || got.PendingUntilMs != 0 {
			t.Fatalf("%s: %+v %v", name, got, err)
		}
	}
	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for b.LiveQuestionWaiters(id) == 0 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if got := b.LiveQuestionWaiters(id); got != 1 {
			t.Errorf("blocking Observe registered %d waiters", got)
		}
		if _, _, err := b.AnswerApplication(id, "allow"); err != nil {
			t.Error(err)
		}
	}()
	answered, err := app.ObserveQuestionContext(ctx, id, 10000)
	if err != nil || answered.Observation.Outcome != wire.PermissionOutcomeAnswered || answered.Observation.Answer.Option != "allow" || answered.PendingUntilMs != 0 {
		t.Fatalf("answered %+v %v", answered, err)
	}
	if got := b.LiveQuestionWaiters(id); got != 0 {
		t.Fatalf("settled Observe left %d waiters", got)
	}
	if elapsed := time.Since(before); elapsed > 5*time.Second {
		t.Fatalf("the answer arrived after %v", elapsed)
	}
	if _, err := app.ObserveQuestionContext(ctx, id, 30001); err == nil {
		t.Fatal("a wait beyond 30 s was sent")
	}
}
