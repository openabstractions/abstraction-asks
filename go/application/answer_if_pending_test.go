package application

import (
	"context"
	"testing"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
)

// TestOperatorAnswerIfPendingNeverReplays is ASK-O8: the conditional answer
// records a choice only while the question is pending. An answer that landed
// first, with any option, is conflict and is never replayed as answered, so a
// presenter cannot mistake another path's answer for its own.
func TestOperatorAnswerIfPendingNeverReplays(t *testing.T) {
	b, _ := book(t)
	var deny bool
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error {
		if deny {
			return ErrOperatorForbidden
		}
		return nil
	})
	ctx := context.Background()
	q := question()
	asked, err := app.AskContext(ctx, q)
	if err != nil || asked.Outcome != wire.ObservationOutcomePending {
		t.Fatal(asked, err)
	}
	q.RequestKey, q.Slots["host"] = "raced", "raced.example"
	raced, err := app.AskContext(ctx, q)
	if err != nil || raced.Outcome != wire.ObservationOutcomePending {
		t.Fatal(raced, err)
	}

	first, err := op.AnswerQuestionIfPendingContext(ctx, asked.Answer.ID, "refuse")
	if err != nil || first.Outcome != wire.OperatorDecisionOutcomeAnswered || first.Record == nil || first.Record.Option != "refuse" {
		t.Fatalf("a pending question must be answered: %+v %v", first, err)
	}
	attributedRead, err := op.ReadAttributedQuestionContext(ctx, asked.Answer.ID)
	read := attributedRead.Question
	if err != nil || read.AnsweredBy == nil || *read.AnsweredBy != wire.AnswererOperator {
		t.Fatalf("an operator's answer must read answered_by operator: %+v %v", read, err)
	}
	if pendingRead, err := op.ReadAttributedQuestionContext(ctx, raced.Answer.ID); err != nil || pendingRead.Question.AnsweredBy != nil {
		t.Fatalf("a pending question has no answered_by: %+v %v", pendingRead, err)
	}
	again, err := op.AnswerQuestionIfPendingContext(ctx, asked.Answer.ID, "refuse")
	if err != nil || again.Outcome != wire.OperatorDecisionOutcomeConflict || again.Record != nil {
		t.Fatalf("a same-option repeat must be conflict, never a replay: %+v %v", again, err)
	}

	// Another path answers between the presenter's re-read and its answer.
	if _, outcome, err := b.AnswerApplication(raced.Answer.ID, "allow"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	late, err := op.AnswerQuestionIfPendingContext(ctx, raced.Answer.ID, "allow")
	if err != nil || late.Outcome != wire.OperatorDecisionOutcomeConflict || late.Record != nil {
		t.Fatalf("an answer that landed elsewhere must be conflict: %+v %v", late, err)
	}
	if plain, err := op.AnswerQuestionContext(ctx, raced.Answer.ID, "allow"); err != nil || plain.Outcome != wire.OperatorDecisionOutcomeAnswered {
		t.Fatalf("AnswerQuestion keeps its same-option replay: %+v %v", plain, err)
	}

	if missing, err := op.AnswerQuestionIfPendingContext(ctx, "0123456789abcdef0123456789abcdef", "allow"); err != nil || missing.Outcome != wire.OperatorDecisionOutcomeUnknown {
		t.Fatalf("an absent question must be unknown: %+v %v", missing, err)
	}
	q.RequestKey, q.Slots["host"] = "options", "options.example"
	other, err := app.AskContext(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if bad, err := op.AnswerQuestionIfPendingContext(ctx, other.Answer.ID, "maybe"); err != nil || bad.Outcome != wire.OperatorDecisionOutcomeInvalid {
		t.Fatalf("an option the catalog does not list must be invalid: %+v %v", bad, err)
	}
	deny = true
	if refused, err := op.AnswerQuestionIfPendingContext(ctx, other.Answer.ID, "allow"); err != nil || refused.Outcome != wire.OperatorDecisionOutcomeForbidden {
		t.Fatalf("an unauthorized answer must be forbidden: %+v %v", refused, err)
	}
	if record, _, _ := b.ApplicationRecord(other.Answer.ID); !record.Pending() {
		t.Fatalf("refused calls must record nothing: %+v", record)
	}
}
