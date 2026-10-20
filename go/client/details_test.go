package client

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
)

type detailsFixture struct {
	details  QuestionDetails
	err      error
	old      OperatorQuestion
	oldReads int
}

func (f *detailsFixture) ReadQuestionAskerContext(context.Context, string) (QuestionAsker, error) {
	return QuestionAsker{Details: f.details}, f.err
}
func (f *detailsFixture) ReadAttributedQuestionContext(context.Context, string) (AttributedQuestion, error) {
	f.oldReads++
	if f.old.Outcome == 0 {
		return f.details.Question, nil
	}
	return AttributedQuestion{Question: f.old}, nil
}

func TestDetailsFallbackRequiresExplicitUnknownMethod(t *testing.T) {
	program := filepath.Join(t.TempDir(), "app.exe")
	old := OperatorQuestion{Outcome: OperatorQuestionOutcomeFound, Record: &RecordMetadata{ID: "question", Key: asks.FirstUseKey, About: program, Text: program + " wants to abstraction.model/lookup on hf"}}
	for _, err := range []error{context.DeadlineExceeded, errors.New("unknown_method"), &wire.ServiceError{Code: wire.ServiceErrorCodeHandlerError}} {
		f := &detailsFixture{err: err, old: old}
		if _, err := ReadDetailsContext(context.Background(), f, "account", "question"); err == nil || f.oldReads != 0 {
			t.Fatalf("fallback after %v: reads %d", f.err, f.oldReads)
		}
	}
	f := &detailsFixture{err: &wire.ServiceError{Code: wire.ServiceErrorCodeUnknownMethod}, old: old}
	details, err := ReadDetailsContext(context.Background(), f, "account", "question")
	if err != nil || f.oldReads != 1 || details.Permission == nil || details.Permission.Targets[0].Program != program {
		t.Fatalf("legacy details %+v %v", details, err)
	}
}

type askerFixture struct {
	detailsFixture
	asker    QuestionAsker
	askerErr error
}

func (f *askerFixture) ReadQuestionAskerContext(context.Context, string) (QuestionAsker, error) {
	return f.asker, f.askerErr
}

// TestAskerReadFallsBackOnlyOnUnknownMethod: an older service without
// ReadQuestionAsker gives legacy metadata with no program; any other error is
// returned, and current details pass the same checks (ASK-O11).
func TestAskerReadFallsBackOnlyOnUnknownMethod(t *testing.T) {
	program := filepath.Join(t.TempDir(), "app.exe")
	q := OperatorQuestion{Outcome: OperatorQuestionOutcomeFound, Record: &RecordMetadata{ID: "question", Asker: "app.exe", Key: "download.reach", Text: "app.exe wants to fetch from example.com"}}
	details := QuestionDetails{Question: AttributedQuestion{Question: q}}
	current := &askerFixture{detailsFixture: detailsFixture{details: details}, asker: QuestionAsker{Details: details, Program: program}}
	read, err := ReadAskerContext(context.Background(), current, "account", "question")
	if err != nil || read.Program != program {
		t.Fatalf("current asker %+v %v", read, err)
	}
	older := &askerFixture{detailsFixture: detailsFixture{details: details}, askerErr: &wire.ServiceError{Code: wire.ServiceErrorCodeUnknownMethod}}
	read, err = ReadAskerContext(context.Background(), older, "account", "question")
	if err != nil || read.Program != "" || read.Details.Question.Question.Record.ID != "question" {
		t.Fatalf("older service %+v %v", read, err)
	}
	failing := &askerFixture{detailsFixture: detailsFixture{details: details}, askerErr: context.DeadlineExceeded}
	if _, err := ReadAskerContext(context.Background(), failing, "account", "question"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a failed read must not fall back: %v", err)
	}
	plain := &detailsFixture{details: details}
	read, err = ReadAskerContext(context.Background(), plain, "account", "question")
	if err != nil || read.Program != "" {
		t.Fatalf("a details-only reader %+v %v", read, err)
	}
	mismatched := &askerFixture{asker: QuestionAsker{Details: QuestionDetails{Question: AttributedQuestion{Question: OperatorQuestion{Outcome: OperatorQuestionOutcomeFound, Record: &RecordMetadata{ID: "other"}}}}, Program: program}}
	if _, err := ReadAskerContext(context.Background(), mismatched, "account", "question"); err == nil {
		t.Fatal("a mismatched occurrence must be refused")
	}
}

func TestAskerProgramMustDeriveTheDisplayName(t *testing.T) {
	program := filepath.Join(t.TempDir(), "app.exe")
	found := func(asker string) QuestionDetails {
		return QuestionDetails{Question: AttributedQuestion{Question: OperatorQuestion{Outcome: OperatorQuestionOutcomeFound, Record: &RecordMetadata{ID: "question", Asker: asker}}}}
	}
	if err := validateAskerProgram(found("app.exe"), program); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		details QuestionDetails
		program string
	}{
		{found("other.exe"), program},
		{found("app.exe"), "app.exe"},
		{QuestionDetails{Question: AttributedQuestion{Question: OperatorQuestion{Outcome: OperatorQuestionOutcomeForbidden}}}, program},
	} {
		if err := validateAskerProgram(c.details, c.program); err == nil {
			t.Fatalf("accepted %+v %q", c.details, c.program)
		}
	}
}

func TestDetailsNeverParsesCurrentMissingMalformedOrForeignPermission(t *testing.T) {
	program := filepath.Join(t.TempDir(), "app.exe")
	q := OperatorQuestion{Outcome: OperatorQuestionOutcomeFound, Record: &RecordMetadata{ID: "question", Key: asks.FirstUseKey, About: program, Text: program + " wants to abstraction.model/lookup on hf"}}
	p, _ := asks.NewPermissionRequest(asks.PermissionTarget{Account: "foreign", Program: program, Action: "abstraction.model/lookup", Resource: "hf"})
	for _, permission := range []*PermissionRequest{nil, &p, {Targets: []PermissionTarget{{Account: "account", Program: program, Action: "", Resource: "hf"}}}} {
		f := &detailsFixture{details: QuestionDetails{Question: AttributedQuestion{Question: q}, Permission: permission}, old: q}
		if _, err := ReadDetailsContext(context.Background(), f, "account", "question"); err == nil || f.oldReads != 0 {
			t.Fatalf("invalid current details fell back: %+v", permission)
		}
	}
}
