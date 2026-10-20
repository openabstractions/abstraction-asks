package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

func permissionDetailsFixture() wire.TypedPermissionDetails {
	effect := rwire.PermissionEffect{Subject: rwire.Subject{Account: "account", Program: filepath.Join(os.TempDir(), "typed-client.exe")}, Selections: []rwire.PermissionSelection{{Permission: rwire.PermissionKey{Action: "abstraction.credentials/read", ResourceProfile: "named-value@1"}, Selector: rwire.ResourceSelector{Kind: rwire.ResourceSelectorKindOne, Target: []byte("canonical")}}}}
	return wire.TypedPermissionDetails{Question: wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: wire.OperatorQuestionOutcomeFound, Record: &wire.RecordMetadata{ID: "question", Asker: "app", Key: asks.TypedPermissionKey, About: effect.Subject.Program, Text: "OA permission request", Options: []string{"allow", "refuse"}, Asked: time.Now().UTC().Format(time.RFC3339Nano)}}}, Effect: &effect, EffectDigest: asks.PermissionEffectDigest(effect), Origin: "application", Display: &wire.PermissionDisplay{Selections: []wire.PermissionDisplaySelection{{Operation: "Read secret", Target: "named secret", Disclosure: "Returns the raw value"}}}}
}

type permissionReader struct{ value wire.TypedPermissionDetails }

func (r permissionReader) ReadPermissionDetailsContext(context.Context, string) (wire.TypedPermissionDetails, error) {
	return r.value, nil
}

func TestTypedPermissionDetailsRejectMalformedAuthority(t *testing.T) {
	for name, mutate := range map[string]func(*wire.TypedPermissionDetails){
		"foreign account": func(v *wire.TypedPermissionDetails) { v.Effect.Subject.Account = "foreign" },
		"foreign ID":      func(v *wire.TypedPermissionDetails) { v.Question.Question.Record.ID = "other" },
		"extra option": func(v *wire.TypedPermissionDetails) {
			v.Question.Question.Record.Options = append(v.Question.Question.Record.Options, "never")
		},
		"changed options": func(v *wire.TypedPermissionDetails) { v.Question.Question.Record.Options = []string{"refuse", "allow"} },
		"wrong text":      func(v *wire.TypedPermissionDetails) { v.Question.Question.Record.Text = "app-selected authority" },
		"inconsistent yes": func(v *wire.TypedPermissionDetails) {
			v.Question.Question.Record.Option = "refuse"
			v.Question.Question.Record.Answered = time.Now().UTC().Format(time.RFC3339Nano)
			v.Question.Question.Record.Yes = true
		},
		"kept": func(v *wire.TypedPermissionDetails) { v.Question.Question.Record.Kept = true },
		"pending answerer": func(v *wire.TypedPermissionDetails) {
			answerer := wire.AnswererOperator
			v.Question.Question.AnsweredBy = &answerer
		},
		"corrupt digest": func(v *wire.TypedPermissionDetails) { v.Effect.Selections[0].Selector.Target[0] ^= 1 },
		"unknown outcome": func(v *wire.TypedPermissionDetails) {
			*v = wire.TypedPermissionDetails{Question: wire.AttributedQuestion{Question: wire.OperatorQuestion{Outcome: 99}}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			v := permissionDetailsFixture()
			mutate(&v)
			if _, err := ReadPermissionDetailsContext(context.Background(), permissionReader{v}, "account", "question"); err == nil {
				t.Fatal("malformed typed permission accepted")
			}
		})
	}
	v := permissionDetailsFixture()
	out, err := ReadPermissionDetailsContext(context.Background(), permissionReader{v}, "account", "question")
	if err != nil {
		t.Fatal(err)
	}
	v.Effect.Selections[0].Selector.Target[0] ^= 1
	v.Display.Selections[0].Disclosure = "changed"
	if string(out.Effect.Selections[0].Selector.Target) != "canonical" || out.Display.Selections[0].Disclosure != "Returns the raw value" {
		t.Fatal("details retained caller alias")
	}
}

func TestTypedPermissionObservationNeverInfersAnswer(t *testing.T) {
	for _, v := range []wire.PermissionObservation{
		{Outcome: wire.PermissionOutcomeUnavailable, Answer: &wire.Answer{ID: "question", Option: "allow", Yes: true}},
		{Outcome: wire.PermissionOutcomePermitted},
		{Outcome: wire.PermissionOutcomeAnswered, Answer: &wire.Answer{ID: "question", Option: "refuse", Yes: true}},
		{Outcome: 99},
	} {
		if _, err := checkedPermissionObservation(v, nil); err == nil {
			t.Fatal("malformed observation accepted")
		}
	}
}
