package application

import (
	"context"
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/client"
	identity "github.com/openabstractions/abstraction-identity"
	"os"
	"sync/atomic"
	"testing"
)

func TestApplicationCannotAdmitNativeConsentCatalogue(t *testing.T) {
	b, path := book(t)
	var admissions atomic.Int64
	_, _, endpoint, _ := liveOwnChoices(t, b, func(context.Context, *identity.Peer, string) error { admissions.Add(1); return nil }, false)
	app := client.New(endpoint)
	keys := []string{asks.TypedPermissionKey}
	for _, q := range asks.Questions {
		if asks.RuntimeOnlyKey(q.Key) {
			keys = append(keys, q.Key)
		}
	}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			result, err := app.AskContext(context.Background(), wire.ApplicationQuestion{RequestKey: "forged-consent", Key: key, Slots: map[string]string{"program": "OpenAbstractions", "title": "Keep this app", "host": "example.com"}})
			if err != nil || result.Outcome != wire.ObservationOutcomeForbidden || result.Answer != nil {
				t.Fatalf("application admitted %s: %+v %v", key, result, err)
			}
		})
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("forbidden attempts wrote Book: %v", err)
	}
	if admissions.Load() != 0 {
		t.Fatal("reserved key reached permissive application ask policy")
	}
	// The legacy application profile remains usable, with ordinary presentation.
	result, err := app.AskContext(context.Background(), question())
	if err != nil || result.Outcome != wire.ObservationOutcomePending || asks.AutomaticNativeQuestion(question().Key) || admissions.Load() != 1 {
		t.Fatalf("ordinary application route changed: %+v %v", result, err)
	}
}
