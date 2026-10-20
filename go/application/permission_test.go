package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
	rightsclient "github.com/openabstractions/abstraction-rights/go/client"
)

func liveTyped(t *testing.T, b *asks.Book, prepare PreparePermission) (*Host, *client.PermissionClient, *client.PermissionOperator, *client.Operator, string) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("native Program fixture uses non-XPC test transport")
	}
	endpoint := listen.Endpoint(fmt.Sprintf("asks-typed-%d-%d", os.Getpid(), serial.Add(1)))
	h, err := Listen(endpoint, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.EnablePermission(prepare); err != nil {
		t.Fatal(err)
	}
	if err = h.EnableOperator(func(context.Context, *identity.Peer) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err = h.EnableAskPolicy(func(context.Context, *identity.Peer, string) error { return errors.New("ordinary ask refused") }); err != nil {
		t.Fatal(err)
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
			t.Error("typed host did not drain")
		}
	})
	return h, client.NewPermission(endpoint), client.NewPermissionOperator(endpoint), client.NewOperator(endpoint), endpoint
}

func prepareFixture(ctx context.Context, peer *identity.Peer, q wire.PermissionIntent) (PreparedPermission, error) {
	subject, err := rightsclient.SubjectFromPeer(peer)
	if err != nil {
		return PreparedPermission{}, err
	}
	p := asks.TypedPermission{Effect: rwire.PermissionEffect{Subject: subject}, Origin: "application", Purpose: q.Purpose}
	var selector rwire.ResourceSelector
	if q.Selection != nil {
		selector = q.Selection.Selector
		p.Effect.Selections = []rwire.PermissionSelection{*q.Selection}
	} else {
		selector = *q.BundleSelector
		p.Effect.Selections = []rwire.PermissionSelection{{Permission: rwire.PermissionKey{Action: "abstraction.credentials/read", ResourceProfile: "abstraction.credentials/named@1"}, Selector: selector}, {Permission: rwire.PermissionKey{Action: "abstraction.credentials/rotate", ResourceProfile: "abstraction.credentials/named@1"}, Selector: selector}}
		p.Effect.Bundle = &rwire.BundleExpansion{BundleRef: q.BundleRef, DefinitionDigest: "sha256:" + strings.Repeat("a", 64), Selector: selector, Selections: append([]rwire.PermissionSelection(nil), p.Effect.Selections...)}
		p.Display.BundleLabel = "Frozen two-operation bundle"
		p.Display.BundleDisclosure = "Read and rotate only"
	}
	for _, s := range p.Effect.Selections {
		if s.Permission.ResourceProfile != "abstraction.credentials/named@1" {
			return PreparedPermission{Outcome: wire.PermissionOutcomeInvalid}, nil
		}
		p.Display.Selections = append(p.Display.Selections, wire.PermissionDisplaySelection{Operation: s.Permission.Action, Target: "selected saved secret", Collection: "saved application credentials", AllEffect: "present and future saved application credentials", Disclosure: "Sensitive secret operation"})
	}
	p.Digest = asks.PermissionEffectDigest(p.Effect)
	return PreparedPermission{Permission: p, Outcome: wire.PermissionOutcomePending, Revision: "fixture-revision"}, nil
}

func TestNativeTypedPermissionRequestOwnsNoOrdinaryOrOperatorAuthority(t *testing.T) {
	b, _ := book(t)
	_, app, op, legacy, endpoint := liveTyped(t, b, prepareFixture)
	ctx := context.Background()
	for _, mode := range []struct {
		name     string
		selector rwire.ResourceSelector
		bundle   bool
	}{{"one", rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindOne, Target: []byte("{\"name\":\"fixture\"}\n")}, false}, {"all", rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindAll}, false}, {"bundle", rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindAll}, true}} {
		t.Run(mode.name, func(t *testing.T) {
			q := wire.PermissionIntent{RequestKey: mode.name, Purpose: "app purpose"}
			if mode.bundle {
				q.BundleRef = "abstraction.credentials/fixture-bundle@1"
				q.BundleSelector = &mode.selector
			} else {
				q.Selection = &rwire.PermissionSelection{Permission: rwire.PermissionKey{Action: "abstraction.credentials/read", ResourceProfile: "abstraction.credentials/named@1"}, Selector: mode.selector}
			}
			admitted, err := app.RequestContext(ctx, q)
			if err != nil || admitted.Outcome != wire.PermissionOutcomePending {
				t.Fatal(admitted, err)
			}
			details, err := op.ReadPermissionDetailsContext(ctx, admitted.Answer.ID)
			if err != nil || details.Effect == nil || details.Display == nil || details.Effect.Subject.Program == "" {
				t.Fatal(details, err)
			}
			askerOld, err := legacy.ReadQuestionAskerContext(ctx, admitted.Answer.ID)
			old := askerOld.Details
			if err != nil || old.Question.Question.Outcome != wire.OperatorQuestionOutcomeInvalid {
				t.Fatal("legacy details leaked typed authority", old, err)
			}
			decision, err := legacy.AnswerQuestionIfPendingContext(ctx, admitted.Answer.ID, "allow")
			if err != nil || decision.Outcome != wire.OperatorDecisionOutcomeUnavailable {
				t.Fatal("legacy answer accepted typed entry", decision, err)
			}
			decision, err = op.AnswerPermissionIfPendingContext(ctx, admitted.Answer.ID, "refuse")
			if err != nil || decision.Outcome != wire.OperatorDecisionOutcomeAnswered {
				t.Fatal(decision, err)
			}
			observed, err := app.ObserveContext(ctx, q.RequestKey, 0)
			if err != nil || observed.Outcome != wire.PermissionOutcomeAnswered || observed.Answer.Option != "refuse" {
				t.Fatal(observed, err)
			}
			replay, err := app.RequestContext(ctx, q)
			if err != nil || replay.Answer == nil || replay.Answer.ID != admitted.Answer.ID {
				t.Fatal(replay, err)
			}
			q.Purpose = "changed"
			changed, err := app.RequestContext(ctx, q)
			if err != nil || changed.Outcome != wire.PermissionOutcomeConflict {
				t.Fatal("changed purpose reused key", changed, err)
			}
		})
	}
	page, err := legacy.ListQuestionsContext(ctx, "", 64)
	if err != nil || len(page.Records) != 0 {
		t.Fatal("legacy list leaked typed entries", page, err)
	}
	page, err = op.ListPermissionsContext(ctx, "", 64)
	if err != nil || len(page.Records) != 3 {
		t.Fatal(page, err)
	}
	// A new permission path remains available while the existing ordinary gate
	// refuses both old catalogue admission and ordinary app choices.
	oldApp := client.New(endpoint)
	if q, err := oldApp.AskContext(ctx, question()); err != nil || q.Outcome != wire.ObservationOutcomeForbidden {
		t.Fatal(q, err)
	}
}

func TestNativeTypedPermissionBlocksOutageAndForeignPreparationAdmitNothing(t *testing.T) {
	for _, mode := range []string{"denied", "unavailable", "foreign"} {
		t.Run(mode, func(t *testing.T) {
			b, _ := book(t)
			_, app, _, _, _ := liveTyped(t, b, func(ctx context.Context, p *identity.Peer, q wire.PermissionIntent) (PreparedPermission, error) {
				result, err := prepareFixture(ctx, p, q)
				switch mode {
				case "denied":
					result.Outcome = wire.PermissionOutcomeDenied
				case "unavailable":
					result.Outcome = wire.PermissionOutcomeUnavailable
				case "foreign":
					result.Permission.Effect.Subject.Account = "other"
					result.Permission.Digest = asks.PermissionEffectDigest(result.Permission.Effect)
				}
				return result, err
			})
			q := wire.PermissionIntent{RequestKey: "refused", Selection: &rwire.PermissionSelection{Permission: rwire.PermissionKey{Action: "abstraction.credentials/read", ResourceProfile: "abstraction.credentials/named@1"}, Selector: rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindAll}}}
			result, err := app.RequestContext(context.Background(), q)
			if err != nil || result.Outcome == wire.PermissionOutcomePending {
				t.Fatal(result, err)
			}
			records, _, err := b.ApplicationHistory()
			if err != nil || len(records) != 0 {
				t.Fatal("refusal admitted an occurrence", records, err)
			}
		})
	}
}
