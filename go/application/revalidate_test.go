package application

import (
	"context"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
)

// TestOperatorRevalidationOfAShownQuestion is the service side of ASK-O6. A
// presenter lists a pending question and shows it; meanwhile another path
// answers or dismisses it. The re-read reports the change, a late answer
// never replaces the recorded one, and the application keeps observing the
// answer that landed first.
func TestOperatorRevalidationOfAShownQuestion(t *testing.T) {
	b, _ := book(t)
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	ctx := context.Background()
	q := question()
	asked, err := app.AskContext(ctx, q)
	if err != nil || asked.Outcome != wire.ObservationOutcomePending {
		t.Fatal(asked, err)
	}
	q.RequestKey, q.Slots["host"] = "dismissed", "dismissed.example"
	toDismiss, err := app.AskContext(ctx, q)
	if err != nil || toDismiss.Outcome != wire.ObservationOutcomePending {
		t.Fatal(toDismiss, err)
	}
	shown, err := op.ListQuestionsContext(ctx, "", 64)
	if err != nil || shown.Outcome != wire.OperatorPageOutcomePage || len(shown.Records) != 2 {
		t.Fatal("shown page", shown, err)
	}

	// Another answering path records its choice while the prompt is open.
	elsewhere, outcome, err := b.AnswerApplication(asked.Answer.ID, "refuse")
	if err != nil || outcome != "answered" {
		t.Fatal(elsewhere, outcome, err)
	}
	if _, err := op.RetireQuestionContext(ctx, toDismiss.Answer.ID); err != nil {
		t.Fatal(err)
	}

	reread, err := op.ListQuestionsContext(ctx, "", 64)
	if err != nil || reread.Outcome != wire.OperatorPageOutcomePage || len(reread.Records) != 1 {
		t.Fatal("re-read page", reread, err)
	}
	if record := reread.Records[0]; record.ID != asked.Answer.ID || record.Option != "refuse" {
		t.Fatalf("re-read must show the answer given elsewhere: %+v", record)
	}

	late, err := op.AnswerQuestionContext(ctx, asked.Answer.ID, "allow")
	if err != nil || late.Outcome != wire.OperatorDecisionOutcomeConflict || late.Record != nil {
		t.Fatalf("a late different answer must conflict without a record: %+v %v", late, err)
	}
	same, err := op.AnswerQuestionContext(ctx, asked.Answer.ID, "refuse")
	if err != nil || same.Outcome != wire.OperatorDecisionOutcomeAnswered || same.Record == nil ||
		same.Record.Answered != elsewhere.Answered.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("a late same answer must replay the first answer's time: %+v %v (first %s)", same, err, elsewhere.Answered)
	}
	gone, err := op.AnswerQuestionContext(ctx, toDismiss.Answer.ID, "allow")
	if err != nil || gone.Outcome != wire.OperatorDecisionOutcomeUnknown || gone.Record != nil {
		t.Fatalf("an answer to a dismissed question must be unknown: %+v %v", gone, err)
	}

	observed, err := app.ObserveContext(ctx, "request", 0)
	if err != nil || observed.Outcome != wire.ObservationOutcomeAnswered || observed.Answer.Option != "refuse" {
		t.Fatalf("the application must observe the first recorded answer: %+v %v", observed, err)
	}
	observed, err = app.ObserveContext(ctx, "dismissed", 0)
	if err != nil || observed.Outcome != wire.ObservationOutcomeGone {
		t.Fatalf("the application must observe the dismissal as gone: %+v %v", observed, err)
	}
}
