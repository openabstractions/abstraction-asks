package application

import (
	"context"
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

// TestOperatorReadQuestionReportsRecordedPublisher: ReadQuestion returns one
// retained question with the publisher its admitting service recorded
// (ASK-A6, ASK-O7). An application's own admission records its bound code.
func TestOperatorReadQuestionReportsRecordedPublisher(t *testing.T) {
	b, _ := book(t)
	var deny bool
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error {
		if deny {
			return ErrOperatorForbidden
		}
		return nil
	})
	ctx := context.Background()
	slots := map[string]string{"program": filepath.Join(t.TempDir(), "opencode.exe"), "action": "abstraction.model/lookup", "resource": "hf"}
	record, _, err := b.AskApplicationFrom("runtime-scope", "runtime", "first-use", asks.FirstUseKey, slots, listen.Seen{},
		asks.Publisher{Subject: "Test Signer", Trusted: true, Description: "OpenCode", OriginalName: "opencode.exe"})
	if err != nil {
		t.Fatal(err)
	}
	attributedRead, err := op.ReadAttributedQuestionContext(ctx, record.ID)
	read := attributedRead.Question
	if err != nil || read.Outcome != wire.OperatorQuestionOutcomeFound || read.Record == nil || read.Record.ID != record.ID ||
		read.Record.About != slots["program"] || read.Publisher != "Test Signer" || !read.PublisherTrusted ||
		read.ProgramDescription != "OpenCode" || read.ProgramOriginalName != "opencode.exe" {
		t.Fatalf("read must carry the record and its recorded publisher: %+v %v", read, err)
	}
	attributedAbsent, err := op.ReadAttributedQuestionContext(ctx, "0123456789abcdef0123456789abcdef")
	absent := attributedAbsent.Question
	if err != nil || absent.Outcome != wire.OperatorQuestionOutcomeUnknown || absent.Record != nil || absent.Publisher != "" {
		t.Fatalf("an absent question must read unknown: %+v %v", absent, err)
	}

	asked, err := app.AskContext(ctx, question())
	if err != nil || asked.Outcome != wire.ObservationOutcomePending {
		t.Fatal(asked, err)
	}
	attributedOwn, err := op.ReadAttributedQuestionContext(ctx, asked.Answer.ID)
	own := attributedOwn.Question
	if err != nil || own.Outcome != wire.OperatorQuestionOutcomeFound || own.Record == nil {
		t.Fatalf("an application admission must read back: %+v %v", own, err)
	}
	stored, _, err := b.ApplicationRecord(asked.Answer.ID)
	if err != nil || stored.Publisher != own.Publisher || stored.PublisherTrusted != own.PublisherTrusted {
		t.Fatalf("read must report the stored evidence: %+v %+v %v", stored, own, err)
	}

	deny = true
	attributedRefused, err := op.ReadAttributedQuestionContext(ctx, record.ID)
	refused := attributedRefused.Question
	if err != nil || refused.Outcome != wire.OperatorQuestionOutcomeForbidden || refused.Record != nil || refused.Publisher != "" || refused.ProgramDescription != "" {
		t.Fatalf("an unauthorized read must be forbidden without content: %+v %v", refused, err)
	}
}

func TestAttributedReadKeepsOriginalClosedQuestionShape(t *testing.T) {
	b, _ := book(t)
	_, _, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	actor := filepath.Join(t.TempDir(), "answerer.exe")
	record, _, err := b.AskApplication("runtime", "runtime", "answer-one", asks.FirstUseKey,
		map[string]string{"program": filepath.Join(t.TempDir(), "target.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if _, outcome, err := b.AnswerApplicationAsResponder(record.ID, "allow", actor, nil); err != nil || outcome != "answered" {
		t.Fatalf("answer %s %v", outcome, err)
	}
	attributedOld, err := op.ReadAttributedQuestionContext(context.Background(), record.ID)
	old := attributedOld.Question
	if err != nil || old.Outcome != wire.OperatorQuestionOutcomeFound {
		t.Fatalf("old read %+v %v", old, err)
	}
	oldJSON, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(oldJSON) || strings.Contains(string(oldJSON), "Answerer") {
		t.Fatalf("old question leaked actor: %s", oldJSON)
	}
	// The old closed type has no actor member; only the new result exposes it.
	attributed, err := op.ReadAttributedQuestionContext(context.Background(), record.ID)
	if err != nil || !reflect.DeepEqual(attributed.Question, old) || attributed.Answerer != actor {
		t.Fatalf("attributed read %+v %v", attributed, err)
	}
}

func TestStructuredDetailsAreAdditiveAndPublicAdmissionCannotMintPermission(t *testing.T) {
	b, _ := book(t)
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	owner, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	permission, err := asks.NewPermissionRequest(asks.PermissionTarget{Account: owner.Uid, Program: filepath.Join(t.TempDir(), strings.Repeat("p", 220), "app.exe"), Action: "abstraction.model/lookup", Resource: strings.Repeat("r", 1024)})
	if err != nil {
		t.Fatal(err)
	}
	record, outcome, err := b.AskPermissionFrom("runtime", "runtime", "long", permission, listen.Seen{}, asks.Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	askerDetails, err := op.ReadQuestionAskerContext(context.Background(), record.ID)
	details := askerDetails.Details
	if err != nil || details.Permission == nil || !reflect.DeepEqual(*details.Permission, permission) {
		t.Fatalf("details %+v %v", details, err)
	}
	attributedOld, err := op.ReadAttributedQuestionContext(context.Background(), record.ID)
	old := attributedOld.Question
	if err != nil {
		t.Fatal(err)
	}
	attributed, err := op.ReadAttributedQuestionContext(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(details.Question, attributed) || !reflect.DeepEqual(old, attributed.Question) {
		t.Fatal("existing response changed")
	}
	data, _ := json.Marshal(old)
	if strings.Contains(string(data), "Permission") || strings.Contains(string(data), "Targets") {
		t.Fatalf("old closed result changed: %s", data)
	}
	q := question()
	q.Key = asks.FirstUseKey
	q.Slots = map[string]string{"program": permission.Targets[0].Program, "action": "abstraction.model/lookup", "resource": "hf"}
	attempted, err := app.AskContext(context.Background(), q)
	if err != nil || attempted.Outcome != wire.ObservationOutcomeForbidden || attempted.Answer != nil {
		t.Fatalf("public minted permission: %+v %v", attempted, err)
	}
	// The runtime's question about a self-registered app is its own too: an
	// application cannot raise it to have its own title vouched for.
	q = question()
	q.Key = asks.SelfRegisteredKey
	q.Slots = map[string]string{"program": permission.Targets[0].Program, "title": "OpenAbstractions Helper"}
	attempted, err = app.AskContext(context.Background(), q)
	if err != nil || attempted.Outcome != wire.ObservationOutcomeForbidden || attempted.Answer != nil {
		t.Fatalf("application raised the self-registration question: %+v %v", attempted, err)
	}
}

// Metadata does not recover an effect. A valid old Book with a truncated
// first-use sentence remains inspectable while its trusted target read refuses.
func TestMetadataReadSurvivesUnrecoverableLegacyPermission(t *testing.T) {
	b, path := book(t)
	record, _, err := b.AskApplication("runtime", "runtime", "legacy", asks.FirstUseKey,
		map[string]string{"program": filepath.Join(t.TempDir(), "target.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	record.Permission = nil
	record.Text = "legacy first-use target was truncated"
	raw, err := json.Marshal(struct {
		Asks    []asks.Record `json:"asks"`
		Profile string        `json:"profile,omitempty"`
	}{[]asks.Record{record}, "asks-application@2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := b.ApplicationRecord(record.ID); err != nil || !found {
		t.Fatalf("fixture must be a readable canonical legacy Book: %v %v", found, err)
	}
	_, _, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	metadata, err := op.ReadAttributedQuestionContext(context.Background(), record.ID)
	if err != nil || metadata.Question.Outcome != wire.OperatorQuestionOutcomeFound || metadata.Question.Record == nil || metadata.Question.Record.Text != record.Text {
		t.Fatalf("metadata must remain readable: %+v %v", metadata, err)
	}
	details, err := op.ReadQuestionAskerContext(context.Background(), record.ID)
	if err != nil || details.Details.Question.Question.Outcome != wire.OperatorQuestionOutcomeUnavailable || details.Details.Question.Question.Record != nil || details.Details.Permission != nil || details.Program != "" {
		t.Fatalf("unrecoverable authority must be unavailable: %+v %v", details, err)
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(raw) {
		t.Fatal("reads must not migrate or rewrite unrecoverable legacy state", err)
	}
}
