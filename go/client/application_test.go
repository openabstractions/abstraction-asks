package client

import (
	"context"
	"errors"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"testing"
	"time"
)

func TestObservationRefusesInventedAnswer(t *testing.T) {
	for _, r := range []QuestionObservation{{Outcome: wire.ObservationOutcomePending}, {Outcome: wire.ObservationOutcomeUnknown, Answer: &wire.Answer{ID: "x"}}, {Outcome: wire.ObservationOutcomePending, Answer: &wire.Answer{ID: "x", Pending: true, Yes: true}}, {Outcome: wire.ObservationOutcomeAnswered, Answer: &wire.Answer{ID: "x"}}, {Outcome: wire.ObservationOutcomeAnswered, Answer: &wire.Answer{ID: "x", Option: "allow", Pending: true}}} {
		if _, e := checked(r, nil); e == nil {
			t.Fatal("accepted", r)
		}
	}
	for _, r := range []QuestionObservation{{Outcome: wire.ObservationOutcomePending, Answer: &wire.Answer{ID: "x", Pending: true}}, {Outcome: wire.ObservationOutcomeAnswered, Answer: &wire.Answer{ID: "x", Option: "never", Kept: true}}, {Outcome: wire.ObservationOutcomeGone}} {
		if _, e := checked(r, nil); e != nil {
			t.Fatal(r, e)
		}
	}
}
func TestDefaultWaitingBudgetRemainsReusable(t *testing.T) {
	c := New("unused")
	long, e := c.WithTimeout(31 * time.Second)
	if e != nil || long.transport.Timeout != 31*time.Second || c.transport.Timeout != 5*time.Second {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = c.AskContext(ctx, ApplicationQuestion{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = c.WithTimeout(0); e == nil {
		t.Fatal("unbounded timeout")
	}
}
