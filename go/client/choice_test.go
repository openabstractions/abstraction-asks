package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

func choiceRecordFixture(t *testing.T, dismiss bool) ChoiceRecord {
	t.Helper()
	now := time.Now().UnixMilli()
	r := ChoiceRecord{ID: "choice", Asker: "fixture", Request: ChoiceRequest{RequestRef: RequestRef{Key: "request", ReplayUntilMs: now + 60000}, Title: "Keep output?", Options: []ChoiceOption{{ID: "keep", Label: "Keep"}}, PendingUntilMs: now + 30000}, State: ChoiceStateAnswered, AskedMs: now}
	r.Answer = &ChoiceAnswer{ID: r.ID, Option: "keep", AnsweredMs: now, AnsweredBy: filepath.Join(t.TempDir(), "operator.exe")}
	if dismiss {
		r.State, r.Answer.Option = ChoiceStateDismissed, ""
	}
	return r
}

func TestOwnChoiceDecisionValidatesFullReferenceAndAnswer(t *testing.T) {
	for _, dismiss := range []bool{false, true} {
		r := choiceRecordFixture(t, dismiss)
		ref, option := r.Request.RequestRef, r.Answer.Option
		outcome := ChoiceDecisionOutcomeAnswered
		if dismiss {
			outcome = ChoiceDecisionOutcomeDismissed
		}
		if _, err := checkedOwnChoiceDecision(ChoiceDecision{Outcome: outcome, Record: &r}, nil, ref, option, dismiss); err != nil {
			t.Fatal(err)
		}
		for _, mutate := range []func(*ChoiceRecord){
			func(r *ChoiceRecord) { r.Request.RequestRef.Key = "another-request" },
			func(r *ChoiceRecord) { r.Request.RequestRef.ReplayUntilMs++ },
			func(r *ChoiceRecord) { r.Answer.ID = "another-occurrence" },
			func(r *ChoiceRecord) { r.Answer.Option = "unoffered" },
			func(r *ChoiceRecord) { r.Answer.AnsweredBy = "relative-program" },
		} {
			bad := r
			answer := *r.Answer
			bad.Answer = &answer
			mutate(&bad)
			if _, err := checkedOwnChoiceDecision(ChoiceDecision{Outcome: outcome, Record: &bad}, nil, ref, option, dismiss); err == nil {
				t.Fatal("mismatched requester decision accepted", bad)
			}
		}
	}
}

func TestOwnChoiceClientOldServiceFailsVisiblyWithoutFallback(t *testing.T) {
	var calls atomic.Int64
	transport := listen.FrameClient{Endpoint: "explicit-fixture", Dialer: func(context.Context, string) (net.Conn, error) {
		calls.Add(1)
		caller, server := net.Pipe()
		go func() {
			defer server.Close()
			frame, err := listen.ReadFrameFrom(server, 0)
			if err != nil {
				t.Error(err)
				return
			}
			var request struct {
				Service string `json:"service"`
				Method  string `json:"method"`
			}
			if err := json.Unmarshal(frame, &request); err != nil {
				t.Error(err)
				return
			}
			reply, err := json.Marshal(map[string]any{"version": 1, "service": request.Service, "method": request.Method, "ok": false, "payload": map[string]string{"code": "unknown_method", "message": ""}})
			if err != nil {
				t.Error(err)
				return
			}
			if err := listen.WriteFrameTo(server, reply, 0); err != nil {
				t.Error(err)
			}
		}()
		return caller, nil
	}}
	c := NewChoiceWithTransport(transport)
	ref := RequestRef{Key: "old-service", ReplayUntilMs: time.Now().Add(time.Hour).UnixMilli()}
	for _, submit := range []func() (ChoiceDecision, error){
		func() (ChoiceDecision, error) { return c.AnswerContext(context.Background(), ref, "keep") },
		func() (ChoiceDecision, error) { return c.ReconcileAnswerContext(context.Background(), ref, "keep") },
		func() (ChoiceDecision, error) { return c.DismissContext(context.Background(), ref) },
	} {
		before := calls.Load()
		result, err := submit()
		var serviceError *wire.ServiceError
		if !errors.As(err, &serviceError) || serviceError.Code != wire.ServiceErrorCodeUnknownMethod || result.Record != nil {
			t.Fatal("old runtime response was concealed", result, err)
		}
		if calls.Load() != before+1 {
			t.Fatal("caller retried or selected a fallback")
		}
	}
}

func TestChoiceDecisionRequiresMatchingOutcomeAndState(t *testing.T) {
	for _, dismiss := range []bool{false, true} {
		r := choiceRecordFixture(t, dismiss)
		outcome := ChoiceDecisionOutcomeAnswered
		if dismiss {
			outcome = ChoiceDecisionOutcomeDismissed
		}
		if _, err := checkedChoiceDecision(ChoiceDecision{Outcome: outcome, Record: &r}, nil, r.ID, r.Answer.Option, dismiss); err != nil {
			t.Fatal("valid fixture refused", err)
		}
		wrong := ChoiceDecisionOutcomeDismissed
		if dismiss {
			wrong = ChoiceDecisionOutcomeAnswered
		}
		if _, err := checkedChoiceDecision(ChoiceDecision{Outcome: wrong, Record: &r}, nil, r.ID, r.Answer.Option, dismiss); err == nil {
			t.Fatal("outcome disagreed with record state")
		}
	}
}

func TestChoiceObservationRequiresFullBoundAnswerer(t *testing.T) {
	r := choiceRecordFixture(t, false)
	observation := ChoiceObservation{Outcome: ChoiceOutcomeAnswered, ID: r.ID, Answer: r.Answer}
	if _, err := checkedChoice(observation, nil); err != nil {
		t.Fatal("valid fixture refused", err)
	}
	for _, actor := range []string{"relative.exe", "", "bad\nactor"} {
		answer := *r.Answer
		answer.AnsweredBy = actor
		observation.Answer = &answer
		if _, err := checkedChoice(observation, nil); err == nil {
			t.Fatalf("malformed actor %q accepted", actor)
		}
	}
}
