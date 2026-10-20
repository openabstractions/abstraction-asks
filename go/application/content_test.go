package application

import (
	"context"
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestOperatorContentCurrentRetiredPagesAndAuthorization(t *testing.T) {
	b, _ := book(t)
	deny, authorizations := false, 0
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error {
		authorizations++
		if deny {
			return ErrOperatorForbidden
		}
		return nil
	})
	ctx := context.Background()
	ids := []string{}
	for _, host := range []string{"first.example", "second.example"} {
		q := question()
		q.RequestKey, q.Slots = host, map[string]string{"host": host}
		asked, err := app.AskContext(ctx, q)
		if err != nil || asked.Outcome != wire.ObservationOutcomePending {
			t.Fatal(asked, err)
		}
		ids = append(ids, asked.Answer.ID)
		content, err := op.ReadQuestionContentContext(ctx, asked.Answer.ID)
		if err != nil || content.Content == nil || content.Content.Slots["host"] != host || content.Question.Details.Question.Question.Record.ID != asked.Answer.ID {
			t.Fatal(content, err)
		}
		if _, err := op.AnswerQuestionIfPendingContext(ctx, asked.Answer.ID, "refuse"); err != nil {
			t.Fatal(err)
		}
		if _, err := op.RetireQuestionContext(ctx, asked.Answer.ID); err != nil {
			t.Fatal(err)
		}
	}
	before := authorizations
	page, err := op.ListAnswerContentsContext(ctx, "", 1)
	if err != nil || page.Outcome != wire.OperatorPageOutcomePage || len(page.Answers) != 1 || page.Complete || page.Next == "" || authorizations-before != 2 {
		t.Fatal(page, err, authorizations-before)
	}
	if page.Answers[0].Answer.ID != ids[1] || page.Answers[0].Content.Slots["host"] != "second.example" {
		t.Fatal(page)
	}
	rest, err := op.ListAnswerContentsContext(ctx, page.Next, 64)
	if err != nil || !rest.Complete || len(rest.Answers) != 1 || rest.Answers[0].Answer.ID != ids[0] || rest.Answers[0].Content.Slots["host"] != "first.example" {
		t.Fatal(rest, err)
	}
	retired, err := op.ReadQuestionContentContext(ctx, ids[0])
	if err != nil || retired.Question.Details.Question.Question.Outcome != wire.OperatorQuestionOutcomeUnknown || retired.Content != nil {
		t.Fatal(retired, err)
	}
	q := question()
	q.RequestKey = "new revision"
	asked, err := app.AskContext(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.AnswerQuestionIfPendingContext(ctx, asked.Answer.ID, "refuse"); err != nil {
		t.Fatal(err)
	}
	gap, err := op.ListAnswerContentsContext(ctx, page.Next, 64)
	if err != nil || gap.Outcome != wire.OperatorPageOutcomeGap || len(gap.Answers) != 0 {
		t.Fatal(gap, err)
	}
	deny = true
	refused, err := op.ListAnswerContentsContext(ctx, "", 64)
	if err != nil || refused.Outcome != wire.OperatorPageOutcomeForbidden || len(refused.Answers) != 0 || refused.Next != "" {
		t.Fatal(refused, err)
	}
	hidden, err := op.ReadQuestionContentContext(ctx, asked.Answer.ID)
	if err != nil || hidden.Question.Details.Question.Question.Outcome != wire.OperatorQuestionOutcomeForbidden || hidden.Content != nil {
		t.Fatal(hidden, err)
	}
}

func TestOperatorContentAuditAndEncodedPageIncludeAddedBytes(t *testing.T) {
	b, _ := book(t)
	_, _, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	owner, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), strings.Repeat("a", 3500)+".exe")
	permission, err := asks.NewPermissionRequest(asks.PermissionTarget{Account: owner.Uid, Program: program, Action: asks.PermissionContentWrite, Resource: "output:image"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 32; i++ {
		r, _, err := b.AskPermissionFrom("runtime", "runtime", fmt.Sprint(i), permission, listen.Seen{}, asks.Publisher{})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := b.AnswerApplication(r.ID, "allow"); err != nil {
			t.Fatal(err)
		}
	}
	all, since, _, err := b.AuditedAnswerContents()
	if err != nil || len(all) == 0 || len(all) >= 32 || since == 0 {
		t.Fatal("added content did not count against audit bytes", len(all), since, err)
	}
	page, err := op.ListAnswerContentsContext(context.Background(), "", 64)
	if err != nil || page.Outcome != wire.OperatorPageOutcomePage || len(page.Answers) == 0 || len(wire.EncodeAnswerContentPage(&page)) > 256<<10 {
		t.Fatal(page, err)
	}
	for _, answer := range page.Answers {
		if answer.Content == nil || answer.Content.Permission == nil || len(answer.Content.Permission.Targets) != 2 || answer.Content.Permission.Targets[0].Program != program {
			t.Fatal(answer)
		}
	}
}
