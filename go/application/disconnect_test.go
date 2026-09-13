package application

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/openabstractions/abstraction-asks/go/client"
	"github.com/openabstractions/abstraction-identity/listen"
	"os"
	"runtime"
	"testing"
	"time"
)

// This test wrapper reports the shared EOF reader's entry. It forwards all I/O
// and leaves framing, identity and disconnect handling to the shared transport.
type eofListener struct {
	listen.Listener
	entered chan struct{}
}

func (l *eofListener) Accept() (listen.Conn, error) {
	c, e := l.Listener.Accept()
	if e != nil {
		return nil, e
	}
	return &eofConn{Conn: c, entered: l.entered}, nil
}

type eofConn struct {
	listen.Conn
	entered  chan struct{}
	header   [4]byte
	seen     int
	size     uint32
	reported bool
}

func (c *eofConn) SetDeadline(at time.Time) error {
	return c.Conn.(interface{ SetDeadline(time.Time) error }).SetDeadline(at)
}
func (c *eofConn) Read(p []byte) (int, error) {
	if c.seen >= 4 && uint64(c.seen) >= 4+uint64(c.size) && !c.reported {
		c.reported = true
		c.entered <- struct{}{}
	}
	n, e := c.Conn.Read(p)
	for i := 0; i < n && c.seen+i < 4; i++ {
		c.header[c.seen+i] = p[i]
	}
	c.seen += n
	if c.seen >= 4 {
		c.size = binary.BigEndian.Uint32(c.header[:])
	}
	return n, e
}
func TestApplicationDisconnectReleasesSlotPreservesAdmission(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("current Program proof limitation")
	}
	b, _ := book(t)
	endpoint := listen.Endpoint(fmt.Sprintf("asks-disconnect-%d-%d", os.Getpid(), serial.Add(1)))
	h, e := Listen(endpoint, b)
	if e != nil {
		t.Fatal(e)
	}
	trace := &eofListener{Listener: h.listener, entered: make(chan struct{}, 8)}
	h.listener = trace
	hostDone := make(chan error, 1)
	go func() { hostDone <- h.Serve(context.Background()) }()
	defer func() {
		h.Close()
		select {
		case e := <-hostDone:
			if e != nil {
				t.Error(e)
			}
		case <-time.After(2 * time.Second):
			t.Error("host did not drain")
		}
	}()
	c := client.New(endpoint)
	admitted, e := c.AskContext(context.Background(), question())
	if e != nil || admitted.Outcome != "pending" {
		t.Fatal(admitted, e)
	}
	// Ask's successful reply has its own EOF drain; consume that event first.
	select {
	case <-trace.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("initial reply not drained")
	}
	waitEmpty := func() {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for len(h.slots) != 0 {
			if time.Now().After(deadline) {
				t.Fatalf("disconnected caller retained %d slots", len(h.slots))
			}
			runtime.Gosched()
		}
	}
	waitEmpty()
	c, e = c.WithTimeout(35 * time.Second)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, e := c.ObserveContext(ctx, question().RequestKey, 30000); done <- e }()
	select {
	case <-trace.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("observation did not begin shared disconnect wait")
	}
	if len(h.slots) != 1 {
		t.Fatal("missing active slot")
	}
	cancel()
	select {
	case e = <-done:
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("caller did not cancel")
	}
	waitEmpty()
	replay, e := c.AskContext(context.Background(), question())
	if e != nil || replay.Outcome != "pending" || replay.Answer.Id != admitted.Answer.Id {
		t.Fatal("admission lost", replay, e)
	}
	if _, e = b.Answer(admitted.Answer.Id, "once"); e != nil {
		t.Fatal(e)
	}
	observed, e := c.ObserveContext(context.Background(), question().RequestKey, 0)
	if e != nil || observed.Outcome != "answered" || observed.Answer.Id != admitted.Answer.Id {
		t.Fatal("decision lost", observed, e)
	}
}
