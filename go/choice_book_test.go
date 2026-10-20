package asks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

func ordinaryBook(t *testing.T) (*Book, *time.Time) {
	t.Helper()
	b, err := LoadApplicationBook(filepath.Join(t.TempDir(), "book.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	return b, &now
}
func ordinaryRequest(now time.Time, key string) wire.ChoiceRequest {
	return wire.ChoiceRequest{RequestRef: wire.RequestRef{Key: key, ReplayUntilMs: now.Add(time.Hour).UnixMilli()}, Title: "Replace this existing output for this delivery?", Options: []wire.ChoiceOption{{ID: "replace", Label: "Replace"}, {ID: "keep", Label: "Keep"}, {ID: "cancel", Label: "Cancel delivery"}}, PendingUntilMs: now.Add(time.Minute).UnixMilli()}
}
func admitChoice(t *testing.T, b *Book, q wire.ChoiceRequest) wire.ChoiceObservation {
	t.Helper()
	r, err := b.AskChoiceFrom("app-scope", "test app", q, listen.Seen{}, Publisher{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func choiceActorPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

func TestOrdinaryImmutableReplayAndExactAcknowledgement(t *testing.T) {
	b, clock := ordinaryBook(t)
	q := ordinaryRequest(*clock, "delivery")
	first := admitChoice(t, b, q)
	if first.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(first)
	}
	if replay := admitChoice(t, b, q); !reflect.DeepEqual(first, replay) {
		t.Fatal(replay)
	}
	changed := q
	changed.Title = "Changed intent"
	if replay := admitChoice(t, b, changed); replay.Outcome != wire.ChoiceOutcomeConflict {
		t.Fatal(replay)
	}
	other := q
	other.RequestRef.ReplayUntilMs++
	if r := admitChoice(t, b, other); r.Outcome != wire.ChoiceOutcomePending || r.ID == first.ID {
		t.Fatal(r)
	}
	actor := choiceActorPath(t, "operator.exe")
	answer, err := b.DecideChoiceAuthorized(first.ID, "keep", actor, false, true, nil)
	if err != nil || answer.Outcome != wire.ChoiceDecisionOutcomeAnswered {
		t.Fatal(answer, err)
	}
	*clock = clock.Add(time.Second)
	if replay, err := b.DecideChoiceAuthorized(first.ID, "keep", choiceActorPath(t, "another.exe"), false, false, nil); err != nil || !reflect.DeepEqual(replay, answer) {
		t.Fatal(replay, err)
	}
	if raced, err := b.DecideChoiceAuthorized(first.ID, "keep", actor, false, true, nil); err != nil || raced.Outcome != wire.ChoiceDecisionOutcomeConflict {
		t.Fatal(raced, err)
	}
	if changed, err := b.DecideChoiceAuthorized(first.ID, "replace", actor, false, false, nil); err != nil || changed.Outcome != wire.ChoiceDecisionOutcomeConflict {
		t.Fatal(changed, err)
	}
	if closed, err := b.DecideChoiceAuthorized(first.ID, "", actor, true, false, nil); err != nil || closed.Outcome != wire.ChoiceDecisionOutcomeConflict {
		t.Fatal(closed, err)
	}
	// Reopening the Book models a lost answer reply and service restart.
	restarted, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatal(err)
	}
	restarted.now = b.now
	observed, err := restarted.ObserveChoice("app-scope", q.RequestRef)
	if err != nil || observed.Outcome != wire.ChoiceOutcomeAnswered || !reflect.DeepEqual(observed.Answer, answer.Record.Answer) {
		t.Fatal(observed, err)
	}
	wrong := *observed.Answer
	wrong.AnsweredMs++
	if ack, err := restarted.AcknowledgeChoice("app-scope", q.RequestRef, wrong, nil); err != nil || ack.Outcome != wire.ChoiceAckOutcomeConflict {
		t.Fatal(ack, err)
	}
	for range 2 {
		if ack, err := restarted.AcknowledgeChoice("app-scope", q.RequestRef, *observed.Answer, nil); err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
			t.Fatal(ack, err)
		}
		restarted, err = LoadApplicationBook(b.path)
		if err != nil {
			t.Fatal(err)
		}
		restarted.now = b.now
	}
	if gone := admitChoice(t, restarted, q); gone.Outcome != wire.ChoiceOutcomeGone {
		t.Fatal(gone)
	}
	if conflict := admitChoice(t, restarted, changed); conflict.Outcome != wire.ChoiceOutcomeConflict {
		t.Fatal(conflict)
	}
	f, err := restarted.readApplication()
	if err != nil {
		t.Fatal(err)
	}
	i := f.choiceIndex("app-scope", q.RequestRef)
	if i < 0 || f.Choices[i].Record != nil || f.Choices[i].Via != nil || f.Choices[i].AnswerHash == "" {
		t.Fatal("full record survived acknowledgement", f.Choices)
	}
	*clock = time.UnixMilli(q.RequestRef.ReplayUntilMs)
	if late := admitChoice(t, restarted, q); late.Outcome != wire.ChoiceOutcomeExpired {
		t.Fatal(late)
	}
	if ack, err := restarted.AcknowledgeChoice("app-scope", q.RequestRef, *observed.Answer, nil); err != nil || ack.Outcome != wire.ChoiceAckOutcomeExpired {
		t.Fatal(ack, err)
	}
	if late, err := restarted.ObserveChoice("app-scope", q.RequestRef); err != nil || late.Outcome != wire.ChoiceOutcomeExpired {
		t.Fatal(late, err)
	}
}

func TestOrdinaryPendingExpiryAndDismissalChooseNoOption(t *testing.T) {
	b, clock := ordinaryBook(t)
	q := ordinaryRequest(*clock, "expires")
	r := admitChoice(t, b, q)
	actor := choiceActorPath(t, "panel.exe")
	*clock = time.UnixMilli(q.PendingUntilMs)
	if expired := admitChoice(t, b, q); expired.Outcome != wire.ChoiceOutcomeExpired || expired.Answer != nil {
		t.Fatal(expired)
	}
	if answer, err := b.DecideChoiceAuthorized(r.ID, "keep", actor, false, true, nil); err != nil || answer.Outcome != wire.ChoiceDecisionOutcomeExpired {
		t.Fatal(answer, err)
	}
	if close, err := b.DecideChoiceAuthorized(r.ID, "", actor, true, false, nil); err != nil || close.Outcome != wire.ChoiceDecisionOutcomeExpired {
		t.Fatal(close, err)
	}
	q = ordinaryRequest(*clock, "dismiss")
	r = admitChoice(t, b, q)
	closed, err := b.DecideChoiceAuthorized(r.ID, "", actor, true, false, nil)
	if err != nil || closed.Outcome != wire.ChoiceDecisionOutcomeDismissed || closed.Record.Answer.Option != "" {
		t.Fatal(closed, err)
	}
	if replay, err := b.DecideChoiceAuthorized(r.ID, "", choiceActorPath(t, "different.exe"), true, false, nil); err != nil || !reflect.DeepEqual(replay, closed) {
		t.Fatal(replay, err)
	}
	if answer, err := b.DecideChoiceAuthorized(r.ID, "keep", actor, false, false, nil); err != nil || answer.Outcome != wire.ChoiceDecisionOutcomeConflict {
		t.Fatal(answer, err)
	}
}

func TestOrdinaryOneOptionAndShapeBounds(t *testing.T) {
	b, clock := ordinaryBook(t)
	q := ordinaryRequest(*clock, "single")
	q.Title = strings.Repeat("界", 80)
	q.Options = []wire.ChoiceOption{{ID: strings.Repeat("é", 32), Label: strings.Repeat("界", 80)}}
	if r := admitChoice(t, b, q); r.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(r)
	}
	for name, mutate := range map[string]func(*wire.ChoiceRequest){
		"empty": func(q *wire.ChoiceRequest) { q.Options = nil },
		"four": func(q *wire.ChoiceRequest) {
			q.Options = append(q.Options, wire.ChoiceOption{ID: "four", Label: "Four"})
		},
		"duplicate":           func(q *wire.ChoiceRequest) { q.Options[1].ID = q.Options[0].ID },
		"id bytes":            func(q *wire.ChoiceRequest) { q.Options[0].ID = strings.Repeat("é", 33) },
		"title chars":         func(q *wire.ChoiceRequest) { q.Title = strings.Repeat("界", 81) },
		"label chars":         func(q *wire.ChoiceRequest) { q.Options[0].Label = strings.Repeat("界", 81) },
		"control":             func(q *wire.ChoiceRequest) { q.Title = "line\nbreak" },
		"line separator":      func(q *wire.ChoiceRequest) { q.Title = "line\u2028break" },
		"paragraph separator": func(q *wire.ChoiceRequest) { q.Options[0].Label = "paragraph\u2029break" },
		"blank":               func(q *wire.ChoiceRequest) { q.Options[0].Label = " " },
		"deadline":            func(q *wire.ChoiceRequest) { q.PendingUntilMs = q.RequestRef.ReplayUntilMs + 1 },
		"replay bound": func(q *wire.ChoiceRequest) {
			q.RequestRef.ReplayUntilMs = clock.UnixMilli() + MaxChoiceReplayMilliseconds + 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := ordinaryRequest(*clock, name)
			mutate(&bad)
			if r := admitChoice(t, b, bad); r.Outcome != wire.ChoiceOutcomeInvalid {
				t.Fatal(r)
			}
		})
	}
}

func TestOrdinaryQuotaCollectionAndLegacyPartition(t *testing.T) {
	b, clock := ordinaryBook(t)
	legacy, _, err := b.AskApplication("runtime", "runtime", "unapplied", FirstUseKey, map[string]string{"program": filepath.Join(t.TempDir(), "app.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = b.AnswerApplication(legacy.ID, "allow"); err != nil {
		t.Fatal(err)
	}
	q := ordinaryRequest(*clock, "seed")
	seed := admitChoice(t, b, q)
	// Fill both independent count partitions using the same canonical provider
	// edit. Public admissions below must report exhausted without corrupting it.
	b.mu.Lock()
	err = b.changeApplication(func(f *book) error {
		base := f.Choices[0]
		for i := 1; i < MaxOrdinaryAdmissions; i++ {
			a := base
			a.ID = fmt.Sprintf("choice-%d", i)
			a.Reference.Key = a.ID
			r := cloneChoice(*base.Record)
			r.ID, r.Request.RequestRef = a.ID, a.Reference
			a.Record, a.Fingerprint = &r, choiceFingerprint(r.Request)
			f.Choices = append(f.Choices, a)
		}
		for len(f.Admissions) < MaxApplicationAdmissions {
			i := len(f.Admissions)
			f.Admissions = append(f.Admissions, applicationAdmission{Scope: "scope", Key: fmt.Sprint(i), Fingerprint: strings.Repeat("0", 64), ID: fmt.Sprint(i)})
		}
		return nil
	})
	b.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if result := admitChoice(t, b, ordinaryRequest(*clock, "overflow")); result.Outcome != wire.ChoiceOutcomeExhausted {
		t.Fatal(result)
	}
	*clock = time.UnixMilli(q.RequestRef.ReplayUntilMs)
	restarted, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatal(err)
	}
	restarted.now = b.now
	if result := admitChoice(t, restarted, ordinaryRequest(*clock, "new")); result.Outcome != wire.ChoiceOutcomePending {
		t.Fatal("legacy full starved ordinary", result)
	}
	if late := admitChoice(t, restarted, q); late.Outcome != wire.ChoiceOutcomeExpired || late.ID == seed.ID {
		t.Fatal(late)
	}
	record, found, err := restarted.ApplicationRecord(legacy.ID)
	if err != nil || !found || record.Option != "allow" {
		t.Fatal("unapplied permission was reclaimed", record, found, err)
	}
	f, err := restarted.readApplication()
	if err != nil || len(f.Admissions) != MaxApplicationAdmissions || len(f.Choices) != 1 {
		t.Fatal(len(f.Admissions), len(f.Choices), err)
	}
}

func TestOrdinaryByteQuotaRejectsBoundedValidRequests(t *testing.T) {
	b, clock := ordinaryBook(t)
	for i := 0; i <= MaxOrdinaryAdmissions; i++ {
		q := ordinaryRequest(*clock, fmt.Sprint(i))
		q.Title = strings.Repeat("💡", 80)
		for n := range q.Options {
			q.Options[n].Label = strings.Repeat("💡", 80)
		}
		r := admitChoice(t, b, q)
		if r.Outcome == wire.ChoiceOutcomeExhausted {
			if i == MaxOrdinaryAdmissions {
				t.Fatal("byte budget never exhausted before count budget")
			}
			if _, err := LoadApplicationBook(b.path); err != nil {
				t.Fatal("capacity refusal corrupted state", err)
			}
			return
		}
		if r.Outcome != wire.ChoiceOutcomePending {
			t.Fatal(r)
		}
	}
	t.Fatal("ordinary capacity was unbounded")
}

func TestOrdinaryProfileMigrationAndStructuredRefusal(t *testing.T) {
	for _, profile := range []string{previousApplicationProfile, answererApplicationProfile, structuredApplicationProfile} {
		t.Run(profile, func(t *testing.T) {
			b, clock := ordinaryBook(t)
			data, err := json.Marshal(book{Profile: profile})
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(b.path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if r := admitChoice(t, b, ordinaryRequest(*clock, "migration")); r.Outcome != wire.ChoiceOutcomePending {
				t.Fatal(r)
			}
			data, err = os.ReadFile(b.path)
			f, err := decodeApplication(data)
			if err != nil || f.Profile != ApplicationProfile || len(f.Choices) != 1 {
				t.Fatal(f, err)
			}
			f.Profile = structuredApplicationProfile
			data, _ = json.Marshal(f)
			if _, err := decodeApplication(data); err == nil {
				t.Fatal("old profile accepted ordinary records")
			}
		})
	}
	for _, profile := range []string{structuredApplicationProfile, ApplicationProfile} {
		f := book{Profile: profile, Asks: []*Record{{ID: "bad", Asker: "runtime", Key: FirstUseKey, Asked: time.Now()}}}
		data, _ := json.Marshal(f)
		if _, err := decodeApplication(data); err == nil {
			t.Fatal("missing structured target accepted", profile)
		}
	}
}

func TestOrdinaryExpiryCannotReopenAfterRollbackAndRestart(t *testing.T) {
	for _, observe := range []string{"pending", "replay", "unadmitted"} {
		t.Run(observe, func(t *testing.T) {
			b, clock := ordinaryBook(t)
			originalTime := *clock
			q := ordinaryRequest(*clock, "rollback")
			if observe != "unadmitted" {
				admitChoice(t, b, q)
			}
			deadline := q.PendingUntilMs
			if observe != "pending" {
				deadline = q.RequestRef.ReplayUntilMs
			}
			*clock = time.UnixMilli(deadline)
			r, err := b.ObserveChoice("app-scope", q.RequestRef)
			if err != nil || r.Outcome != wire.ChoiceOutcomeExpired {
				t.Fatal(r, err)
			}
			data, err := os.ReadFile(b.path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := decodeApplication(data)
			if err != nil || f.ChoiceTimeMs < deadline {
				t.Fatal("expiry time was not durable", f.ChoiceTimeMs, err)
			}
			if observe == "replay" && len(f.Choices) != 0 {
				t.Fatal("replay-expired occurrence retained")
			}
			*clock = originalTime
			restarted, err := LoadApplicationBook(b.path)
			if err != nil {
				t.Fatal(err)
			}
			restarted.now = b.now
			if r := admitChoice(t, restarted, q); r.Outcome != wire.ChoiceOutcomeExpired {
				t.Fatal("rollback reopened old ref", r)
			}
			if r, err := restarted.ObserveChoice("app-scope", q.RequestRef); err != nil || r.Outcome != wire.ChoiceOutcomeExpired {
				t.Fatal(r, err)
			}
		})
	}
}

func TestOrdinaryUnexpiredReadsDoNotAdvancePersistedTime(t *testing.T) {
	b, clock := ordinaryBook(t)
	q := ordinaryRequest(*clock, "read-only")
	admitChoice(t, b, q)
	before, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	*clock = clock.Add(20 * time.Second)
	for range 3 {
		if r, err := b.ObserveChoice("app-scope", q.RequestRef); err != nil || r.Outcome != wire.ChoiceOutcomePending {
			t.Fatal(r, err)
		}
		if _, _, err := b.ChoiceHistory(); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(b.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("unexpired reads wrote clock state", err)
	}
}

func TestOrdinaryExpiryStorageFailureDoesNotReportSuccess(t *testing.T) {
	b, clock := ordinaryBook(t)
	q := ordinaryRequest(*clock, "failed-expiry")
	admitChoice(t, b, q)
	path, failedPath := b.path, t.TempDir()
	*clock = time.UnixMilli(q.PendingUntilMs)
	// Inject a storage refusal after the read, at the expiry transition's edit.
	reads := 0
	b.now = func() time.Time {
		reads++
		if reads == 2 {
			b.path = failedPath
		}
		return *clock
	}
	if r, err := b.ObserveChoice("app-scope", q.RequestRef); err == nil || r.Outcome == wire.ChoiceOutcomeExpired {
		t.Fatal("uncommitted expiry reported success", r, err)
	}
	b.path = path
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := decodeApplication(data)
	if err != nil || f.ChoiceTimeMs >= q.PendingUntilMs || f.Choices[0].Record.State != wire.ChoiceStatePending {
		t.Fatal("failed expiry changed durable state", f, err)
	}
}

func TestOrdinaryClockBytesDoNotChargeFullLegacyPartition(t *testing.T) {
	b, clock := ordinaryBook(t)
	r, _, err := b.AskApplication("runtime", "runtime", "legacy", FirstUseKey, map[string]string{"program": filepath.Join(t.TempDir(), "app.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = b.AnswerApplication(r.ID, "allow"); err != nil {
		t.Fatal(err)
	}
	f, err := b.readApplication()
	if err != nil {
		t.Fatal(err)
	}
	// Version 3 carries no answer audit (ASK-O15).
	f.Profile, f.ChoiceTimeMs, f.Answers = structuredApplicationProfile, 0, nil
	data, _ := json.Marshal(f)
	f.Asks[0].Text += strings.Repeat("x", MaxApplicationBytes-len(data))
	data, _ = json.Marshal(f)
	if len(data) != MaxApplicationBytes {
		t.Fatal(len(data))
	}
	if _, err = decodeApplication(data); err != nil {
		t.Fatal("legacy boundary fixture is invalid", err)
	}
	if err = os.WriteFile(b.path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if r := admitChoice(t, b, ordinaryRequest(*clock, "independent")); r.Outcome != wire.ChoiceOutcomePending {
		t.Fatal("legacy byte capacity starved ordinary", r)
	}
	if _, err := LoadApplicationBook(b.path); err != nil {
		t.Fatal(err)
	}
	f, err = b.readApplication()
	if err != nil || f.ChoiceTimeMs <= 0 {
		t.Fatal(f.ChoiceTimeMs, err)
	}
	for _, invalidTime := range []int64{-1, 0, clock.UnixMilli() - 1} {
		f.ChoiceTimeMs = invalidTime
		data, _ = json.Marshal(f)
		if _, err := decodeApplication(data); err == nil {
			t.Fatal("invalid persisted ordinary time accepted", invalidTime)
		}
	}
}

func TestOrdinaryClockRequirementPreservesLegacyPreEpochEdits(t *testing.T) {
	b, clock := ordinaryBook(t)
	*clock = time.Date(1960, 1, 2, 3, 4, 5, 0, time.UTC)
	data, _ := json.Marshal(book{Profile: structuredApplicationProfile})
	if err := os.WriteFile(b.path, data, 0600); err != nil {
		t.Fatal(err)
	}
	r, outcome, err := b.AskApplication("runtime", "runtime", "legacy-pre-epoch", FirstUseKey, map[string]string{"program": filepath.Join(t.TempDir(), "app.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil || outcome != "pending" || !r.Asked.Equal(*clock) {
		t.Fatal("ordinary clock changed legacy admission", r, outcome, err)
	}
	if r, outcome, err := b.AnswerApplicationIfPendingAuthorized(r.ID, "allow", nil); err != nil || outcome != "answered" || r.Option != "allow" || r.Answered.IsZero() {
		t.Fatal("ordinary clock changed legacy answer", r, outcome, err)
	}
	q := ordinaryRequest(time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), "unavailable-clock")
	if r, err := b.AskChoiceFrom("app-scope", "app", q, listen.Seen{}, Publisher{}, nil); err == nil || r.Outcome == wire.ChoiceOutcomePending {
		t.Fatal("ordinary operation accepted unusable clock", r, err)
	}
	f, err := b.readApplication()
	if err != nil || f.ChoiceTimeMs != 0 || len(f.Choices) != 0 {
		t.Fatal("legacy edit created ordinary state", f.ChoiceTimeMs, err)
	}
}
