package application

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	rightswire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

// Generated clients deliberately bypass handwritten client validation. Values
// accepted by published codecs must reach the existing service refusal owner.
func TestPublishedChoiceConstraintsRemainServiceValidation(t *testing.T) {
	b, path := book(t)
	_, _, _, endpoint := liveOperator(t, b, func(context.Context, *identity.Peer) error { return nil })
	c := wire.NewChoiceApplicationClient(listen.FrameClient{Endpoint: endpoint})
	before, _ := os.ReadFile(path)
	for _, change := range []func(*wire.ChoiceRequest){
		func(q *wire.ChoiceRequest) { q.RequestRef.Key = "" },
		func(q *wire.ChoiceRequest) { q.RequestRef.Key = strings.Repeat("k", 129) },
		func(q *wire.ChoiceRequest) { q.RequestRef.Key = "bad\x07key" },
		func(q *wire.ChoiceRequest) { q.RequestRef.ReplayUntilMs = 0 },
		func(q *wire.ChoiceRequest) { q.Options[0].ID = "bad\noption" },
		func(q *wire.ChoiceRequest) { q.Options[0].ID = "" },
		func(q *wire.ChoiceRequest) { q.Options[0].ID = strings.Repeat("é", 32) + "x" },
		func(q *wire.ChoiceRequest) { q.PendingUntilMs = 0 },
	} {
		q := customChoice("published")
		change(&q)
		got, err := c.Ask(q)
		if err != nil || got.Outcome != wire.ChoiceOutcomeInvalid {
			t.Fatalf("service refusal: %+v %v", got, err)
		}
	}
	for _, wait := range []int64{-1, 30001} {
		got, err := c.Observe(customChoice("unknown").RequestRef, wait)
		if err != nil || got.Outcome != wire.ChoiceOutcomeInvalid {
			t.Fatal(got, err)
		}
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid choice changed Book")
	}
	q := customChoice(strings.Repeat("é", 64)) // 128 UTF-8 bytes.
	q.Options[0].ID = strings.Repeat("é", 32)  // 64 UTF-8 bytes.
	got, err := c.Ask(q)
	if err != nil || got.Outcome != wire.ChoiceOutcomePending {
		t.Fatal("valid boundary choice", got, err)
	}
}

func TestPublishedNotificationConstraintsRemainServiceValidation(t *testing.T) {
	b, path := book(t)
	f := liveNotifications(t, b, true)
	c := wire.NewNotificationApplicationClient(listen.FrameClient{Endpoint: f.endpoint})
	before, _ := os.ReadFile(path)
	for _, change := range []func(*wire.NotificationRequest){
		func(q *wire.NotificationRequest) { q.Actions = append(q.Actions, q.Actions[0], q.Actions[0]) },
		func(q *wire.NotificationRequest) { q.Actions[0].ID = "" },
		func(q *wire.NotificationRequest) { q.Actions[0].ID = strings.Repeat("é", 32) + "x" },
		func(q *wire.NotificationRequest) { q.Actions[0].ID = "bad\x07action" },
		func(q *wire.NotificationRequest) { q.ExpiresMs = 0 },
		func(q *wire.NotificationRequest) { q.OperationID = strings.Repeat("x", 129) },
		func(q *wire.NotificationRequest) { q.Tag = "bad\ntag" },
		func(q *wire.NotificationRequest) { q.Tag = strings.Repeat("é", 32) + "x" },
	} {
		q := liveRequest("published")
		change(&q)
		got, err := c.Post(q)
		if err != nil || got.Outcome != wire.NotificationOutcomeInvalid {
			t.Fatalf("service refusal: %+v %v", got, err)
		}
	}
	got, err := c.Observe(liveRequest("unknown").RequestRef, 30001)
	if err != nil || got.Outcome != wire.NotificationOutcomeInvalid {
		t.Fatal(got, err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid notification changed Book")
	}
	// Empty optional operation/tag and no actions are legitimate published values.
	q := liveRequest("empty-optionals")
	q.Actions = nil
	q.OperationID = ""
	q.Tag = ""
	got, err = c.Post(q)
	if err != nil || got.Outcome != wire.NotificationOutcomePosted {
		t.Fatal(got, err)
	}
	q = liveRequest(strings.Repeat("é", 64))
	q.Actions[0].ID = strings.Repeat("é", 32)
	q.Actions = append(q.Actions, wire.NotificationAction{ID: "second", Title: "Second", Activation: wire.NotificationActivationApp})
	q.Tag = strings.Repeat("é", 32)
	q.OperationID = strings.Repeat("o", 128)
	f.owned[q.OperationID] = true
	got, err = c.Post(q)
	if err != nil || got.Outcome != wire.NotificationOutcomePosted {
		t.Fatal("valid boundary notification", got, err)
	}
}

func TestPublishedPermissionConstraintsRemainServiceValidation(t *testing.T) {
	b, path := book(t)
	var calls atomic.Int64
	var validPreparation atomic.Bool
	_, _, _, _, endpoint := liveTyped(t, b, func(context.Context, *identity.Peer, wire.PermissionIntent) (PreparedPermission, error) {
		calls.Add(1)
		if !validPreparation.Load() {
			t.Error("invalid intent reached preparation")
		}
		return PreparedPermission{Outcome: wire.PermissionOutcomeUnavailable}, nil
	})
	c := wire.NewPermissionApplicationClient(listen.FrameClient{Endpoint: endpoint})
	before, _ := os.ReadFile(path)
	for _, q := range []wire.PermissionIntent{{RequestKey: ""}, {RequestKey: strings.Repeat("k", 129)}, {RequestKey: "bad\x07key"}, {RequestKey: "purpose", Purpose: strings.Repeat("p", 801)}} {
		q.Selection = &rightswire.PermissionSelection{Permission: rightswire.PermissionKey{Action: "abstraction.credentials/read", ResourceProfile: "abstraction.credentials/named@1"}, Selector: rightswire.ResourceSelector{Kind: rightswire.ResourceSelectorKindAll}}
		got, err := c.Request(q)
		if err != nil || got.Outcome != wire.PermissionOutcomeInvalid {
			t.Fatal(got, err)
		}
	}
	for _, q := range []struct {
		key  string
		wait int64
	}{{"", 0}, {strings.Repeat("k", 129), 0}, {"bad\x07key", 0}, {"valid", -1}, {"valid", 30001}} {
		got, err := c.Observe(q.key, q.wait)
		if err != nil || got.Outcome != wire.PermissionOutcomeInvalid {
			t.Fatal(got, err)
		}
	}
	after, _ := os.ReadFile(path)
	if calls.Load() != 0 || !bytes.Equal(before, after) {
		t.Fatal("invalid permission had an effect")
	}
	validPreparation.Store(true)
	for _, q := range []wire.PermissionIntent{{RequestKey: "k"}, {RequestKey: strings.Repeat("é", 64), Purpose: strings.Repeat("é", 400)}} {
		q.Selection = &rightswire.PermissionSelection{Permission: rightswire.PermissionKey{Action: "abstraction.credentials/read", ResourceProfile: "abstraction.credentials/named@1"}, Selector: rightswire.ResourceSelector{Kind: rightswire.ResourceSelectorKindAll}}
		got, err := c.Request(q)
		if err != nil || got.Outcome != wire.PermissionOutcomeUnavailable {
			t.Fatal("valid boundary did not reach preparation", got, err)
		}
	}
	for _, wait := range []int64{0, 30000} {
		got, err := c.Observe(strings.Repeat("é", 64), wait)
		if err != nil || got.Outcome != wire.PermissionOutcomeUnknown {
			t.Fatal("valid unknown boundary key", got, err)
		}
	}
	after, _ = os.ReadFile(path)
	if calls.Load() != 2 || !bytes.Equal(before, after) {
		t.Fatal("unavailable preparation changed Book", calls.Load())
	}
}
