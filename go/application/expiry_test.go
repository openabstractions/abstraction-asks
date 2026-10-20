package application

import (
	"context"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
)

// TestExpiredQuestionIsUnknownOverIPC: after the pending lifetime (ASK-A7)
// the operator no longer lists or answers the question, the application
// observes unknown, and its key admits a new question.
func TestExpiredQuestionIsUnknownOverIPC(t *testing.T) {
	b, _ := book(t)
	if err := b.SetPendingLifetime(300 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	ctx := context.Background()
	asked, err := app.AskContext(ctx, question())
	if err != nil || asked.Outcome != wire.ObservationOutcomePending {
		t.Fatal(asked, err)
	}
	time.Sleep(400 * time.Millisecond)

	page, err := op.ListQuestionsContext(ctx, "", 64)
	if err != nil || page.Outcome != wire.OperatorPageOutcomePage || len(page.Records) != 0 {
		t.Fatalf("an expired question must not be listed: %+v %v", page, err)
	}
	if read, err := op.ReadAttributedQuestionContext(ctx, asked.Answer.ID); err != nil || read.Question.Outcome != wire.OperatorQuestionOutcomeUnknown {
		t.Fatalf("an expired question must read unknown: %+v %v", read, err)
	}
	if answer, err := op.AnswerQuestionIfPendingContext(ctx, asked.Answer.ID, "allow"); err != nil || answer.Outcome != wire.OperatorDecisionOutcomeUnknown {
		t.Fatalf("an expired question must answer unknown: %+v %v", answer, err)
	}
	if observed, err := app.ObserveContext(ctx, question().RequestKey, 0); err != nil || observed.Outcome != wire.ObservationOutcomeUnknown {
		t.Fatalf("the application must observe unknown: %+v %v", observed, err)
	}
	again, err := app.AskContext(ctx, question())
	if err != nil || again.Outcome != wire.ObservationOutcomePending || again.Answer.ID == asked.Answer.ID {
		t.Fatalf("the same key must admit a new question: %+v %v", again, err)
	}
}
