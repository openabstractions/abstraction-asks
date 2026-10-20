package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	cas "github.com/openabstractions/abstraction-cas/go"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

func TestResponderPreparationOnceBeforeBookAcrossAllMethods(t *testing.T) {
	b, path := book(t)
	r, _, err := b.AskApplication("runtime-only", "runtime", "prepare", asks.FirstUseKey, map[string]string{"program": filepath.Join(t.TempDir(), "target.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		t.Skip("native Program fixture; no XPC proof")
	}
	endpoint := listen.Endpoint(fmt.Sprintf("prepare-responder-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	type operationKey struct{}
	var calls atomic.Int64
	var fail atomic.Bool
	if err := h.EnableResponderPreparation(func(ctx context.Context) (context.Context, error) {
		calls.Add(1)
		if fail.Load() {
			return ctx, errors.New("metadata unavailable")
		}
		// This takes the real Book file's CAS lock. The hook must run before
		// any Book read/edit, including Answer and replay.
		bounded, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if _, err := cas.ReadLimitContext(bounded, path, 16<<20); err != nil {
			return ctx, err
		}
		return context.WithValue(ctx, operationKey{}, calls.Load()), nil
	}); err != nil {
		t.Fatal(err)
	}
	check := func(ctx context.Context) error {
		if ctx.Value(operationKey{}) != calls.Load() {
			return errors.New("operation context lost")
		}
		return ctx.Err()
	}
	if err := h.EnableResponder(func(ctx context.Context, _ *identity.Peer, _ asks.Record, _, _ string) (ResponderEffect, error) {
		return nil, check(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.EnableChoiceResponder(func(ctx context.Context, _ *identity.Peer, _ asks.ChoiceCandidate) (ChoiceResponderEffect, error) {
		return nil, check(ctx)
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.EnableAnsweringRequest(func(ctx context.Context, _ *identity.Peer) rwire.Decision {
		if err := check(ctx); err != nil {
			return rwire.Decision{Outcome: rwire.DecisionOutcomeUnavailable}
		}
		return rwire.Decision{Outcome: rwire.DecisionOutcomeNotGranted}
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Serve(t.Context()) }()
	t.Cleanup(func() {
		h.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	c := client.NewResponder(endpoint)
	parent := t.Context()
	if got, err := c.ListAnswerableContext(parent, "", 16); err != nil || got.Outcome != wire.OperatorPageOutcomePage {
		t.Fatal(got, err)
	}
	for range 2 {
		if got, err := c.AnswerContext(parent, r.ID, "allow"); err != nil || got.Outcome != wire.OperatorDecisionOutcomeAnswered {
			t.Fatal(got, err)
		}
	}
	shared, err := client.NewChoice(endpoint).ShareContext(parent, outputChoice("prepared-choice"), []string{"keep"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := c.ListChoicesContext(parent, "", 16); err != nil || len(got.Choices) != 1 {
		t.Fatal(got, err)
	}
	for range 2 {
		if got, err := c.AnswerChoiceContext(parent, shared.ID, "keep"); err != nil || got.Outcome != wire.ChoiceDecisionOutcomeAnswered {
			t.Fatal(got, err)
		}
	}
	if got, err := c.RequestAnsweringContext(parent); err != nil || got.Outcome != rwire.DecisionOutcomeNotGranted {
		t.Fatal(got, err)
	}
	if calls.Load() != 7 {
		t.Fatal("outer preparation count", calls.Load())
	}
	fail.Store(true)
	if got, err := c.ListAnswerableContext(parent, "", 16); err != nil || got.Outcome != wire.OperatorPageOutcomeUnavailable {
		t.Fatal(got, err)
	}
	if got, err := c.AnswerContext(parent, r.ID, "allow"); err != nil || got.Outcome != wire.OperatorDecisionOutcomeUnavailable {
		t.Fatal(got, err)
	}
	if got, err := c.ListChoicesContext(parent, "", 16); err != nil || got.Outcome != wire.OperatorPageOutcomeUnavailable {
		t.Fatal(got, err)
	}
	if got, err := c.AnswerChoiceContext(parent, shared.ID, "keep"); err != nil || got.Outcome != wire.ChoiceDecisionOutcomeUnavailable {
		t.Fatal(got, err)
	}
	if got, err := c.RequestAnsweringContext(parent); err != nil || got.Outcome != rwire.DecisionOutcomeUnavailable {
		t.Fatal(got, err)
	}
}

func TestResponderTypedProjectionListAnswerAndArchivalText(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("native Program IPC fixture; no XPC proof")
	}
	b, _ := book(t)
	owner, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	p := asks.TypedPermission{Origin: "service", Effect: rwire.PermissionEffect{Subject: rwire.Subject{Account: owner.Uid, Program: filepath.Join(t.TempDir(), "beneficiary.exe")}}}
	for _, action := range []string{"propose", "read", "reveal"} {
		p.Effect.Selections = append(p.Effect.Selections, rwire.PermissionSelection{Permission: rwire.PermissionKey{Action: "example/" + action, ResourceProfile: "example/exact@1"}, Selector: rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindOne, Target: []byte(`"app:editor"`)}})
		p.Display.Selections = append(p.Display.Selections, wire.PermissionDisplaySelection{Operation: action, Target: "app:editor", Disclosure: "Exact " + action + " capability"})
	}
	p.Digest = asks.PermissionEffectDigest(p.Effect)
	r, outcome, err := b.AskTypedPermissionOccurrenceAuthorizedFrom("trusted-finite-owner", "runtime", "native-occurrence", p, listen.Seen{}, asks.Publisher{}, func() error { return nil })
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	endpoint := listen.Endpoint(fmt.Sprintf("typed-responder-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.EnableResponder(func(_ context.Context, _ *identity.Peer, record asks.Record, scope, option string) (ResponderEffect, error) {
		if scope != "trusted-finite-owner" || record.ID != r.ID || record.TypedPermission == nil || option != "allow" {
			return nil, ErrResponderForbidden
		}
		return func(_ context.Context, _ func() error, first bool) (wire.ResponderEffectOutcome, string) {
			if !first {
				return wire.ResponderEffectOutcomeConflict, "permitted"
			}
			return wire.ResponderEffectOutcomeApplied, "permitted"
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Serve(t.Context()) }()
	t.Cleanup(func() {
		h.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("host not joined")
		}
	})
	client := client.NewResponder(endpoint)
	page, err := client.ListAnswerableContext(t.Context(), "", 16)
	if err != nil || len(page.Records) != 1 {
		t.Fatal(page, err)
	}
	text := page.Records[0].Text
	for _, word := range []string{p.Effect.Subject.Program, owner.Uid, "app:editor", "Exact propose capability", "Exact read capability", "Exact reveal capability"} {
		if !strings.Contains(text, word) {
			t.Fatal("missing disclosure", word, text)
		}
	}
	answer, err := client.AnswerContext(t.Context(), r.ID, "allow")
	if err != nil || answer.Record == nil || answer.Record.Text != text || answer.Effect != wire.ResponderEffectOutcomeApplied {
		t.Fatal(answer, err)
	}
	saved, _, err := b.ApplicationRecord(r.ID)
	if err != nil || saved.Text != "OA permission request" || saved.AnsweredBy != asks.AnsweredByResponder || saved.Answerer == "" {
		t.Fatal(saved, err)
	}
}

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
	if err := h.EnableNotifications(func(context.Context, *identity.Peer) error { return nil }, func(context.Context, string) error { return nil }, nil); err != nil {
		t.Fatal(err)
	}
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
	// The service refuses an unlisted option without relying on any adapter
	// pre-list. The Book and target policy remain unchanged before dispatch.
	if decision, err := responder.AnswerContext(t.Context(), first.ID, "unlisted"); err != nil || decision.Outcome != wire.OperatorDecisionOutcomeForbidden {
		t.Fatalf("unlisted option %+v %v", decision, err)
	}
	if stored, found, err := b.ApplicationRecord(first.ID); err != nil || !found || !stored.Pending() || writes.Load() != 0 {
		t.Fatalf("unlisted option changed the question: %+v %v writes=%d", stored, err, writes.Load())
	}
	answer, err := responder.AnswerContext(context.Background(), first.ID, "allow")
	if err != nil || answer.Outcome != wire.OperatorDecisionOutcomeAnswered || answer.Effect != wire.ResponderEffectOutcomeForbidden || answer.TargetOutcome != "not_granted" {
		t.Fatalf("partial answer %+v %v", answer, err)
	}
	// A caller that loses the reply has only the committed native record to
	// inspect. No background retry or policy write is scheduled by the service.
	if answer.Record == nil || answer.Record.ID != first.ID || answer.Record.Option != "allow" || writes.Load() != 1 {
		t.Fatalf("partial result lost recorded answer %+v writes=%d", answer, writes.Load())
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
	notifications, _, err := b.NotificationHistory()
	if err != nil || len(notifications) != 1 || notifications[0].State != wire.NotificationStatePosted || notifications[0].Request.Title != "Answered for you" {
		t.Fatalf("recorded answer needs one service notification after reply replay: %+v %v", notifications, err)
	}
	human, _, err := b.AskApplication("runtime-only", "runtime", "human", asks.FirstUseKey,
		map[string]string{"program": filepath.Join(t.TempDir(), "human-target.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if _, outcome, err := b.AnswerApplication(human.ID, "allow"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	if replay, err := responder.AnswerContext(t.Context(), human.ID, "allow"); err != nil || replay.Outcome != wire.OperatorDecisionOutcomeAnswered {
		t.Fatal(replay, err)
	}
	if notices, _, err := b.NotificationHistory(); err != nil || len(notices) != 1 {
		t.Fatal("person's answer was attributed to the assistant", notices, err)
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
