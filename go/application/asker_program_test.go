package application

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

// TestOperatorReadsBoundAskerProgram: an application admission keeps the
// program the receiver bound, and only the opt-in operator read returns it
// (ASK-A1, ASK-O11). The record's asker is the name derived from it.
func TestOperatorReadsBoundAskerProgram(t *testing.T) {
	b, _ := book(t)
	var deny bool
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error {
		if deny {
			return ErrOperatorForbidden
		}
		return nil
	})
	ctx := context.Background()
	asked, err := app.AskContext(ctx, question())
	if err != nil || asked.Outcome != wire.ObservationOutcomePending {
		t.Fatal(asked, err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	read, err := op.ReadQuestionAskerContext(ctx, asked.Answer.ID)
	if err != nil || !identity.SameSubjectProgram(read.Program, filepath.Clean(self)) {
		t.Fatalf("the read must carry the bound program %s: %+v %v", self, read, err)
	}
	record := read.Details.Question.Question.Record
	if record == nil || record.Asker != asks.AskerDisplay(read.Program) {
		t.Fatalf("the asker must be derived from the program: %+v", read)
	}
	askerDetails, err := op.ReadQuestionAskerContext(ctx, asked.Answer.ID)
	details := askerDetails.Details
	if err != nil || !reflect.DeepEqual(details, read.Details) {
		t.Fatalf("the asker read must wrap the unchanged details: %+v %v", details, err)
	}
	shared, err := client.ReadAskerContext(ctx, op, "unused", asked.Answer.ID)
	if err != nil || shared.Program != read.Program {
		t.Fatalf("the shared reader must return the program: %+v %v", shared, err)
	}

	// The runtime's own question keeps no asker program.
	slots := map[string]string{"program": filepath.Join(t.TempDir(), "target.exe"), "action": "abstraction.model/lookup", "resource": "hf"}
	runtimeRecord, _, err := b.AskApplication("runtime-scope", "runtime", "first-use", asks.FirstUseKey, slots, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	own, err := op.ReadQuestionAskerContext(ctx, runtimeRecord.ID)
	if err != nil || own.Program != "" || own.Details.Question.Question.Outcome != wire.OperatorQuestionOutcomeFound {
		t.Fatalf("a record without a program reads none: %+v %v", own, err)
	}

	deny = true
	refused, err := op.ReadQuestionAskerContext(ctx, asked.Answer.ID)
	if err != nil || refused.Program != "" || refused.Details.Question.Question.Outcome != wire.OperatorQuestionOutcomeForbidden {
		t.Fatalf("a forbidden read must disclose no program: %+v %v", refused, err)
	}
}

// TestAskerProgramLeavesClosedOperatorResultsUnchanged is the old-reader
// regression for the asker program: every closed result an earlier operator@1
// client decodes keeps its exact member set, so the program cannot reach an
// old reader that refuses unknown fields (hiccup 2026-10-03).
func TestAskerProgramLeavesClosedOperatorResultsUnchanged(t *testing.T) {
	pinned := map[reflect.Type][]string{
		reflect.TypeOf(wire.RecordMetadata{}):     {"ID", "Asker", "Key", "About", "Text", "Options", "Asked", "Option", "Answered", "Kept", "Yes"},
		reflect.TypeOf(wire.OperatorQuestion{}):   {"Outcome", "Record", "Publisher", "PublisherTrusted", "AnsweredBy", "ProgramDescription", "ProgramOriginalName"},
		reflect.TypeOf(wire.AttributedQuestion{}): {"Question", "Answerer"},
		reflect.TypeOf(wire.QuestionDetails{}):    {"Question", "Permission"},
		reflect.TypeOf(wire.OperatorPage{}):       {"Outcome", "Records", "Next", "Complete"},
	}
	for typ, want := range pinned {
		var got []string
		for i := range typ.NumField() {
			got = append(got, typ.Field(i).Name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("closed %s changed: %v, want %v", typ.Name(), got, want)
		}
	}

	b, _ := book(t)
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	ctx := context.Background()
	asked, err := app.AskContext(ctx, question())
	if err != nil {
		t.Fatal(err)
	}
	program, err := op.ReadQuestionAskerContext(ctx, asked.Answer.ID)
	if err != nil || program.Program == "" {
		t.Fatalf("asker read %+v %v", program, err)
	}
	// Each earlier read decodes through its unchanged closed type and carries
	// only the derived display name, never the path.
	attributedOld, err := op.ReadAttributedQuestionContext(ctx, asked.Answer.ID)
	old := attributedOld.Question
	if err != nil {
		t.Fatal(err)
	}
	askerDetails, err := op.ReadQuestionAskerContext(ctx, asked.Answer.ID)
	details := askerDetails.Details
	if err != nil {
		t.Fatal(err)
	}
	page, err := op.ListQuestionsContext(ctx, "", 64)
	if err != nil || len(page.Records) != 1 {
		t.Fatal(page, err)
	}
	for _, value := range []any{old, details, page} {
		data, _ := json.Marshal(value)
		if strings.Contains(string(data), filepath.Dir(program.Program)) {
			t.Fatalf("an earlier result disclosed the program: %s", data)
		}
	}
}
