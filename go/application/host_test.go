package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	"github.com/openabstractions/abstraction-identity/listen"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var serial atomic.Uint64

func live(t *testing.T, b *asks.Book) (*Host, *client.Client, string) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("Program process/path evidence is not implemented on macOS")
	}
	endpoint := listen.Endpoint(fmt.Sprintf("asks-application-%d-%d", os.Getpid(), serial.Add(1)))
	h, e := Listen(endpoint, b)
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- h.Serve(context.Background()) }()
	t.Cleanup(func() {
		h.Close()
		select {
		case e := <-done:
			if e != nil {
				t.Error(e)
			}
		case <-time.After(2 * time.Second):
			t.Error("host did not drain")
		}
	})
	return h, client.New(endpoint), endpoint
}
func question() wire.ApplicationQuestion {
	return wire.ApplicationQuestion{RequestKey: "request", Key: "download.reach", Slots: map[string]string{"host": "example.com"}}
}
func book(t *testing.T) (*asks.Book, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "application.json")
	b, e := asks.LoadApplicationBook(path)
	if e != nil {
		t.Fatal(e)
	}
	return b, path
}
func TestApplicationWaitRestartAndOperatorDecision(t *testing.T) {
	b, path := book(t)
	h, c, _ := live(t, b)
	q := question()
	r, e := c.AskContext(context.Background(), q)
	if e != nil || r.Outcome != wire.ObservationOutcomePending {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, e = c.ObserveContext(ctx, q.RequestKey, 1000)
	if !errors.Is(e, context.DeadlineExceeded) && !os.IsTimeout(e) {
		t.Fatal(e)
	}
	pending, e := c.ObserveContext(context.Background(), q.RequestKey, 1)
	if e != nil || pending.Outcome != wire.ObservationOutcomePending || pending.Answer.Yes {
		t.Fatal(pending, e)
	}
	if _, e = b.Answer(r.Answer.ID, "once"); e != nil {
		t.Fatal(e)
	}
	answer, e := c.ObserveContext(context.Background(), q.RequestKey, 100)
	if e != nil || answer.Outcome != wire.ObservationOutcomeAnswered || answer.Answer.Option != "once" {
		t.Fatal(answer, e)
	}
	h.Close()
	reopened, e := asks.LoadApplicationBook(path)
	if e != nil {
		t.Fatal(e)
	}
	_, fresh, _ := live(t, reopened)
	replay, e := fresh.AskContext(context.Background(), q)
	if e != nil || replay.Answer.ID != r.Answer.ID || replay.Answer.Option != "once" {
		t.Fatal(replay, e)
	}
	if e = reopened.Forget(r.Answer.ID); e != nil {
		t.Fatal(e)
	}
	gone, e := fresh.AskContext(context.Background(), q)
	if e != nil || gone.Outcome != wire.ObservationOutcomeGone || gone.Answer != nil {
		t.Fatal(gone, e)
	}
}
func TestApplicationCrossExecutableCannotReuseAnswer(t *testing.T) {
	b, _ := book(t)
	_, c, endpoint := live(t, b)
	r, e := c.AskContext(context.Background(), question())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = b.Answer(r.Answer.ID, "allow"); e != nil {
		t.Fatal(e)
	}
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(executable)
	if e != nil {
		t.Fatal(e)
	}
	copyPath := filepath.Join(t.TempDir(), "different-program.exe")
	if e = os.WriteFile(copyPath, data, 0700); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(copyPath, "-test.run=^TestApplicationCallerProcess$")
	cmd.Env = append(os.Environ(), "OA_ASKS_CHILD="+endpoint)
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("child: %v %s", e, out)
	}
	var got wire.QuestionObservation
	if e = json.Unmarshal(out, &got); e != nil {
		t.Fatal(e, string(out))
	}
	if got.Outcome != wire.ObservationOutcomePending || got.Answer.ID == r.Answer.ID {
		t.Fatalf("cross-program reused answer: %+v", got)
	}
	records := b.List(true)
	if len(records) != 1 || !records[0].Via.Bound || records[0].Via.Path == "" || records[0].For != nil {
		t.Fatal("native evidence missing", records)
	}
}
func TestApplicationCallerProcess(t *testing.T) {
	endpoint := os.Getenv("OA_ASKS_CHILD")
	if endpoint == "" {
		return
	}
	c := client.New(endpoint)
	r, e := c.ObserveContext(context.Background(), "request", 0)
	if e != nil || r.Outcome != wire.ObservationOutcomeUnknown {
		os.Exit(4)
	}
	r, e = c.AskContext(context.Background(), question())
	if e != nil {
		os.Exit(5)
	}
	json.NewEncoder(os.Stdout).Encode(r)
	os.Exit(0)
}
func TestApplicationExpiredBeforeIOAndWrongAccount(t *testing.T) {
	b, _ := book(t)
	h, c, _ := live(t, b)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := c.AskContext(ctx, question()); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if len(b.List(true)) != 0 {
		t.Fatal("expired call admitted")
	}
	// Configure before a fresh host starts to avoid changing live authorization.
	h.Close()
	endpoint := listen.Endpoint(fmt.Sprintf("asks-denied-%d-%d", os.Getpid(), serial.Add(1)))
	denied, e := Listen(endpoint, b)
	if e != nil {
		t.Fatal(e)
	}
	denied.owner = "another-account"
	done := make(chan error, 1)
	go func() { done <- denied.Serve(context.Background()) }()
	defer func() { denied.Close(); <-done }()
	r, e := client.New(endpoint).AskContext(context.Background(), question())
	if e != nil || r.Outcome != wire.ObservationOutcomeForbidden || len(b.List(true)) != 0 {
		t.Fatal(r, e)
	}
}

type delayedListener struct {
	listen.Listener
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (l *delayedListener) Accept() (listen.Conn, error) {
	c, e := l.Listener.Accept()
	if e != nil {
		return nil, e
	}
	return &delayedConn{Conn: c, l: l}, nil
}

type delayedConn struct {
	listen.Conn
	l *delayedListener
}

func (c *delayedConn) SetDeadline(at time.Time) error {
	return c.Conn.(interface{ SetDeadline(time.Time) error }).SetDeadline(at)
}

func (c *delayedConn) Write(p []byte) (int, error) {
	c.l.once.Do(func() { close(c.l.entered); <-c.l.release })
	return c.Conn.Write(p)
}
func TestApplicationLostReplyReplaysSameAdmission(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Program process/path evidence is not implemented on macOS")
	}
	b, _ := book(t)
	endpoint := listen.Endpoint(fmt.Sprintf("asks-lost-%d-%d", os.Getpid(), serial.Add(1)))
	h, e := Listen(endpoint, b)
	if e != nil {
		t.Fatal(e)
	}
	delay := &delayedListener{Listener: h.listener, entered: make(chan struct{}), release: make(chan struct{})}
	h.listener = delay
	done := make(chan error, 1)
	go func() { done <- h.Serve(context.Background()) }()
	defer func() { h.Close(); <-done }()
	c := client.New(endpoint)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, e := c.AskContext(ctx, question()); result <- e }()
	select {
	case <-delay.entered:
	case <-time.After(2 * time.Second):
		close(delay.release)
		t.Fatal("admission never reached reply")
	}
	cancel()
	if e = <-result; !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	records := b.List(true)
	if len(records) != 1 {
		t.Fatal(records)
	}
	close(delay.release)
	replay, e := c.AskContext(context.Background(), question())
	if e != nil || replay.Answer.ID != records[0].ID || len(b.List(true)) != 1 {
		t.Fatal(replay, e)
	}
}
