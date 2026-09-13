package application

import (
	"context"
	"errors"
	"fmt"
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	cas "github.com/openabstractions/abstraction-cas/go"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func liveOperator(t *testing.T, b *asks.Book, authorize AuthorizeOperator) (*Host, *client.Client, *client.Operator, string) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("Program process/path evidence is not implemented on macOS")
	}
	endpoint := listen.Endpoint(fmt.Sprintf("asks-operator-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	if authorize != nil {
		if err = h.EnableOperator(authorize); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 1)
	go func() { done <- h.Serve(context.Background()) }()
	t.Cleanup(func() {
		h.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("operator host did not drain")
		}
	})
	return h, client.New(endpoint), client.NewOperator(endpoint), endpoint
}
func TestOperatorAuthorizedAnswerHistoryReplayAndRestart(t *testing.T) {
	b, path := book(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var all, denied atomic.Bool
	authorize := func(_ context.Context, peer *identity.Peer) error {
		path, err := peer.Path.AtLeast(listen.Program.Path)
		if err != nil {
			return err
		}
		if denied.Load() || (!all.Load() && filepath.Clean(path) != filepath.Clean(exe)) {
			return ErrOperatorForbidden
		}
		return nil
	}
	h, app, op, endpoint := liveOperator(t, b, authorize)
	q := question()
	first, err := app.AskContext(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	q.RequestKey = "second"
	q.Slots["host"] = "second.example"
	_, err = app.AskContext(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	page, err := op.ListQuestionsContext(context.Background(), "", 1)
	if err != nil || len(page.Records) != 1 || page.Next == "" || page.Complete {
		t.Fatal(page, err)
	}
	replay, err := op.ListQuestionsContext(context.Background(), page.Next, 1)
	if err != nil || !replay.Complete || len(replay.Records) != 1 {
		t.Fatal(replay, err)
	}
	repeated, err := op.ListQuestionsContext(context.Background(), page.Next, 1)
	if err != nil || !reflect.DeepEqual(repeated, replay) {
		t.Fatal("continuation replay changed", repeated, err)
	}
	// A distinct actual executable under this account has no operator grant.
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(t.TempDir(), "ordinary.exe")
	if err = os.WriteFile(copied, data, 0700); err != nil {
		t.Fatal(err)
	}
	child := func(want string) {
		t.Helper()
		cmd := exec.Command(copied, "-test.run=^TestOperatorCallerProcess$")
		cmd.Env = append(os.Environ(), "OA_OPERATOR_ENDPOINT="+endpoint, "OA_OPERATOR_CURSOR="+page.Next, "OA_OPERATOR_ID="+first.Answer.Id, "OA_OPERATOR_WANT="+want)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("operator child: %v %s", err, output)
		}
	}
	child("forbidden")
	all.Store(true)
	child("gap")
	all.Store(false)
	answer, err := op.AnswerQuestionContext(context.Background(), first.Answer.Id, "once")
	if err != nil || answer.Outcome != "answered" || answer.Record.Kept {
		t.Fatal(answer, err)
	}
	again, err := op.AnswerQuestionContext(context.Background(), first.Answer.Id, "once")
	if err != nil || !reflect.DeepEqual(again, answer) {
		t.Fatal("answer replay changed", again, err)
	}
	conflict, err := op.AnswerQuestionContext(context.Background(), first.Answer.Id, "refuse")
	if err != nil || conflict.Outcome != "conflict" {
		t.Fatal(conflict, err)
	}
	gap, err := op.ListQuestionsContext(context.Background(), page.Next, 1)
	if err != nil || gap.Outcome != "gap" {
		t.Fatal("edit gap hidden", gap, err)
	}
	observed, err := app.ObserveContext(context.Background(), "request", 0)
	if err != nil || observed.Outcome != "answered" || observed.Answer.Option != "once" {
		t.Fatal(observed, err)
	}
	denied.Store(true)
	refusal, err := op.ListQuestionsContext(context.Background(), "", 1)
	if err != nil || refusal.Outcome != "forbidden" {
		t.Fatal(refusal, err)
	}
	denied.Store(false)
	h.Close()
	freshBook, err := asks.LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _, fresh, _ := liveOperator(t, freshBook, authorize)
	after, err := fresh.AnswerQuestionContext(context.Background(), first.Answer.Id, "once")
	if err != nil || !reflect.DeepEqual(after, answer) {
		t.Fatal("restart changed answer", after, err)
	}
	gap, err = fresh.ListQuestionsContext(context.Background(), page.Next, 1)
	if err != nil || gap.Outcome != "gap" {
		t.Fatal("restart gap hidden", gap, err)
	}
}
func TestOperatorCallerProcess(t *testing.T) {
	endpoint := os.Getenv("OA_OPERATOR_ENDPOINT")
	if endpoint == "" {
		return
	}
	op := client.NewOperator(endpoint)
	page, err := op.ListQuestionsContext(context.Background(), os.Getenv("OA_OPERATOR_CURSOR"), 1)
	if err != nil || page.Outcome != os.Getenv("OA_OPERATOR_WANT") {
		os.Exit(3)
	}
	if page.Outcome == "forbidden" {
		answer, err := op.AnswerQuestionContext(context.Background(), os.Getenv("OA_OPERATOR_ID"), "allow")
		if err != nil || answer.Outcome != "forbidden" {
			os.Exit(4)
		}
	}
	os.Exit(0)
}
func TestOperatorUnconfiguredAndCanceledRefusesWithoutEffects(t *testing.T) {
	b, _ := book(t)
	h, app, op, _ := liveOperator(t, b, nil)
	pending, err := app.AskContext(context.Background(), question())
	if err != nil {
		t.Fatal(err)
	}
	answer, err := op.AnswerQuestionContext(context.Background(), pending.Answer.Id, "allow")
	if err != nil || answer.Outcome != "forbidden" || h.OperatorAvailable() {
		t.Fatal(answer, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = op.AnswerQuestionContext(ctx, pending.Answer.Id, "allow"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	observed, err := app.ObserveContext(context.Background(), "request", 0)
	if err != nil || observed.Outcome != "pending" {
		t.Fatal(observed, err)
	}
	if err = h.EnableOperator(func(context.Context, *identity.Peer) error { return nil }); err == nil {
		t.Fatal("live authority configuration accepted")
	}
}

func TestOperatorWireBoundsAndStorageRefusal(t *testing.T) {
	b, path := book(t)
	var denied atomic.Bool
	authorize := func(context.Context, *identity.Peer) error {
		if denied.Load() {
			return ErrOperatorForbidden
		}
		return nil
	}
	_, app, _, endpoint := liveOperator(t, b, authorize)
	pending, err := app.AskContext(context.Background(), question())
	if err != nil {
		t.Fatal(err)
	}
	c := wire.NewQuestionOperatorClient(listen.FrameClient{Endpoint: endpoint, Timeout: time.Second, MaxFrame: MaxFrameBytes})
	for _, limit := range []int64{0, 65} {
		page, err := c.ListQuestions("", limit)
		if err != nil || page.Outcome != "invalid" {
			t.Fatal(page, err)
		}
	}
	page, err := c.ListQuestions(strings.Repeat("x", 257), 1)
	if err != nil || page.Outcome != "invalid" {
		t.Fatal(page, err)
	}
	answer, err := c.AnswerQuestion(pending.Answer.Id, strings.Repeat("x", 129))
	if err != nil || answer.Outcome != "invalid" {
		t.Fatal(answer, err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(`{"broken":`), 0600); err != nil {
		t.Fatal(err)
	}
	denied.Store(true)
	page, err = c.ListQuestions("", 1)
	if err != nil || page.Outcome != "forbidden" {
		t.Fatal("authorization did not precede storage", page, err)
	}
	denied.Store(false)
	page, err = c.ListQuestions("", 1)
	if err != nil || page.Outcome != "unavailable" || len(page.Records) != 0 {
		t.Fatal("storage error hidden", page, err)
	}
	if err = os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	page, err = c.ListQuestions("", 64)
	if err != nil || page.Outcome != "page" || len(page.Records) != 1 {
		t.Fatal("fresh reuse failed", page, err)
	}
}

func TestOperatorCanceledWhileEditWaitsDoesNotAnswer(t *testing.T) {
	b, path := book(t)
	entered := make(chan struct{}, 1)
	h, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		return nil
	})
	pending, err := app.AskContext(context.Background(), question())
	if err != nil {
		t.Fatal(err)
	}
	locked, release := make(chan struct{}), make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- cas.ChangeLimit(path, asks.MaxApplicationBytes, func(data []byte) ([]byte, error) { close(locked); <-release; return data, nil })
	}()
	<-locked
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := op.AnswerQuestionContext(ctx, pending.Answer.Id, "once"); done <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("operator not admitted before edit wait")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
	case <-time.After(2 * time.Second):
		t.Error("caller cancellation did not return")
	}
	close(release)
	if err := <-lockDone; err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(h.slots) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("cancelled edit retained slot")
		}
		runtime.Gosched()
	}
	observed, err := app.ObserveContext(context.Background(), "request", 0)
	if err != nil || observed.Outcome != "pending" {
		t.Fatal("cancelled lock wait answered", observed, err)
	}
	answer, err := op.AnswerQuestionContext(context.Background(), pending.Answer.Id, "once")
	if err != nil || answer.Outcome != "answered" {
		t.Fatal("fresh operator failed", answer, err)
	}
}
func TestOperatorPolicyOutageIsUnavailable(t *testing.T) {
	b, _ := book(t)
	_, app, op, _ := liveOperator(t, b, func(context.Context, *identity.Peer) error { return errors.New("policy connection failed") })
	pending, err := app.AskContext(context.Background(), question())
	if err != nil {
		t.Fatal(err)
	}
	page, err := op.ListQuestionsContext(context.Background(), "", 1)
	if err != nil || page.Outcome != "unavailable" {
		t.Fatal(page, err)
	}
	answer, err := op.AnswerQuestionContext(context.Background(), pending.Answer.Id, "once")
	if err != nil || answer.Outcome != "unavailable" {
		t.Fatal(answer, err)
	}
	observed, err := app.ObserveContext(context.Background(), "request", 0)
	if err != nil || observed.Outcome != "pending" {
		t.Fatal(observed, err)
	}
}

type measuredOperatorTransport struct {
	listen.FrameClient
	replyBytes int
}

func (m *measuredOperatorTransport) ExchangeFrame(frame []byte) ([]byte, error) {
	reply, err := m.FrameClient.ExchangeFrame(frame)
	m.replyBytes = len(reply)
	return reply, err
}
func TestOperatorFullPageEncodedBudget(t *testing.T) {
	b, _ := book(t)
	for i := 0; i < 65; i++ {
		_, _, err := b.AskApplication("scope", "application", fmt.Sprint(i), "download.reach", map[string]string{"host": strings.Repeat("界", 190) + fmt.Sprint(i)}, listen.Seen{})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, _, _, endpoint := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	transport := &measuredOperatorTransport{FrameClient: listen.FrameClient{Endpoint: endpoint, Timeout: 5 * time.Second, MaxFrame: MaxFrameBytes}}
	c := wire.NewQuestionOperatorClient(transport)
	page, err := c.ListQuestions("", 64)
	if err != nil || page.Outcome != "page" || len(page.Records) != 64 || page.Complete || transport.replyBytes > 256<<10 {
		t.Fatal("full encoded page exceeds contract", len(page.Records), transport.replyBytes, err)
	}
	end, err := c.ListQuestions(page.Next, 64)
	if err != nil || !end.Complete || len(end.Records) != 1 {
		t.Fatal(end, err)
	}
}
