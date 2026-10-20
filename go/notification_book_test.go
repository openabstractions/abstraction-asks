package asks

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

func notificationRequest(now time.Time, key string) wire.NotificationRequest {
	return wire.NotificationRequest{
		RequestRef: wire.RequestRef{Key: key, ReplayUntilMs: now.Add(24 * time.Hour).UnixMilli()},
		Title:      "Download finished",
		Body:       "model.gguf is ready.",
		Actions:    []wire.NotificationAction{{ID: "load", Title: "Load now", Activation: wire.NotificationActivationApp}},
		ExpiresMs:  now.Add(time.Hour).UnixMilli(),
		Delivery:   wire.NotificationDeliveryNow,
	}
}

func notifyBook(t *testing.T) (*Book, *time.Time, string) {
	t.Helper()
	b, clock := ordinaryBook(t)
	return b, clock, filepath.Join(t.TempDir(), "poster.exe")
}

func post(t *testing.T, b *Book, scope, program string, q wire.NotificationRequest, authorize func() error) wire.NotificationObservation {
	t.Helper()
	r, _, err := b.PostNotificationFrom(scope, program, q, listen.Seen{}, Publisher{}, authorize)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// ASK-N1: shape bounds hold independently of the clock.
func TestNotificationRequestShape(t *testing.T) {
	now := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)
	ok := notificationRequest(now, "k")
	if !ValidNotificationRequest(ok) {
		t.Fatal("a plain request must be valid")
	}
	bad := map[string]func(*wire.NotificationRequest){
		"blank title":            func(q *wire.NotificationRequest) { q.Title = " " },
		"two-line title":         func(q *wire.NotificationRequest) { q.Title = "a\nb" },
		"long title":             func(q *wire.NotificationRequest) { q.Title = strings.Repeat("x", 81) },
		"long body":              func(q *wire.NotificationRequest) { q.Body = strings.Repeat("x", 241) },
		"control in body":        func(q *wire.NotificationRequest) { q.Body = "a\x07" },
		"three actions":          func(q *wire.NotificationRequest) { q.Actions = append(q.Actions, q.Actions[0], q.Actions[0]) },
		"duplicate action":       func(q *wire.NotificationRequest) { q.Actions = append(q.Actions, q.Actions[0]) },
		"long action title":      func(q *wire.NotificationRequest) { q.Actions[0].Title = strings.Repeat("x", 41) },
		"open without operation": func(q *wire.NotificationRequest) { q.Actions[0].Activation = wire.NotificationActivationOpenActivity },
		"end without operation":  func(q *wire.NotificationRequest) { q.Delivery = wire.NotificationDeliveryOperationEnd },
		"bad operation":          func(q *wire.NotificationRequest) { q.OperationID = "Op/1" },
		"expiry after replay":    func(q *wire.NotificationRequest) { q.ExpiresMs = q.RequestRef.ReplayUntilMs + 1 },
		"unknown delivery":       func(q *wire.NotificationRequest) { q.Delivery = 9 },
		"unknown activation":     func(q *wire.NotificationRequest) { q.Actions[0].Activation = 9 },
	}
	for name, change := range bad {
		q := notificationRequest(now, "k")
		q.Actions = append([]wire.NotificationAction{}, q.Actions...)
		change(&q)
		if ValidNotificationRequest(q) {
			t.Error(name, "must be refused")
		}
	}
	q := notificationRequest(now, "k")
	q.Body = "first line\nsecond line"
	q.OperationID = "op-1"
	q.Actions = append(q.Actions, wire.NotificationAction{ID: "open", Title: "Open", Activation: wire.NotificationActivationOpenActivity})
	if !ValidNotificationRequest(q) {
		t.Fatal("a body line feed and an owned operation action are valid")
	}
}

// ASK-N1, ASK-N3, ASK-N4, ASK-N5: replay, conflict, first action wins,
// exact acknowledgement and tombstone, all surviving a reload.
func TestNotificationLifecycleReplayAndAcknowledgement(t *testing.T) {
	b, clock, program := notifyBook(t)
	q := notificationRequest(*clock, "finished")
	first := post(t, b, "scope", program, q, nil)
	if first.Outcome != wire.NotificationOutcomePosted || first.ID == "" {
		t.Fatal(first)
	}
	if replay := post(t, b, "scope", program, q, nil); !reflect.DeepEqual(first, replay) {
		t.Fatal(replay)
	}
	changed := q
	changed.Title = "Something else"
	if r := post(t, b, "scope", program, changed, nil); r.Outcome != wire.NotificationOutcomeConflict {
		t.Fatal(r)
	}
	operator := filepath.Join(t.TempDir(), "panel.exe")
	if r, _ := b.DecideNotification(first.ID, NotificationAct, "missing", operator, nil); r.Outcome != wire.NotificationDecisionOutcomeInvalid {
		t.Fatal("an unoffered action must be invalid", r)
	}
	*clock = clock.Add(time.Minute)
	acted, err := b.DecideNotification(first.ID, NotificationAct, "load", operator, nil)
	if err != nil || acted.Outcome != wire.NotificationDecisionOutcomeActed || acted.Record.Result.DecidedBy != operator || acted.Record.Result.Action != "load" {
		t.Fatal(acted, err)
	}
	*clock = clock.Add(time.Minute)
	if again, _ := b.DecideNotification(first.ID, NotificationAct, "load", operator, nil); !reflect.DeepEqual(again, acted) {
		t.Fatal("the same action replays its original record", again)
	}
	if other, _ := b.DecideNotification(first.ID, NotificationDismiss, "", operator, nil); other.Outcome != wire.NotificationDecisionOutcomeConflict {
		t.Fatal(other)
	}
	reloaded, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.now = b.now
	seen, err := reloaded.ObserveNotification("scope", q.RequestRef)
	if err != nil || seen.Outcome != wire.NotificationOutcomeActed || !reflect.DeepEqual(seen.Result, acted.Record.Result) {
		t.Fatal(seen, err)
	}
	if other, _ := reloaded.ObserveNotification("other-scope", q.RequestRef); other.Outcome != wire.NotificationOutcomeUnknown {
		t.Fatal("another caller scope must not see it", other)
	}
	wrong := *seen.Result
	wrong.Action = "x"
	if ack, _ := reloaded.AcknowledgeNotification("scope", q.RequestRef, wrong, nil); ack.Outcome != wire.ChoiceAckOutcomeConflict {
		t.Fatal(ack)
	}
	for range 2 {
		if ack, err := reloaded.AcknowledgeNotification("scope", q.RequestRef, *seen.Result, nil); err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
			t.Fatal(ack, err)
		}
	}
	if gone, _ := reloaded.ObserveNotification("scope", q.RequestRef); gone.Outcome != wire.NotificationOutcomeGone {
		t.Fatal(gone)
	}
	if gone := post(t, reloaded, "scope", program, q, nil); gone.Outcome != wire.NotificationOutcomeGone {
		t.Fatal("an acknowledged reference never admits again", gone)
	}
	*clock = time.UnixMilli(q.RequestRef.ReplayUntilMs)
	if expired, _ := reloaded.ObserveNotification("scope", q.RequestRef); expired.Outcome != wire.NotificationOutcomeExpired {
		t.Fatal(expired)
	}
	if expired := post(t, reloaded, "scope", program, q, nil); expired.Outcome != wire.NotificationOutcomeExpired {
		t.Fatal("a reference past its replay deadline never admits again", expired)
	}
}

// ASK-N3, ASK-N4: an action after expiry records nothing; a backward clock
// cannot reopen it.
func TestNotificationExpiryRefusesActionsAndStaysExpired(t *testing.T) {
	b, clock, program := notifyBook(t)
	q := notificationRequest(*clock, "short")
	q.ExpiresMs = clock.Add(2 * time.Minute).UnixMilli()
	first := post(t, b, "scope", program, q, nil)
	*clock = clock.Add(3 * time.Minute)
	operator := filepath.Join(t.TempDir(), "panel.exe")
	if r, err := b.DecideNotification(first.ID, NotificationAct, "load", operator, nil); err != nil || r.Outcome != wire.NotificationDecisionOutcomeExpired || r.Record != nil {
		t.Fatal(r, err)
	}
	*clock = clock.Add(-10 * time.Minute)
	if r, _ := b.ObserveNotification("scope", q.RequestRef); r.Outcome != wire.NotificationOutcomeExpired || r.ID != "" {
		t.Fatal("a backward clock must not reopen an expired notification", r)
	}
	if r, _ := b.DecideNotification(first.ID, NotificationAct, "load", operator, nil); r.Outcome != wire.NotificationDecisionOutcomeExpired {
		t.Fatal(r)
	}
	q2 := notificationRequest(*clock, "late")
	q2.ExpiresMs = clock.Add(-time.Second).UnixMilli()
	if r := post(t, b, "scope", program, q2, nil); r.Outcome != wire.NotificationOutcomeExpired {
		t.Fatal("a request already past its expiry is expired", r)
	}
}

// ASK-N1, ASK-N6: a block suppresses and stores nothing; the hourly bound,
// the live bound and tags hold per program.
func TestNotificationSuppressionRateLiveAndTag(t *testing.T) {
	b, clock, program := notifyBook(t)
	blocked := func() error { return ErrNotificationSuppressed }
	if _, _, err := b.PostNotificationFrom("scope", program, notificationRequest(*clock, "x"), listen.Seen{}, Publisher{}, blocked); !errors.Is(err, ErrNotificationSuppressed) {
		t.Fatal("a block must refuse with suppression", err)
	}
	if rows, _, _ := b.NotificationHistory(); len(rows) != 0 {
		t.Fatal("a suppressed post stores nothing", rows)
	}
	var ids []string
	for i := range MaxLiveNotifications + 1 {
		*clock = clock.Add(time.Second)
		r := post(t, b, "scope", program, notificationRequest(*clock, "live-"+string(rune('a'+i))), nil)
		ids = append(ids, r.ID)
	}
	rows, _, _ := b.NotificationHistory()
	live := 0
	for _, r := range rows {
		if r.State == wire.NotificationStatePosted {
			live++
		}
		if r.ID == ids[0] && (r.State != wire.NotificationStateWithdrawn || r.Result.DecidedBy != program) {
			t.Fatal("the oldest live notification is withdrawn by the fifth", r)
		}
	}
	if live != MaxLiveNotifications {
		t.Fatal("live bound", live)
	}
	tagged := notificationRequest(*clock, "tag-1")
	tagged.Tag = "progress"
	first := post(t, b, "scope", program, tagged, nil)
	tagged2 := notificationRequest(*clock, "tag-2")
	tagged2.Tag = "progress"
	post(t, b, "scope", program, tagged2, nil)
	if r, _, _ := b.NotificationRecord(first.ID); r.State != wire.NotificationStateWithdrawn {
		t.Fatal("a tag replaces this program's live notification", r)
	}
	other := filepath.Join(t.TempDir(), "other.exe")
	for i := 0; i < MaxNotificationsPerHour-7; i++ {
		post(t, b, "scope", program, notificationRequest(*clock, "rate-"+string(rune('a'+i))), nil)
	}
	if r := post(t, b, "scope", program, notificationRequest(*clock, "over"), nil); r.Outcome != wire.NotificationOutcomeExhausted {
		t.Fatal("the hourly bound must exhaust", r)
	}
	if r := post(t, b, "scope-b", other, notificationRequest(*clock, "over"), nil); r.Outcome != wire.NotificationOutcomePosted {
		t.Fatal("another program keeps its own bound", r)
	}
	*clock = clock.Add(time.Hour)
	if r := post(t, b, "scope", program, notificationRequest(*clock, "next-hour"), nil); r.Outcome != wire.NotificationOutcomePosted {
		t.Fatal("the bound is per rolling hour", r)
	}
	counts, _, err := b.NotifierCounts()
	if err != nil || len(counts) != 2 {
		t.Fatal(counts, err)
	}
	total := int64(0)
	for _, d := range counts[0].Days {
		total += d.Posts
	}
	if counts[0].Program != program || total != MaxNotificationsPerHour+1 || counts[0].Asker != "poster.exe" {
		t.Fatal(counts[0])
	}
	// The hour also passed every earlier notification's expiry.
	n, changed, err := b.WithdrawProgramNotifications(program, filepath.Join(t.TempDir(), "panel.exe"), nil)
	if err != nil || n != 1 || len(changed) != 1 {
		t.Fatal("turning a program off withdraws its live notifications", n, err)
	}
	if r := post(t, b, "scope-b", other, notificationRequest(*clock, "still"), nil); r.Outcome != wire.NotificationOutcomePosted {
		t.Fatal(r)
	}
	if rows, _, _ := b.NotificationHistory(); rows[len(rows)-1].Program != other || rows[len(rows)-1].State != wire.NotificationStatePosted {
		t.Fatal("another program is unaffected", rows[len(rows)-1])
	}
}

// ASK-N1: operation_end holds a notification until the runtime posts it.
func TestNotificationScheduledUntilOperationEnds(t *testing.T) {
	b, clock, program := notifyBook(t)
	q := notificationRequest(*clock, "held")
	q.OperationID = "op-7"
	q.Delivery = wire.NotificationDeliveryOperationEnd
	q.Actions = append(q.Actions, wire.NotificationAction{ID: "open", Title: "Open", Activation: wire.NotificationActivationOpenActivity})
	held := post(t, b, "scope", program, q, nil)
	if held.Outcome != wire.NotificationOutcomeScheduled {
		t.Fatal(held)
	}
	operator := filepath.Join(t.TempDir(), "panel.exe")
	if r, _ := b.DecideNotification(held.ID, NotificationAct, "open", operator, nil); r.Outcome != wire.NotificationDecisionOutcomeInvalid {
		t.Fatal("a scheduled notification is not shown and takes no action", r)
	}
	list, err := b.ScheduledNotifications()
	if err != nil || len(list) != 1 || list[0].OperationID != "op-7" || list[0].Program != program {
		t.Fatal(list, err)
	}
	*clock = clock.Add(time.Minute)
	if ok, err := b.PostScheduledNotification(held.ID); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, _ := b.PostScheduledNotification(held.ID); ok {
		t.Fatal("posting twice changes nothing")
	}
	r, _, _ := b.NotificationRecord(held.ID)
	if r.State != wire.NotificationStatePosted || r.PostedMs != clock.UnixMilli() {
		t.Fatal(r)
	}
	if acted, _ := b.DecideNotification(held.ID, NotificationAct, "open", operator, nil); acted.Outcome != wire.NotificationDecisionOutcomeActed {
		t.Fatal(acted)
	}
}

// A notification with no actions is admitted, replays the same occurrence
// and survives a reload: a nil and an empty action list fingerprint alike.
func TestNotificationWithoutActions(t *testing.T) {
	b, clock, program := notifyBook(t)
	q := notificationRequest(*clock, "plain")
	q.Actions = nil
	first := post(t, b, "scope", program, q, nil)
	if first.Outcome != wire.NotificationOutcomePosted {
		t.Fatal(first)
	}
	q.Actions = []wire.NotificationAction{}
	if replay := post(t, b, "scope", program, q, nil); replay.ID != first.ID {
		t.Fatal("an empty action list replays the same occurrence", replay)
	}
	reloaded, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.now = b.now
	if seen, err := reloaded.ObserveNotification("scope", q.RequestRef); err != nil || seen.ID != first.ID {
		t.Fatal(seen, err)
	}
}

// The notification partition needs version 8; version 7 still reads.
func TestNotificationVersion7Reads(t *testing.T) {
	b, clock := ordinaryBook(t)
	admitChoice(t, b, ordinaryRequest(*clock, "old"))
	current, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeApplication(bytes.Replace(current, []byte(`"asks-application@9"`), []byte(`"asks-application@7"`), 1)); err != nil {
		t.Fatalf("version 7 without notifications must read: %v", err)
	}
}

// The notification partition needs version 8; earlier versions migrate.
func TestNotificationProfileVersion(t *testing.T) {
	b, clock, program := notifyBook(t)
	post(t, b, "scope", program, notificationRequest(*clock, "v"), nil)
	data, err := os.ReadFile(b.path)
	if err != nil || !bytes.Contains(data, []byte(`"profile":"asks-application@9"`)) || !bytes.Contains(data, []byte(`"notifications":[`)) {
		t.Fatalf("%s %v", data, err)
	}
	if _, err := decodeApplication(bytes.Replace(data, []byte(`"asks-application@9"`), []byte(`"asks-application@7"`), 1)); err == nil {
		t.Fatal("version 7 cannot carry notifications")
	}
	tampered := bytes.Replace(data, []byte(`"Title":"Download finished"`), []byte(`"Title":"Something else"`), 1)
	if bytes.Equal(tampered, data) {
		t.Fatal("fixture did not change the title")
	}
	if _, err := decodeApplication(tampered); err == nil {
		t.Fatal("a record whose content does not match its fingerprint is refused")
	}
}
