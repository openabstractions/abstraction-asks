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
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

type notificationFixture struct {
	host     *Host
	endpoint string
	poster   *client.NotificationClient
	operator *client.NotificationOperator
	blocked  *atomic.Bool
	owned    map[string]bool
}

func liveNotifications(t *testing.T, b *asks.Book, enable bool) *notificationFixture {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("Program process/path evidence is not implemented on macOS")
	}
	endpoint := listen.Endpoint(fmt.Sprintf("asks-notification-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	f := &notificationFixture{host: h, endpoint: endpoint, blocked: &atomic.Bool{}, owned: map[string]bool{"op-1": true}}
	if err = h.EnableOperator(func(context.Context, *identity.Peer) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if enable {
		policy := func(context.Context, *identity.Peer) error {
			if f.blocked.Load() {
				return asks.ErrNotificationSuppressed
			}
			return nil
		}
		blocked := func(context.Context, string) error {
			if f.blocked.Load() {
				return asks.ErrNotificationSuppressed
			}
			return nil
		}
		owner := func(_ context.Context, _ *identity.Peer, id string) error {
			if f.owned[id] {
				return nil
			}
			return ErrOperationNotOwned
		}
		if err = h.EnableNotifications(policy, blocked, owner); err != nil {
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
			t.Error("notification host did not drain")
		}
	})
	f.poster, f.operator = client.NewNotification(endpoint), client.NewNotificationOperator(endpoint)
	return f
}

func liveRequest(key string) wire.NotificationRequest {
	now := time.Now()
	return wire.NotificationRequest{
		RequestRef: wire.RequestRef{Key: key, ReplayUntilMs: now.Add(time.Hour).UnixMilli()},
		Title:      "Download finished",
		Actions:    []wire.NotificationAction{{ID: "load", Title: "Load now", Activation: wire.NotificationActivationApp}},
		ExpiresMs:  now.Add(10 * time.Minute).UnixMilli(),
		Delivery:   wire.NotificationDeliveryNow,
	}
}

// ASK-N1, ASK-N4, ASK-N5: a native poster's notification is acted on by an
// operator with its presentation token, the poster observes and acknowledges,
// and a token from before a restart is stale.
func TestNotificationActionReachesPosterAfterRestart(t *testing.T) {
	b, path := book(t)
	f := liveNotifications(t, b, true)
	ctx := context.Background()
	q := liveRequest("finished")
	posted, err := f.poster.PostContext(ctx, q)
	if err != nil || posted.Outcome != wire.NotificationOutcomePosted {
		t.Fatal(posted, err)
	}
	records, err := f.operator.ListAllNotificationsContext(ctx)
	if err != nil || len(records) != 1 || records[0].ID != posted.ID || records[0].Program == "" || records[0].Presentation == "" {
		t.Fatal(records, err)
	}
	shown := records[0]
	waited := make(chan wire.NotificationObservation, 1)
	go func() {
		waiting, _ := f.poster.WithTimeout(5 * time.Second)
		r, _ := waiting.ObserveContext(ctx, q.RequestRef, 3000)
		waited <- r
	}()
	time.Sleep(30 * time.Millisecond)
	if r, err := f.operator.ActContext(ctx, shown.ID, "load", "not-the-token"); err != nil || r.Outcome != wire.NotificationDecisionOutcomeStale {
		t.Fatal(r, err)
	}
	acted, err := f.operator.ActContext(ctx, shown.ID, "load", shown.Presentation)
	if err != nil || acted.Outcome != wire.NotificationDecisionOutcomeActed || acted.Record.Result.DecidedBy == "" {
		t.Fatal(acted, err)
	}
	select {
	case r := <-waited:
		if r.Outcome != wire.NotificationOutcomeActed || r.Result.Action != "load" {
			t.Fatal(r)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the action did not wake the observing poster")
	}

	// A second posted notification survives a runtime restart; its old token is stale.
	q2 := liveRequest("second")
	second, err := f.poster.PostContext(ctx, q2)
	if err != nil || second.Outcome != wire.NotificationOutcomePosted {
		t.Fatal(second, err)
	}
	read, err := f.operator.ReadNotificationContext(ctx, second.ID)
	if err != nil || read.Record == nil {
		t.Fatal(read, err)
	}
	f.host.Close()
	restarted, err := asks.LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	g := liveNotifications(t, restarted, true)
	if r, err := g.operator.ActContext(ctx, second.ID, "load", read.Record.Presentation); err != nil || r.Outcome != wire.NotificationDecisionOutcomeStale {
		t.Fatal("a token from before the restart must be stale", r, err)
	}
	seen, err := g.poster.ObserveContext(ctx, q.RequestRef, 0)
	if err != nil || seen.Outcome != wire.NotificationOutcomeActed {
		t.Fatal(seen, err)
	}
	if ack, err := g.poster.AcknowledgeContext(ctx, q.RequestRef, *seen.Result); err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
		t.Fatal(ack, err)
	}
	if gone, err := g.poster.ObserveContext(ctx, q.RequestRef, 0); err != nil || gone.Outcome != wire.NotificationOutcomeGone {
		t.Fatal(gone, err)
	}
	withdrawn, err := g.poster.WithdrawContext(ctx, q2.RequestRef)
	if err != nil || withdrawn.Outcome != wire.NotificationOutcomeWithdrawn || withdrawn.Result == nil {
		t.Fatal(withdrawn, err)
	}
	if again, err := g.poster.WithdrawContext(ctx, q2.RequestRef); err != nil || again.Outcome != wire.NotificationOutcomeWithdrawn || again.Result.DecidedMs != withdrawn.Result.DecidedMs {
		t.Fatal("the same withdrawal replays", again, err)
	}
}

// ASK-N1, ASK-N6: a block suppresses posting and acting; no configuration
// forbids; an operation outside the caller's scope is invalid.
func TestNotificationAdmissionPolicy(t *testing.T) {
	ctx := context.Background()
	b, _ := book(t)
	off := liveNotifications(t, b, false)
	if r, err := off.poster.PostContext(ctx, liveRequest("x")); err != nil || r.Outcome != wire.NotificationOutcomeForbidden {
		t.Fatal("without a configured policy posting is forbidden", r, err)
	}
	b2, _ := book(t)
	f := liveNotifications(t, b2, true)
	posted, err := f.poster.PostContext(ctx, liveRequest("one"))
	if err != nil || posted.Outcome != wire.NotificationOutcomePosted {
		t.Fatal(posted, err)
	}
	read, _ := f.operator.ReadNotificationContext(ctx, posted.ID)
	f.blocked.Store(true)
	if r, err := f.poster.PostContext(ctx, liveRequest("two")); err != nil || r.Outcome != wire.NotificationOutcomeSuppressed {
		t.Fatal(r, err)
	}
	if r, err := f.operator.ActContext(ctx, posted.ID, "load", read.Record.Presentation); err != nil || r.Outcome != wire.NotificationDecisionOutcomeForbidden {
		t.Fatal("a blocked program's notification takes no action", r, err)
	}
	if r, err := f.operator.WithdrawProgramContext(ctx, read.Record.Program); err != nil || r.Count != 1 {
		t.Fatal(r, err)
	}
	f.blocked.Store(false)
	q := liveRequest("foreign")
	q.OperationID = "op-2"
	q.Actions = append(q.Actions, wire.NotificationAction{ID: "open", Title: "Open", Activation: wire.NotificationActivationOpenActivity})
	if r, err := f.poster.PostContext(ctx, q); err != nil || r.Outcome != wire.NotificationOutcomeInvalid {
		t.Fatal("another scope's operation is refused", r, err)
	}
	q.OperationID = "op-1"
	if r, err := f.poster.PostContext(ctx, q); err != nil || r.Outcome != wire.NotificationOutcomePosted {
		t.Fatal(r, err)
	}
	page, err := f.operator.ListNotifiersContext(ctx, "", 64)
	if err != nil || len(page.Notifiers) != 1 || len(page.Notifiers[0].Days) != 1 || page.Notifiers[0].Days[0].Posts != 2 {
		t.Fatal(page, err)
	}
}

func TestNotificationsNeedAnExplicitPolicy(t *testing.T) {
	endpoint := listen.Endpoint(fmt.Sprintf("asks-notification-cfg-%d-%d", os.Getpid(), serial.Add(1)))
	// A macOS socket endpoint cannot carry Program proof; the installed
	// runtime serves asks-application-v1 over XPC there.
	if err := listen.CanEver(endpoint, listen.Program); err != nil {
		t.Skipf("this platform's socket endpoint cannot carry Program proof: %v", err)
	}
	b, _ := book(t)
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.EnableNotifications(nil, nil, nil); err == nil {
		t.Fatal("an explicit policy is required")
	}
	h.Close()
	if err := h.EnableNotifications(func(context.Context, *identity.Peer) error { return nil }, func(context.Context, string) error { return nil }, nil); err == nil {
		t.Fatal("a closed host cannot be configured")
	}
}
