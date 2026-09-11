package asks

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

type responseConn struct {
	net.Conn
	reader   io.Reader
	cancel   context.CancelFunc
	writeErr error
}

func (c *responseConn) Close() error { return nil }
func (c *responseConn) Write(p []byte) (int, error) {
	if c.writeErr != nil {
		c.cancel()
		return 0, c.writeErr
	}
	return len(p), nil
}
func (c *responseConn) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.cancel()
	return n, err
}

func TestCancellationDuringWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := exchange(ctx, &responseConn{cancel: cancel, writeErr: net.ErrClosed}, Request{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestUncanceledWriteErrorIsPreserved(t *testing.T) {
	failure := errors.New("transport failed")
	_, err := exchange(context.Background(), &responseConn{cancel: func() {}, writeErr: failure}, Request{})
	if !errors.Is(err, failure) {
		t.Fatalf("got %v", err)
	}
}

func TestCompletedResponseSurvivesCancellation(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		wantErr    bool
	}{
		{"success", "{}\n", false},
		{"refusal", "{\"error\":\"denied\"}\n", true},
		{"malformed", "invalid\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, err := exchange(ctx, &responseConn{reader: strings.NewReader(tc.wire), cancel: cancel}, Request{})
			if errors.Is(err, context.Canceled) || (err != nil) != tc.wantErr {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestCancellationReleasesResponseRead(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := exchange(ctx, client, Request{}); done <- err }()
	server.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := bufio.NewReader(server).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("response read remained blocked after cancellation")
	}
}

func TestAlreadyCanceledRequestDoesNotDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&Client{Endpoint: "absent"}).Await(ctx, Ask{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
