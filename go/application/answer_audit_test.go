package application

import (
	"context"
	"testing"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
)

// ListAnswers serves the answer audit to the authorized operator only, in
// bounded pages bound to the Book revision (ASK-O15): an answer the person
// gave through operator@1 is listed after its question is dismissed, and a
// refused operator reads nothing.
func TestOperatorListsTheAnswerAuditInBoundedPages(t *testing.T) {
	b, _ := book(t)
	deny := false
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error {
		if deny {
			return ErrOperatorForbidden
		}
		return nil
	})
	ctx := context.Background()
	for _, key := range []string{"first", "second"} {
		q := question()
		q.RequestKey = key
		q.Slots = map[string]string{"host": key + ".example.com"}
		asked, err := app.AskContext(ctx, q)
		if err != nil || asked.Outcome != wire.ObservationOutcomePending {
			t.Fatal(asked, err)
		}
		if d, err := op.AnswerQuestionIfPendingContext(ctx, asked.Answer.ID, "refuse"); err != nil || d.Outcome != wire.OperatorDecisionOutcomeAnswered {
			t.Fatal(d, err)
		}
		if key == "first" {
			if r, err := op.RetireQuestionContext(ctx, asked.Answer.ID); err != nil || r.Outcome != wire.OperatorRetirementOutcomeRetired {
				t.Fatal(r, err)
			}
		}
	}
	first, err := op.ListAnswersContext(ctx, "", 1)
	if err != nil || first.Outcome != wire.OperatorPageOutcomePage || len(first.Answers) != 1 || first.Complete || first.Next == "" {
		t.Fatalf("%+v %v", first, err)
	}
	if a := first.Answers[0]; a.Kind != wire.AnswerKindQuestion || a.Key != "download.reach" || a.About != "second.example.com" || a.Option != "refuse" || a.AnsweredBy != wire.AnswererOperator || a.Answerer != "" {
		t.Fatalf("newest first: %+v", a)
	}
	rest, err := op.ListAnswersContext(ctx, first.Next, 64)
	if err != nil || rest.Outcome != wire.OperatorPageOutcomePage || len(rest.Answers) != 1 || !rest.Complete || rest.Answers[0].About != "first.example.com" {
		t.Fatalf("a dismissed question's answer stays listed: %+v %v", rest, err)
	}
	// A new answer changes the revision: the old cursor is a gap.
	q := question()
	q.RequestKey, q.Slots = "third", map[string]string{"host": "third.example.com"}
	asked, err := app.AskContext(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.AnswerQuestionIfPendingContext(ctx, asked.Answer.ID, "once"); err != nil {
		t.Fatal(err)
	}
	if gap, err := op.ListAnswersContext(ctx, first.Next, 64); err != nil || gap.Outcome != wire.OperatorPageOutcomeGap || len(gap.Answers) != 0 {
		t.Fatalf("%+v %v", gap, err)
	}
	if invalid, err := op.ListAnswersContext(ctx, "", 65); err == nil {
		t.Fatal("the client must refuse a limit past 64", invalid)
	}
	deny = true
	if refused, err := op.ListAnswersContext(ctx, "", 64); err != nil || refused.Outcome != wire.OperatorPageOutcomeForbidden || len(refused.Answers) != 0 {
		t.Fatalf("a refused operator must read nothing: %+v %v", refused, err)
	}
}
