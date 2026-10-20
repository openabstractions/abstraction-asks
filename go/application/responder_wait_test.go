package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
	legacy "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/responderwait"
	"github.com/openabstractions/abstraction-asks/go/client"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestResponderWaitOnlyListsEnrolledAnswerableLiveOccurrences(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("native Program fixture; no XPC proof")
	}
	b, _ := book(t)
	add := func(request string) string {
		t.Helper()
		record, _, err := b.AskApplication("runtime-only", "runtime", request, asks.FirstUseKey,
			map[string]string{"program": filepath.Join(t.TempDir(), "target.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
		if err != nil {
			t.Fatal(err)
		}
		return record.ID
	}
	eligible, forbidden := add("eligible"), add("forbidden")
	endpoint := listen.Endpoint(fmt.Sprintf("responder-wait-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.EnableResponder(func(_ context.Context, _ *identity.Peer, record asks.Record, _, _ string) (ResponderEffect, error) {
		if record.ID != eligible {
			return nil, ErrResponderForbidden
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	var enrolled atomic.Bool
	enrolled.Store(true)
	if err := h.EnableResponderWait(func(context.Context, *identity.Peer) (bool, error) {
		return enrolled.Load(), nil
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- h.Serve(t.Context()) }()
	t.Cleanup(func() {
		h.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	c := client.NewResponderWaitWithTransport(listen.FrameClient{Endpoint: endpoint})
	read := func(outcome wire.Outcome, ids ...string) {
		t.Helper()
		page, err := c.ListWaitingContext(t.Context(), "", 64)
		if err != nil || page.Outcome != outcome || len(page.IDs) != len(ids) ||
			len(ids) == 1 && page.IDs[0] != ids[0] {
			t.Fatalf("wait page = %+v, %v", page, err)
		}
	}
	_, releaseForbidden := b.ObserveQuestionWait(forbidden)
	defer releaseForbidden()
	read(wire.OutcomePage)
	_, release := b.ObserveQuestionWait(eligible)
	defer release()
	read(wire.OutcomePage, eligible)
	enrolled.Store(false)
	read(wire.OutcomeForbidden)
	old, err := client.NewResponder(endpoint).ListAnswerableContext(t.Context(), "", 64)
	if err != nil || old.Outcome != legacy.OperatorPageOutcomePage || len(old.Records) != 1 || old.Records[0].ID != eligible {
		t.Fatalf("old responder = %+v, %v", old, err)
	}
	enrolled.Store(true)
	if _, outcome, err := b.AnswerApplication(eligible, "allow"); err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	read(wire.OutcomePage)
}
