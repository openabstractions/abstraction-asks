package application

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

func customChoice(key string) wire.ChoiceRequest {
	now := time.Now()
	return wire.ChoiceRequest{RequestRef: wire.RequestRef{Key: key, ReplayUntilMs: now.Add(time.Hour).UnixMilli()}, Title: "Keep this output for this delivery?", Options: []wire.ChoiceOption{{ID: "keep", Label: "Keep"}}, PendingUntilMs: now.Add(time.Minute).UnixMilli()}
}
func TestOrdinaryGoClientNativeOperatorAndLegacyIsolation(t *testing.T) {
	b, path := book(t)
	h, old, op, endpoint := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	c := client.NewChoice(endpoint)
	q := customChoice("custom")
	pending, err := c.AskContext(context.Background(), q)
	if err != nil || pending.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(pending, err)
	}
	page, err := op.ListChoicesContext(context.Background(), "", 64)
	if err != nil || len(page.Records) != 1 || page.Records[0].ID != pending.ID || page.Records[0].Request.Title != q.Title {
		t.Fatal(page, err)
	}
	legacy, err := op.ListQuestionsContext(context.Background(), "", 64)
	if err != nil || len(legacy.Records) != 0 {
		t.Fatal("ordinary record leaked into legacy page", legacy, err)
	}
	askerDetails, err := op.ReadQuestionAskerContext(context.Background(), pending.ID)
	details := askerDetails.Details
	if err != nil || details.Question.Question.Outcome != wire.OperatorQuestionOutcomeUnknown || details.Permission != nil {
		t.Fatal(details, err)
	}
	// The old admission/result still decodes and keeps its own catalogue.
	if r, err := old.AskContext(context.Background(), question()); err != nil || r.Outcome != wire.ObservationOutcomePending {
		t.Fatal(r, err)
	}
	waited := make(chan wire.ChoiceObservation, 1)
	go func() { result, _ := c.ObserveContext(context.Background(), q.RequestRef, 1000); waited <- result }()
	select {
	case r := <-waited:
		t.Fatal("pending wait returned before an answer", r)
	case <-time.After(30 * time.Millisecond):
	}
	answer, err := op.AnswerChoiceContext(context.Background(), pending.ID, "keep")
	if err != nil || answer.Outcome != wire.ChoiceDecisionOutcomeAnswered || answer.Record.Answer.AnsweredBy == "" {
		t.Fatal(answer, err)
	}
	select {
	case r := <-waited:
		if r.Outcome != wire.ChoiceOutcomeAnswered {
			t.Fatal(r)
		}
	case <-time.After(time.Second):
		t.Fatal("answer did not wake observer")
	}
	if r, err := op.AnswerChoiceContext(context.Background(), pending.ID, "keep"); err != nil || r.Outcome != wire.ChoiceDecisionOutcomeConflict {
		t.Fatal(r, err)
	}
	if r, err := op.ReconcileChoiceAnswerContext(context.Background(), pending.ID, "keep"); err != nil || !reflect.DeepEqual(r, answer) {
		t.Fatal(r, err)
	}
	h.Close()
	restarted, err := asks.LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, newEndpoint := liveOperator(t, restarted, func(context.Context, *identity.Peer) error { return nil })
	c = client.NewChoice(newEndpoint)
	if replay, err := c.AskContext(context.Background(), q); err != nil || !reflect.DeepEqual(replay.Answer, answer.Record.Answer) {
		t.Fatal(replay, err)
	}
	for range 2 {
		if ack, err := c.AcknowledgeContext(context.Background(), q.RequestRef, *answer.Record.Answer); err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
			t.Fatal(ack, err)
		}
	}
}
func TestOrdinaryPendingDeadlineWakesAndOperatorIsRequired(t *testing.T) {
	b, _ := book(t)
	_, _, denied, endpoint := liveOperator(t, b, nil)
	c := client.NewChoice(endpoint)
	q := customChoice("short")
	q.PendingUntilMs = time.Now().Add(150 * time.Millisecond).UnixMilli()
	pending, err := c.AskContext(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := denied.AnswerChoiceContext(context.Background(), pending.ID, "keep"); err != nil || r.Outcome != wire.ChoiceDecisionOutcomeForbidden {
		t.Fatal(r, err)
	}
	started := time.Now()
	r, err := c.ObserveContext(context.Background(), q.RequestRef, 1000)
	if err != nil || r.Outcome != wire.ChoiceOutcomeExpired || time.Since(started) >= 700*time.Millisecond {
		t.Fatal(r, err, time.Since(started))
	}
}

// Python exchanges actual generated frames with the Go dispatcher and Book.
// Native peer attribution is covered by the live service test above; this
// supplied test transport adds no production transport or activation path.
type pythonChoiceBook struct {
	book  *asks.Book
	actor string
}

func (p *pythonChoiceBook) Ask(q wire.ChoiceRequest) (wire.ChoiceObservation, error) {
	return p.book.AskChoiceFrom("python-fixture", "Python application", q, listen.Seen{}, asks.Publisher{}, nil)
}
func (p *pythonChoiceBook) ShareChoice(q wire.ChoiceRequest, shared []string) (wire.ChoiceObservation, error) {
	return p.book.AskChoiceAs("python-fixture", p.actor, q, shared, listen.Seen{}, asks.Publisher{}, nil)
}
func (p *pythonChoiceBook) Observe(ref wire.RequestRef, waitMS int64) (wire.ChoiceObservation, error) {
	return p.book.ObserveChoice("python-fixture", ref)
}
func (p *pythonChoiceBook) Answer(ref wire.RequestRef, option string, ifPending bool) (wire.ChoiceDecision, error) {
	return p.book.DecideOwnChoiceAuthorized("python-fixture", ref, option, p.actor, false, ifPending, nil)
}
func (p *pythonChoiceBook) Dismiss(ref wire.RequestRef) (wire.ChoiceDecision, error) {
	return p.book.DecideOwnChoiceAuthorized("python-fixture", ref, "", p.actor, true, false, nil)
}
func (p *pythonChoiceBook) Acknowledge(ref wire.RequestRef, answer wire.ChoiceAnswer) (wire.ChoiceAcknowledgement, error) {
	return p.book.AcknowledgeChoice("python-fixture", ref, answer, nil)
}

func TestOrdinaryPythonGeneratedClientAndBook(t *testing.T) {
	python, args := "python3", []string{}
	if runtime.GOOS == "windows" {
		python, args = "py", []string{"-3.12"}
	}
	if _, err := exec.LookPath(python); err != nil {
		t.Skip("Python interpreter unavailable")
	}
	b, _ := book(t)
	actor, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pythonPath := generatedClientPythonPath(t, python, args)
	args = append(args, "testdata/choice_client.py")
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Env = append(os.Environ(), "PYTHONPATH="+pythonPath, "PYTHONIOENCODING=utf-8")
	cmd.Stderr = os.Stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	provider := &pythonChoiceBook{book: b, actor: identity.CanonicalProgramPath(actor)}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), MaxFrameBytes)
	for scanner.Scan() {
		frame, err := base64.StdEncoding.DecodeString(scanner.Text())
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		reply, err := wire.ServeEndpoint(frame, "test-provider", "", &wire.ChoiceApplicationDispatcher{Handler: provider})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if _, err = fmt.Fprintln(input, base64.StdEncoding.EncodeToString(reply)); err != nil {
			cancel()
			t.Fatal(err)
		}
	}
	if err := scanner.Err(); err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	legacy, _, err := b.ApplicationHistory()
	if err != nil || len(legacy) != 0 {
		t.Fatal("ordinary choice created permission records", legacy, err)
	}
}
