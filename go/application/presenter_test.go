package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/presenter"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestPresenterNativeLease(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("fixture requires Windows process creation evidence")
	}
	b, _ := book(t)
	endpoint := listen.Endpoint(fmt.Sprintf("asks-presenter-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	var deny, otherSession atomic.Bool
	if err := h.EnableOperator(func(context.Context, *identity.Peer) error {
		if deny.Load() {
			return ErrOperatorForbidden
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.EnablePresenter(func(peer *identity.Peer) (string, bool) {
		proc, err := peer.Process.AtLeast(identity.ProofKernel)
		if otherSession.Load() {
			return "other-session", err == nil && proc.PID > 0
		}
		return "fixture-session", err == nil && proc.PID > 0
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Serve(context.Background()) }()
	defer func() {
		h.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	rec, outcome, err := b.AskApplicationAs("scope", filepath.Join(t.TempDir(), "opencode.exe"), "12345678901234567890",
		asks.ApplicationCommandOnceKey, map[string]string{"program": "OpenCode", "operation": "Open ComfyUI",
			"operationmore": "", "command": "open", "code": "12345678901234567890",
			"until": time.Now().Add(time.Minute).UTC().Format(time.RFC3339)}, listen.Seen{}, asks.Publisher{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	c := wire.NewPresenterClient(listen.FrameClient{Endpoint: endpoint, Timeout: time.Second, MaxFrame: MaxFrameBytes})
	check := func(result wire.Lease, err error, want wire.LeaseOutcome) {
		t.Helper()
		if err != nil || result.Outcome != want {
			t.Fatalf("got %v, error %v, want %v", result.Outcome, err, want)
		}
	}
	deny.Store(true)
	refused, err := c.Claim("")
	check(refused, err, wire.LeaseOutcomeForbidden)
	if refused.ID != "" || refused.Token != "" {
		t.Fatal("forbidden claim disclosed its occurrence")
	}
	deny.Store(false)
	claim, err := c.Claim("")
	check(claim, err, wire.LeaseOutcomeHeld)
	if claim.ID != rec.ID || len(claim.Token) != 96 || claim.ExpiresMs <= time.Now().UnixMilli() {
		t.Fatal("claim did not bind the pending occurrence")
	}
	busy, err := c.Claim("")
	check(busy, err, wire.LeaseOutcomeBusy)
	child := exec.Command(os.Args[0], "-test.run=^TestPresenterNativeLeaseChild$")
	child.Env = append(os.Environ(), "OA_TEST_PRESENTER_ENDPOINT="+endpoint, "OA_TEST_PRESENTER_TOKEN="+claim.Token)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("other process: %v: %s", err, output)
	}
	otherSession.Store(true)
	result, err := c.Release(claim.Token, true)
	check(result, err, wire.LeaseOutcomeStale)
	otherSession.Store(false)
	deny.Store(true)
	result, err = c.Renew(claim.Token)
	check(result, err, wire.LeaseOutcomeForbidden)
	deny.Store(false)
	result, err = c.Renew(claim.Token)
	check(result, err, wire.LeaseOutcomeHeld)
	result, err = c.Release(claim.Token, true)
	check(result, err, wire.LeaseOutcomeReleased)
	result, err = c.Claim("")
	check(result, err, wire.LeaseOutcomeIdle)
}

func TestPresenterNativeLeaseChild(t *testing.T) {
	endpoint := os.Getenv("OA_TEST_PRESENTER_ENDPOINT")
	if endpoint == "" {
		return
	}
	c := wire.NewPresenterClient(listen.FrameClient{Endpoint: endpoint, Timeout: time.Second, MaxFrame: MaxFrameBytes})
	result, err := c.Renew(os.Getenv("OA_TEST_PRESENTER_TOKEN"))
	if err != nil || result.Outcome != wire.LeaseOutcomeStale || result.ID != "" {
		t.Fatal(result, err)
	}
}

func TestPresenterOptOutIsExplicitUnknownService(t *testing.T) {
	b, _ := book(t)
	_, _, _, endpoint := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	c := wire.NewPresenterClient(listen.FrameClient{Endpoint: endpoint, Timeout: time.Second, MaxFrame: MaxFrameBytes})
	_, err := c.Claim("")
	var service *wire.ServiceError
	if !errors.As(err, &service) || service.Code != wire.ServiceErrorCodeUnknownService {
		t.Fatalf("expected explicit unknown service, got %v", err)
	}
}
