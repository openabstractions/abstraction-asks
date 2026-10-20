package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

// sharedHost serves one Book with a permissive operator and the given
// ordinary-choice answerer check (ASK-R9). The same test binary is requester,
// operator and responder; the runtime's party rule is not under test here.
func sharedHost(t *testing.T, b *asks.Book, authorize AuthorizeChoiceResponder) string {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("Program process/path evidence is not implemented on macOS")
	}
	endpoint := listen.Endpoint(fmt.Sprintf("asks-shared-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.EnableOperator(func(context.Context, *identity.Peer) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if authorize != nil {
		if err := h.EnableChoiceResponder(authorize); err != nil {
			t.Fatal(err)
		}
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
			t.Error("shared choice host did not drain")
		}
	})
	return endpoint
}

func outputChoice(key string) wire.ChoiceRequest {
	now := time.Now()
	return wire.ChoiceRequest{RequestRef: wire.RequestRef{Key: key, ReplayUntilMs: now.Add(time.Hour).UnixMilli()}, Title: "The output already exists",
		Options:        []wire.ChoiceOption{{ID: "replace", Label: "Replace"}, {ID: "keep", Label: "Keep"}, {ID: "cancel", Label: "Cancel"}},
		PendingUntilMs: now.Add(time.Minute).UnixMilli()}
}

// The gap of research/release-0.4.0/planned/ordinary-agent-delegation.md §3:
// an enrolled answerer could neither see nor answer an ordinary choice. A
// shared choice now reaches it with only its shared options, and the answer
// is recorded with its route and an audit entry acknowledgement keeps.
func TestEnrolledAnswererAnswersSharedChoice(t *testing.T) {
	b, path := book(t)
	var renewed atomic.Int64
	var refuse atomic.Bool
	endpoint := sharedHost(t, b, func(_ context.Context, _ *identity.Peer, c asks.ChoiceCandidate) (ChoiceResponderEffect, error) {
		if refuse.Load() {
			return nil, ErrResponderForbidden
		}
		if c.AskerProgram == "" || len(c.Shared) == 0 {
			return nil, errors.New("candidate lacks requester program or shared options")
		}
		return func(context.Context) { renewed.Add(1) }, nil
	})
	ctx := context.Background()
	app, responder, op := client.NewChoice(endpoint), client.NewResponder(endpoint), client.NewOperator(endpoint)
	shared, unshared := outputChoice("shared"), outputChoice("unshared")
	pending, err := app.ShareContext(ctx, shared, []string{"keep", "cancel"})
	if err != nil || pending.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(pending, err)
	}
	other, err := app.AskContext(ctx, unshared)
	if err != nil || other.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(other, err)
	}
	// The same reference cannot change between shared and unshared.
	if conflict, err := app.AskContext(ctx, shared); err != nil || conflict.Outcome != wire.ChoiceOutcomeConflict {
		t.Fatal("sharing is part of the immutable request", conflict, err)
	}
	if conflict, err := app.ShareContext(ctx, shared, []string{"keep"}); err != nil || conflict.Outcome != wire.ChoiceOutcomeConflict {
		t.Fatal("shared options are part of the immutable request", conflict, err)
	}
	page, err := responder.ListChoicesContext(ctx, "", 16)
	if err != nil || page.Outcome != wire.OperatorPageOutcomePage || len(page.Choices) != 1 || !page.Complete {
		t.Fatalf("answerable page %+v %v", page, err)
	}
	listed := page.Choices[0]
	if listed.ID != pending.ID || listed.Title != shared.Title || len(listed.Options) != 2 || listed.Options[0].ID != "keep" || listed.Options[1].ID != "cancel" {
		t.Fatalf("listed choice %+v", listed)
	}
	if denied, err := responder.AnswerChoiceContext(ctx, other.ID, "keep"); err != nil || denied.Outcome != wire.ChoiceDecisionOutcomeForbidden || denied.Record != nil {
		t.Fatal("an unshared choice was answerable", denied, err)
	}
	if denied, err := responder.AnswerChoiceContext(ctx, pending.ID, "replace"); err != nil || denied.Outcome != wire.ChoiceDecisionOutcomeForbidden {
		t.Fatal("a reserved option was answerable", denied, err)
	}
	answered, err := responder.AnswerChoiceContext(ctx, pending.ID, "keep")
	exe, exeErr := os.Executable()
	if err != nil || exeErr != nil || answered.Outcome != wire.ChoiceDecisionOutcomeAnswered || answered.Record.Answer.Option != "keep" ||
		answered.Record.Answer.AnsweredBy != identity.CanonicalProgramPath(filepath.Clean(exe)) || renewed.Load() != 1 {
		t.Fatal(answered, err, exeErr, renewed.Load())
	}
	if replay, err := responder.AnswerChoiceContext(ctx, pending.ID, "keep"); err != nil || replay.Outcome != wire.ChoiceDecisionOutcomeAnswered || replay.Record.Answer.AnsweredMs != answered.Record.Answer.AnsweredMs || renewed.Load() != 1 {
		t.Fatal("a lost reply must replay without renewing again", replay, err)
	}
	if conflict, err := responder.AnswerChoiceContext(ctx, pending.ID, "cancel"); err != nil || conflict.Outcome != wire.ChoiceDecisionOutcomeConflict {
		t.Fatal(conflict, err)
	}
	if mine, err := app.AnswerContext(ctx, shared.RequestRef, "replace"); err != nil || mine.Outcome != wire.ChoiceDecisionOutcomeConflict {
		t.Fatal("the first recorded answer wins", mine, err)
	}
	observed, err := app.ObserveContext(ctx, shared.RequestRef, 0)
	if err != nil || observed.Outcome != wire.ChoiceOutcomeAnswered || *observed.Answer != *answered.Record.Answer {
		t.Fatal(observed, err)
	}
	details, err := op.ReadChoiceDetailsContext(ctx, pending.ID)
	if err != nil || details.Choice.Outcome != wire.OperatorQuestionOutcomeFound || details.AnsweredVia == nil || *details.AnsweredVia != wire.ChoiceRouteResponder ||
		!identity.SameSubjectProgram(details.AskerProgram, exe) || !slices.Equal(details.AssistantOptions, []string{"keep", "cancel"}) {
		t.Fatalf("details %+v %v", details, err)
	}
	if again, err := responder.ListChoicesContext(ctx, "", 16); err != nil || len(again.Choices) != 0 {
		t.Fatal("an answered choice stays listed", again, err)
	}
	if ack, err := app.AcknowledgeContext(ctx, shared.RequestRef, *observed.Answer); err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
		t.Fatal(ack, err)
	}
	restarted, err := asks.LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	opAfter := client.NewOperator(sharedHost(t, restarted, nil))
	audit, err := opAfter.ListAnswersContext(ctx, "", 16)
	if err != nil || audit.Outcome != wire.OperatorPageOutcomePage || len(audit.Answers) != 1 || audit.CompleteSinceMs != 0 {
		t.Fatalf("audit %+v %v", audit, err)
	}
	entry := audit.Answers[0]
	if entry.ID != pending.ID || entry.Option != "keep" || entry.Label != "Keep" || entry.Title != shared.Title ||
		!identity.SameSubjectProgram(entry.Answerer, exe) || !identity.SameSubjectProgram(entry.AskerProgram, exe) || entry.AnsweredMs != answered.Record.Answer.AnsweredMs {
		t.Fatalf("audit entry %+v", entry)
	}
	// A revoked answerer sees nothing and cannot answer.
	refuse.Store(true)
	if _, err := app.ShareContext(ctx, outputChoice("after-stop"), []string{"keep"}); err != nil {
		t.Fatal(err)
	}
	if none, err := responder.ListChoicesContext(ctx, "", 16); err != nil || none.Outcome != wire.OperatorPageOutcomePage || len(none.Choices) != 0 {
		t.Fatal("a refused answerer saw a choice", none, err)
	}
}

func TestSharedChoiceRefusalsWithoutAnswerer(t *testing.T) {
	b, _ := book(t)
	endpoint := sharedHost(t, b, nil)
	ctx := context.Background()
	app, responder := client.NewChoice(endpoint), client.NewResponder(endpoint)
	pending, err := app.ShareContext(ctx, outputChoice("no-answerer"), []string{"keep"})
	if err != nil || pending.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(pending, err)
	}
	if page, err := responder.ListChoicesContext(ctx, "", 16); err != nil || page.Outcome != wire.OperatorPageOutcomeForbidden || len(page.Choices) != 0 {
		t.Fatal("no configured answerer check must refuse", page, err)
	}
	if denied, err := responder.AnswerChoiceContext(ctx, pending.ID, "keep"); err != nil || denied.Outcome != wire.ChoiceDecisionOutcomeForbidden {
		t.Fatal(denied, err)
	}
	for _, bad := range [][]string{nil, {}, {"absent"}, {"keep", "keep"}, {"keep", "cancel", "replace", "keep"}} {
		if _, err := app.ShareContext(ctx, outputChoice("bad"), bad); err == nil {
			t.Fatalf("shared options %q accepted", bad)
		}
	}
}
