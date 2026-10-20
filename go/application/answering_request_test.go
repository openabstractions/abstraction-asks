package application

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	"github.com/openabstractions/abstraction-asks/go/client"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

// ASK-R10: RequestAnswering forwards the bound peer to the host's request;
// a refused Answer or AnswerChoice asks the same way, after the Book edit;
// without a configured request the method reads forbidden.
func TestRefusedAnswersAndTheRequestAskForTheEnrollment(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Program process/path evidence is not implemented on macOS")
	}
	b, _ := book(t)
	q, _, err := b.AskApplication("runtime-only", "runtime", "first", asks.FirstUseKey,
		map[string]string{"program": os.Args[0], "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := listen.Endpoint(fmt.Sprintf("asks-answering-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int64
	if err := h.EnableResponder(func(context.Context, *identity.Peer, asks.Record, string, string) (ResponderEffect, error) {
		return nil, ErrResponderForbidden
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.EnableChoiceResponder(func(context.Context, *identity.Peer, asks.ChoiceCandidate) (ChoiceResponderEffect, error) {
		return nil, ErrResponderForbidden
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.EnableAnsweringRequest(func(_ context.Context, peer *identity.Peer) rwire.Decision {
		if peer == nil {
			t.Error("no bound peer")
		}
		requests.Add(1)
		return rwire.Decision{Outcome: rwire.DecisionOutcomeNotGranted}
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Serve(context.Background()) }()
	t.Cleanup(func() {
		h.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("host did not drain")
		}
	})
	ctx := context.Background()
	responder := client.NewResponder(endpoint)
	if d, err := responder.RequestAnsweringContext(ctx); err != nil || d.Outcome != rwire.DecisionOutcomeNotGranted || requests.Load() != 1 {
		t.Fatal(d, err, requests.Load())
	}
	if d, err := responder.AnswerContext(ctx, q.ID, "allow"); err != nil || d.Outcome.String() != "forbidden" || requests.Load() != 2 {
		t.Fatal("a refused Answer did not ask for the enrollment", d, err, requests.Load())
	}
	shared, err := client.NewChoice(endpoint).ShareContext(ctx, outputChoice("enrol"), []string{"keep"})
	if err != nil {
		t.Fatal(err)
	}
	if d, err := responder.AnswerChoiceContext(ctx, shared.ID, "keep"); err != nil || d.Outcome.String() != "forbidden" || requests.Load() != 3 {
		t.Fatal("a refused AnswerChoice did not ask for the enrollment", d, err, requests.Load())
	}
	// A choice or option the app did not share asks nothing.
	if d, err := responder.AnswerChoiceContext(ctx, shared.ID, "replace"); err != nil || d.Outcome.String() != "forbidden" || requests.Load() != 3 {
		t.Fatal(d, err, requests.Load())
	}
}

func TestRequestAnsweringWithoutAHostRequestIsForbidden(t *testing.T) {
	b, _ := book(t)
	endpoint := sharedHost(t, b, nil)
	if d, err := client.NewResponder(endpoint).RequestAnsweringContext(context.Background()); err != nil || d.Outcome != rwire.DecisionOutcomeForbidden {
		t.Fatal(d, err)
	}
}
