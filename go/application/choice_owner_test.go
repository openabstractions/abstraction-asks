package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	cas "github.com/openabstractions/abstraction-cas/go"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

func liveOwnChoices(t *testing.T, b *asks.Book, policy AskPolicy, trace bool) (*Host, *client.ChoiceClient, string, <-chan struct{}) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("current Program proof limitation")
	}
	endpoint := listen.Endpoint(fmt.Sprintf("asks-own-choice-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	if policy != nil {
		if err := h.EnableAskPolicy(policy); err != nil {
			t.Fatal(err)
		}
	}
	var entered chan struct{}
	if trace {
		entered = make(chan struct{}, 8)
		h.listener = &eofListener{Listener: h.listener, entered: entered}
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
			t.Error("own choice host did not drain")
		}
	})
	return h, client.NewChoice(endpoint), endpoint, entered
}

func TestOrdinaryCallerOwnAnswerWithoutOperatorAndRevokedAdmission(t *testing.T) {
	b, path := book(t)
	var revoked atomic.Bool
	var admissions atomic.Int64
	policy := func(_ context.Context, _ *identity.Peer, key string) error {
		admissions.Add(1)
		if key != OrdinaryChoicePolicyKey {
			t.Errorf("unexpected admission key %q", key)
		}
		if revoked.Load() {
			return ErrOperatorForbidden
		}
		return nil
	}
	h, c, endpoint, _ := liveOwnChoices(t, b, policy, false)
	ctx := context.Background()
	q, closeQ := customChoice("own"), customChoice("close")
	pending, err := c.AskContext(ctx, q)
	if err != nil || pending.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(pending, err)
	}
	if pending, err := c.AskContext(ctx, closeQ); err != nil || pending.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(pending, err)
	}
	if denied, err := client.NewOperator(endpoint).AnswerChoiceContext(ctx, pending.ID, "keep"); err != nil || denied.Outcome != wire.ChoiceDecisionOutcomeForbidden {
		t.Fatal("operator authority unexpectedly admitted", denied, err)
	}
	revoked.Store(true)
	if denied, err := c.AskContext(ctx, customChoice("new")); err != nil || denied.Outcome != wire.ChoiceOutcomeForbidden {
		t.Fatal("revoked admission accepted", denied, err)
	}
	calls := admissions.Load()
	first, err := c.AnswerContext(ctx, q.RequestRef, "keep")
	exe, exeErr := os.Executable()
	if err != nil || exeErr != nil || first.Outcome != wire.ChoiceDecisionOutcomeAnswered || first.Record.Answer.AnsweredBy != identity.CanonicalProgramPath(filepath.Clean(exe)) {
		t.Fatal(first, err, exeErr)
	}
	if stale, err := c.AnswerContext(ctx, q.RequestRef, "keep"); err != nil || stale.Outcome != wire.ChoiceDecisionOutcomeConflict || stale.Record != nil {
		t.Fatal(stale, err)
	}
	if replay, err := c.ReconcileAnswerContext(ctx, q.RequestRef, "keep"); err != nil || !reflect.DeepEqual(replay, first) {
		t.Fatal(replay, err)
	}
	closed, err := c.DismissContext(ctx, closeQ.RequestRef)
	if err != nil || closed.Outcome != wire.ChoiceDecisionOutcomeDismissed || closed.Record.Answer.Option != "" {
		t.Fatal(closed, err)
	}
	if replay, err := c.DismissContext(ctx, closeQ.RequestRef); err != nil || !reflect.DeepEqual(replay, closed) {
		t.Fatal(replay, err)
	}
	if admissions.Load() != calls {
		t.Fatal("own decisions consulted admission policy")
	}
	h.Close()
	restarted, err := asks.LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	_, c, _, _ = liveOwnChoices(t, restarted, policy, false)
	if replay, err := c.ReconcileAnswerContext(ctx, q.RequestRef, "keep"); err != nil || !reflect.DeepEqual(replay, first) {
		t.Fatal("restart changed exact answer", replay, err)
	}
	if observed, err := c.ObserveContext(ctx, q.RequestRef, 0); err != nil || !reflect.DeepEqual(observed.Answer, first.Record.Answer) {
		t.Fatal(observed, err)
	}
	for range 2 {
		if ack, err := c.AcknowledgeContext(ctx, q.RequestRef, *first.Record.Answer); err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
			t.Fatal(ack, err)
		}
	}
	if gone, err := c.ReconcileAnswerContext(ctx, q.RequestRef, "keep"); err != nil || gone.Outcome != wire.ChoiceDecisionOutcomeGone || gone.Record != nil {
		t.Fatal(gone, err)
	}
	if admissions.Load() != calls {
		t.Fatal("own recovery or cleanup consulted admission policy")
	}
}

func TestOrdinaryCallerOtherProgramReferenceAndIDDiscloseNothing(t *testing.T) {
	b, _ := book(t)
	_, c, endpoint, _ := liveOwnChoices(t, b, nil, false)
	q := customChoice("shared-reference")
	pending, err := c.AskContext(context.Background(), q)
	if err != nil || pending.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(pending, err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	otherProgram := filepath.Join(t.TempDir(), "other-requester.exe")
	if err := os.WriteFile(otherProgram, data, 0700); err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(otherProgram, "-test.run=^TestOrdinaryOwnCallerProcess$")
	cmd.Env = append(os.Environ(), "OA_OWN_CHOICE_ENDPOINT="+endpoint, "OA_OWN_CHOICE_REQUEST="+base64.StdEncoding.EncodeToString(request), "OA_OWN_CHOICE_ID="+pending.ID)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("distinct proven program: %v %s", err, output)
	}
	if original, err := c.ObserveContext(context.Background(), q.RequestRef, 0); err != nil || original.Outcome != wire.ChoiceOutcomePending || original.ID != pending.ID {
		t.Fatal("another program changed the original", original, err)
	}
	if own, err := c.AnswerContext(context.Background(), q.RequestRef, "keep"); err != nil || own.Outcome != wire.ChoiceDecisionOutcomeAnswered || own.Record.ID != pending.ID {
		t.Fatal(own, err)
	}
}

func TestOrdinaryOwnCallerProcess(t *testing.T) {
	endpoint := os.Getenv("OA_OWN_CHOICE_ENDPOINT")
	if endpoint == "" {
		return
	}
	data, err := base64.StdEncoding.DecodeString(os.Getenv("OA_OWN_CHOICE_REQUEST"))
	var q wire.ChoiceRequest
	if err != nil || json.Unmarshal(data, &q) != nil {
		t.Fatal("invalid child fixture", err)
	}
	c := client.NewChoice(endpoint)
	ctx := context.Background()
	for _, submit := range []func() (wire.ChoiceDecision, error){
		func() (wire.ChoiceDecision, error) { return c.AnswerContext(ctx, q.RequestRef, "keep") },
		func() (wire.ChoiceDecision, error) { return c.ReconcileAnswerContext(ctx, q.RequestRef, "keep") },
		func() (wire.ChoiceDecision, error) { return c.DismissContext(ctx, q.RequestRef) },
	} {
		if result, err := submit(); err != nil || result.Outcome != wire.ChoiceDecisionOutcomeUnknown || result.Record != nil {
			t.Fatal("foreign reference disclosed or changed a record", result, err)
		}
	}
	id := os.Getenv("OA_OWN_CHOICE_ID")
	for _, key := range []string{id, "absent-reference"} {
		ref := q.RequestRef
		ref.Key = key
		if result, err := c.AnswerContext(ctx, ref, "keep"); err != nil || result.Outcome != wire.ChoiceDecisionOutcomeUnknown || result.Record != nil {
			t.Fatal("ID or absence disclosed a record", result, err)
		}
	}
	if denied, err := client.NewOperator(endpoint).AnswerChoiceContext(ctx, id, "keep"); err != nil || denied.Outcome != wire.ChoiceDecisionOutcomeForbidden || denied.Record != nil {
		t.Fatal("ordinary ownership gained operator authority", denied, err)
	}
	own, err := c.AskContext(ctx, q)
	if err != nil || own.Outcome != wire.ChoiceOutcomePending || own.ID == id {
		t.Fatal("same full ref reused another program's occurrence", own, err)
	}
	if answered, err := c.AnswerContext(ctx, q.RequestRef, "keep"); err != nil || answered.Outcome != wire.ChoiceDecisionOutcomeAnswered || answered.Record.ID != own.ID {
		t.Fatal(answered, err)
	}
}

func TestOrdinaryCallerCanceledAfterEditWaitDoesNotDecide(t *testing.T) {
	for _, dismiss := range []bool{false, true} {
		t.Run(fmt.Sprint(dismiss), func(t *testing.T) {
			b, path := book(t)
			h, c, _, entered := liveOwnChoices(t, b, nil, true)
			q := customChoice("cancel-after-lock")
			if pending, err := c.AskContext(context.Background(), q); err != nil || pending.Outcome != wire.ChoiceOutcomePending {
				t.Fatal(pending, err)
			}
			<-entered
			locked, release := make(chan struct{}), make(chan struct{})
			var once atomic.Bool
			unlock := func() {
				if once.CompareAndSwap(false, true) {
					close(release)
				}
			}
			defer unlock()
			lockDone := make(chan error, 1)
			go func() {
				lockDone <- cas.ChangeLimit(path, asks.MaxApplicationFileBytes, func(data []byte) ([]byte, error) { close(locked); <-release; return data, nil })
			}()
			<-locked
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				if dismiss {
					_, err = c.DismissContext(ctx, q.RequestRef)
				} else {
					_, err = c.AnswerContext(ctx, q.RequestRef, "keep")
				}
				done <- err
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("request did not reach service before edit wait")
			}
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			// EOF cancellation propagates independently while the edit lock is held.
			time.Sleep(30 * time.Millisecond)
			unlock()
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
			if pending, err := c.ObserveContext(context.Background(), q.RequestRef, 0); err != nil || pending.Outcome != wire.ChoiceOutcomePending {
				t.Fatal("cancelled decision changed occurrence", pending, err)
			}
		})
	}
}
