package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestResponderPartialAnswerAndReplayNeverReapply(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Darwin Unix stream transport cannot bind Program evidence; this fixture does not use XPC")
	}
	b, _ := book(t)
	first, outcome, err := b.AskApplication("runtime-only", "runtime", "first", asks.FirstUseKey,
		map[string]string{"program": filepath.Join(t.TempDir(), "target.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatalf("first use %+v %s %v", first, outcome, err)
	}
	other, outcome, err := b.AskApplication("ordinary-app", "app", "second", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatalf("ordinary question %+v %s %v", other, outcome, err)
	}
	endpoint := listen.Endpoint(fmt.Sprintf("asks-responder-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	var writes atomic.Int64
	if err := h.EnableResponder(func(_ context.Context, _ *identity.Peer, record asks.Record, scope, option string) (ResponderEffect, error) {
		if scope != "runtime-only" || record.Key != asks.FirstUseKey || option != "allow" {
			return nil, ErrResponderForbidden
		}
		return func(_ context.Context, _ func() error, firstAnswer bool) (wire.ResponderEffectOutcome, string) {
			if !firstAnswer {
				return wire.ResponderEffectOutcomeConflict, "not_granted"
			}
			writes.Add(1)
			return wire.ResponderEffectOutcomeForbidden, "not_granted" // expiry after Book commit, before policy write
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Serve(context.Background()) }()
	t.Cleanup(func() {
		h.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("responder host did not drain")
		}
	})
	responder := client.NewResponder(endpoint)
	page, err := responder.ListAnswerableContext(context.Background(), "", 16)
	if err != nil || len(page.Records) != 1 || page.Records[0].ID != first.ID || len(page.Records[0].Options) != 1 {
		t.Fatalf("filtered page %+v %v", page, err)
	}
	if decision, err := responder.AnswerContext(context.Background(), other.ID, "allow"); err != nil || decision.Outcome != wire.OperatorDecisionOutcomeForbidden {
		t.Fatalf("forged application question %+v %v", decision, err)
	}
	answer, err := responder.AnswerContext(context.Background(), first.ID, "allow")
	if err != nil || answer.Outcome != wire.OperatorDecisionOutcomeAnswered || answer.Effect != wire.ResponderEffectOutcomeForbidden || answer.TargetOutcome != "not_granted" {
		t.Fatalf("partial answer %+v %v", answer, err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// The answer records its actor, the program the call bound (ASK-O7, ASK-R5).
	if stored, _, err := b.ApplicationRecord(first.ID); err != nil || stored.AnsweredBy != asks.AnsweredByResponder || !identity.SameSubjectProgram(stored.Answerer, self) {
		t.Fatalf("a responder's answer must record who answered: %+v %v", stored, err)
	}
	replay, err := responder.AnswerContext(context.Background(), first.ID, "allow")
	if err != nil || replay.Outcome != wire.OperatorDecisionOutcomeAnswered || replay.Effect != wire.ResponderEffectOutcomeConflict || writes.Load() != 1 {
		t.Fatalf("unsafe replay %+v %v writes=%d", replay, err, writes.Load())
	}
	if conflict, err := responder.AnswerContext(context.Background(), first.ID, "refuse"); err != nil || conflict.Outcome != wire.OperatorDecisionOutcomeForbidden {
		t.Fatalf("different ungranted option %+v %v", conflict, err)
	}
	if _, _, err := b.AnswerApplicationScopedAuthorized(first.ID, "allow", func(record asks.Record, scope string) error {
		if scope != "runtime-only" || record.Pending() {
			return errors.New("lost answer")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
