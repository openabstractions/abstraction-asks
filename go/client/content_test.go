package client

import (
	"context"
	"errors"
	"testing"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
)

type contentFixture struct {
	detailsFixture
	content    QuestionContent
	contentErr error
}

func (f *contentFixture) ReadQuestionContentContext(context.Context, string) (QuestionContent, error) {
	return f.content, f.contentErr
}

func TestReadContentOnlyUnknownMethodUsesExistingMetadataCompatibility(t *testing.T) {
	q := QuestionAsker{Details: QuestionDetails{Question: AttributedQuestion{Question: OperatorQuestion{Outcome: OperatorQuestionOutcomeFound,
		Record: &RecordMetadata{ID: "question", Key: "download.reach", About: "example.com", Text: "different wording"}}}}}
	for _, err := range []error{context.DeadlineExceeded, errors.New("unknown_method"), &wire.ServiceError{Code: wire.ServiceErrorCodeHandlerError}} {
		f := &contentFixture{contentErr: err}
		if _, err := ReadContentContext(context.Background(), f, "account", "question"); err == nil || f.oldReads != 0 {
			t.Fatal(err, f.oldReads)
		}
	}
	f := &contentFixture{detailsFixture: detailsFixture{details: q.Details}, contentErr: &wire.ServiceError{Code: wire.ServiceErrorCodeUnknownMethod}}
	read, err := ReadContentContext(context.Background(), f, "account", "question")
	if err != nil || read.Content != nil || read.Question.Details.Question.Question.Record.ID != "question" {
		t.Fatal(read, err)
	}
	modern := &contentFixture{content: QuestionContent{Question: q, Content: &CatalogueContent{Slots: map[string]string{"host": "example.com"}}}}
	read, err = ReadContentContext(context.Background(), modern, "account", "question")
	if err != nil || read.Content == nil {
		t.Fatal(read, err)
	}
	modern.content.Content.Slots["host"] = "different.example"
	if _, err = ReadContentContext(context.Background(), modern, "account", "question"); err == nil {
		t.Fatal("mismatched content fell back")
	}
	modern.content.Content = &CatalogueContent{Slots: map[string]string{"host": "example.com"}}
	modern.content.Question.Details.Question.Question.Outcome = OperatorQuestionOutcomeForbidden
	modern.content.Question.Details.Question.Question.Record = nil
	if _, err = ReadContentContext(context.Background(), modern, "account", "question"); err == nil {
		t.Fatal("refusal disclosed content")
	}
}
