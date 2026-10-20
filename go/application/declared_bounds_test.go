package application

import (
	"github.com/openabstractions/abstraction-identity/listen"
	"strings"
	"testing"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
)

// Published application@1 retains codec-valid out-of-bound input. Its actual
// receiver/Book still returns method invalid and admits no question.
// permission@1 has the same published boundary; published_constraints_test.go
// exercises its generated client against the actual service refusal owner.
func TestPublishedApplicationBoundsRemainMethodInvalid(t *testing.T) {
	b, _ := book(t)
	_, _, endpoint := live(t, b)
	c := wire.NewQuestionApplicationClient(listen.FrameClient{Endpoint: endpoint})
	q := question()
	q.RequestKey = strings.Repeat("k", 129)
	got, err := c.Ask(q)
	if err != nil || got.Outcome != wire.ObservationOutcomeInvalid {
		t.Fatalf("Ask: %+v %v", got, err)
	}
	for _, v := range []struct {
		key  string
		wait int64
	}{{strings.Repeat("k", 129), 0}, {"", 0}, {"k\x07", 0}, {"k", 30001}, {"k", -1}} {
		got, err = c.Observe(v.key, v.wait)
		if err != nil || got.Outcome != wire.ObservationOutcomeInvalid {
			t.Fatalf("Observe: %+v %v", got, err)
		}
	}
	if len(b.List(false)) != 0 {
		t.Fatal("invalid legacy input admitted a question")
	}
}
