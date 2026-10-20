package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestListAllRefusals(t *testing.T) {
	page := func(records []int, next string, complete bool) Page[int] {
		return Page[int]{Outcome: OperatorPageOutcomePage, Records: records, Next: next, Complete: complete}
	}
	listErr := errors.New("transport down")
	cases := []struct {
		name    string
		serve   func(calls int, cursor string) (Page[int], error)
		records []int
		refusal string
		err     error
		calls   int
	}{
		{name: "gap restarts and discards the aborted attempt", serve: func(calls int, cursor string) (Page[int], error) {
			switch {
			case calls == 2:
				return Page[int]{Outcome: OperatorPageOutcomeGap}, nil
			case cursor == "":
				return page([]int{1}, "a", false), nil
			default:
				return page([]int{2}, "", true), nil
			}
		}, records: []int{1, 2}, calls: 4},
		{name: "gap on both attempts", serve: func(int, string) (Page[int], error) {
			return Page[int]{Outcome: OperatorPageOutcomeGap}, nil
		}, refusal: "gap", calls: 2},
		{name: "repeated cursor", serve: func(calls int, _ string) (Page[int], error) {
			return page([]int{calls}, "same", false), nil
		}, refusal: "stalled", calls: 2},
		{name: "incomplete page without next", serve: func(int, string) (Page[int], error) {
			return page([]int{1}, "", false), nil
		}, refusal: "stalled", calls: 1},
		{name: "page budget runs out", serve: func(calls int, _ string) (Page[int], error) {
			return page([]int{calls}, string(rune('a'+calls%26))+string(rune('a'+calls/26)), false), nil
		}, refusal: "incomplete", calls: 64},
		{name: "service outcome passes through", serve: func(int, string) (Page[int], error) {
			return Page[int]{Outcome: OperatorPageOutcomeUnavailable}, nil
		}, refusal: "unavailable", calls: 1},
		{name: "list error is returned unchanged", serve: func(int, string) (Page[int], error) {
			return Page[int]{}, listErr
		}, err: listErr, calls: 1},
	}
	for _, c := range cases {
		calls := 0
		records, err := listAll(context.Background(), func(_ context.Context, cursor string, _ int64) (Page[int], error) {
			calls++
			return c.serve(calls, cursor)
		})
		var refusal *PageRefusal
		switch {
		case calls != c.calls:
			t.Errorf("%s: calls=%d, want %d", c.name, calls, c.calls)
		case c.err != nil && err != c.err:
			t.Errorf("%s: err=%v, want %v", c.name, err, c.err)
		case c.refusal != "" && (!errors.As(err, &refusal) || refusal.Outcome != c.refusal || records != nil):
			t.Errorf("%s: records=%v err=%v, want refusal %s", c.name, records, err, c.refusal)
		case c.refusal == "" && c.err == nil && (err != nil || len(records) != len(c.records)):
			t.Errorf("%s: records=%v err=%v, want %v", c.name, records, err, c.records)
		}
	}
}

// A notification server whose Next repeats the cursor is refused as stalled
// on the second call through the real NotificationOperator. The earlier
// hand-written reader made 128 calls and reported "kept changing".
func TestListAllNotificationsRefusesRepeatedCursor(t *testing.T) {
	book, err := asks.LoadApplicationBook(filepath.Join(t.TempDir(), "questions.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	request := wire.NotificationRequest{
		RequestRef: wire.RequestRef{Key: "stall", ReplayUntilMs: now.Add(24 * time.Hour).UnixMilli()},
		Title:      "Download finished",
		ExpiresMs:  now.Add(time.Hour).UnixMilli(),
		Delivery:   wire.NotificationDeliveryNow,
	}
	program := filepath.Join(t.TempDir(), "poster.exe")
	if _, _, err := book.PostNotificationFrom("scope", program, request, listen.Seen{}, asks.Publisher{}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	history, _, err := book.NotificationHistory()
	if err != nil || len(history) != 1 {
		t.Fatalf("history=%v %v", history, err)
	}
	record := history[0]
	record.Presentation = "shown"
	encoded := wire.EncodeNotificationPage(&wire.NotificationPage{Outcome: wire.OperatorPageOutcomePage, Records: []wire.NotificationRecord{record}, Next: "same"})
	calls := 0
	transport := listen.FrameClient{Endpoint: "explicit-fixture", Dialer: func(context.Context, string) (net.Conn, error) {
		calls++
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
			reply, err := json.Marshal(map[string]any{"version": 1, "service": request.Service, "method": request.Method, "ok": true, "payload": map[string]json.RawMessage{"value": encoded}})
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
	records, err := NewNotificationOperatorWithTransport(transport).ListAllNotificationsContext(context.Background())
	var refusal *PageRefusal
	if records != nil || !errors.As(err, &refusal) || refusal.Outcome != "stalled" || calls != 2 {
		t.Fatalf("records=%v err=%v calls=%d", records, err, calls)
	}
}
