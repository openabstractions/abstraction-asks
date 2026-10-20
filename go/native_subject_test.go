package asks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

func nativeBookView(t *testing.T, b *Book, version string) *NativeApplicationBook {
	t.Helper()
	subject := identity.Subject{AccountKind: "windows", Account: b.owner, Program: "msix:SubjectFixture_pzs8sxrjxfjjc"}
	view, err := b.BindNativeSubject(subject, filepath.Join(t.TempDir(), version, "app.exe"))
	if err != nil {
		t.Fatal(err)
	}
	return view
}
func nativeAsk(t *testing.T, v *NativeApplicationBook, key, host string) (Record, string) {
	t.Helper()
	r, o, e := v.AskApplicationAs(v.owner.Executable, key, "download.reach", map[string]string{"host": host}, listen.Seen{}, Publisher{})
	if e != nil {
		t.Fatal(e)
	}
	return r, o
}
func TestNativeApplicationReplayRecoveryAndCollision(t *testing.T) {
	b, _ := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	first, o := nativeAsk(t, v, "key", "example.com")
	if o != "pending" {
		t.Fatal(o)
	}
	if _, err := b.Answer(first.ID, "once"); err != nil {
		t.Fatal(err)
	}
	recovered, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatal(err)
	}
	next := nativeBookView(t, recovered, "v2")
	replay, o := nativeAsk(t, next, "key", "example.com")
	if o != "answered" || replay.ID != first.ID {
		t.Fatal(o, replay)
	}
	if _, o = nativeAsk(t, next, "key", "different.com"); o != "conflict" {
		t.Fatal(o)
	}
	old, oldOutcome, e := recovered.AskApplication(v.owner.legacy(), "old", "key", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if e != nil || oldOutcome != "pending" || old.ID == first.ID {
		t.Fatal(old, oldOutcome, e)
	}
	before, _ := os.ReadFile(b.path)
	if _, o = nativeAsk(t, v, "key", "example.com"); o != "conflict" {
		t.Fatal("distinct occurrences selected", o)
	}
	after, _ := os.ReadFile(b.path)
	if !bytes.Equal(before, after) {
		t.Fatal("collision changed persisted data")
	}
	if _, found, e := b.ApplicationRecord(first.ID); e != nil || !found {
		t.Fatal(found, e)
	}
}
func TestNativeApplicationLegacyReplayDoesNotAcquirePackageProvenance(t *testing.T) {
	b, _ := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	old, o, e := b.AskApplication(v.owner.legacy(), "legacy", "key", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if e != nil || o != "pending" {
		t.Fatal(o, e)
	}
	if _, e = b.Answer(old.ID, "once"); e != nil {
		t.Fatal(e)
	}
	replay, o := nativeAsk(t, v, "key", "example.com")
	if o != "answered" || replay.ID != old.ID {
		t.Fatal(o, replay)
	}
	f, e := b.readApplication()
	if e != nil {
		t.Fatal(e)
	}
	if len(f.Admissions) != 1 || f.Admissions[0].NativeOwner != nil || f.Admissions[0].Scope != v.owner.legacy() {
		t.Fatal("legacy row promoted", f.Admissions)
	}
	next := nativeBookView(t, b, "v2")
	fresh, o := nativeAsk(t, next, "key", "example.com")
	if o != "pending" || fresh.ID == old.ID {
		t.Fatal("unproven old package data transferred", o, fresh)
	}
}
func TestNativeApplicationConcurrentViewsUseOneBookTransaction(t *testing.T) {
	b, _ := ordinaryBook(t)
	other, e := LoadApplicationBook(b.path)
	if e != nil {
		t.Fatal(e)
	}
	other.now = b.now
	views := []*NativeApplicationBook{nativeBookView(t, b, "v1"), nativeBookView(t, other, "v2")}
	var wg sync.WaitGroup
	ids := make([]string, 2)
	errs := make([]error, 2)
	for i, v := range views {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _, err := v.AskApplicationAs(v.owner.Executable, "same", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{}, Publisher{})
			ids[i], errs[i] = r.ID, err
		}()
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || ids[0] == "" || ids[0] != ids[1] {
		t.Fatal(ids, errs)
	}
	f, e := b.readApplication()
	if e != nil || len(f.Admissions) != 1 {
		t.Fatal(f.Admissions, e)
	}
}
func TestNativeApplicationChoiceAndNotificationControl(t *testing.T) {
	b, clock := ordinaryBook(t)
	first := nativeBookView(t, b, "v1")
	next := nativeBookView(t, b, "v2")
	q := ordinaryRequest(*clock, "choice")
	chosen, e := first.AskChoiceAs(first.owner.Executable, q, nil, listen.Seen{}, Publisher{}, nil)
	if e != nil || chosen.Outcome != wire.ChoiceOutcomePending {
		t.Fatal(chosen, e)
	}
	seen, e := next.ObserveChoice(q.RequestRef)
	if e != nil || seen.ID != chosen.ID {
		t.Fatal(seen, e)
	}
	decided, e := next.DecideOwnChoiceAuthorized(q.RequestRef, "replace", next.owner.Subject.Program, false, true, nil)
	if e != nil || decided.Outcome != wire.ChoiceDecisionOutcomeAnswered {
		t.Fatal(decided, e)
	}
	notice := notificationRequest(*clock, "notice")
	posted, _, e := first.PostNotificationFrom(first.owner.Executable, notice, listen.Seen{}, Publisher{}, nil)
	if e != nil || posted.Outcome != wire.NotificationOutcomePosted {
		t.Fatal(posted, e)
	}
	observed, e := next.ObserveNotification(notice.RequestRef)
	if e != nil || observed.ID != posted.ID {
		t.Fatal(observed, e)
	}
	withdrawn, e := next.WithdrawOwnNotification(notice.RequestRef, next.owner.Subject.Program, nil)
	if e != nil || withdrawn.Outcome != wire.NotificationDecisionOutcomeWithdrawn {
		t.Fatal(withdrawn, e)
	}
}
func TestNativeApplicationProfileProvenanceValidation(t *testing.T) {
	b, _ := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	nativeAsk(t, v, "key", "example.com")
	data, e := os.ReadFile(b.path)
	if e != nil {
		t.Fatal(e)
	}
	var f book
	if e = json.Unmarshal(data, &f); e != nil {
		t.Fatal(e)
	}
	if f.Profile != ApplicationProfile {
		t.Fatal(f.Profile)
	}
	f.Profile = "asks-application@11"
	dropCatalogueContent(&f)
	old, _ := json.Marshal(f)
	if _, e = decodeApplication(old); e == nil {
		t.Fatal("legacy profile accepted native provenance")
	}
	f.Profile = ApplicationProfile
	f.Admissions[0].NativeOwner.Subject.Account = "another-account"
	bad, _ := json.Marshal(f)
	if _, e = decodeApplication(bad); e == nil {
		t.Fatal("mismatched provenance accepted")
	}
}

func TestNativeApplicationCanonicalProgramSharePreservesBounds(t *testing.T) {
	b, _ := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	nativeAsk(t, v, "first", "example.com")
	err := b.changeApplication(func(f *book) error {
		base := f.Admissions[0]
		for i := 1; i < programApplicationShare; i++ {
			a := base
			a.Key = fmt.Sprint("retained-", i)
			f.Admissions = append(f.Admissions, a)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	next := nativeBookView(t, b, "v2")
	_, _, err = next.AskApplicationAs(next.owner.Executable, "another", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{}, Publisher{})
	if !errors.Is(err, ErrApplicationCapacity) {
		t.Fatal("package version reset its program share", err)
	}
	f, err := b.readApplication()
	if err != nil {
		t.Fatal(err)
	}
	if records, admissions := f.applicationShare(programShare(v.owner.Subject.Program, v.owner.scope())); records != 1 || admissions != programApplicationShare {
		t.Fatal(records, admissions)
	}
}

func TestNativeApplicationLegacyChoiceAndNotificationControls(t *testing.T) {
	b, clock := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	q := ordinaryRequest(*clock, "old-choice")
	old, err := b.AskChoiceAs(v.owner.legacy(), v.owner.Executable, q, nil, listen.Seen{}, Publisher{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen, err := v.ObserveChoice(q.RequestRef)
	if err != nil || seen.ID != old.ID {
		t.Fatal(seen, err)
	}
	decided, err := v.DecideOwnChoiceAuthorized(q.RequestRef, "replace", v.owner.Subject.Program, false, true, nil)
	if err != nil || decided.Outcome != wire.ChoiceDecisionOutcomeAnswered {
		t.Fatal(decided, err)
	}
	ack, err := v.AcknowledgeChoice(q.RequestRef, *decided.Record.Answer, nil)
	if err != nil || ack.Outcome != wire.ChoiceAckOutcomeAcknowledged {
		t.Fatal(ack, err)
	}
	notice := notificationRequest(*clock, "old-notice")
	posted, _, err := b.PostNotificationFrom(v.owner.legacy(), v.owner.Executable, notice, listen.Seen{}, Publisher{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	withdrawn, err := v.WithdrawOwnNotification(notice.RequestRef, v.owner.Subject.Program, nil)
	if err != nil || withdrawn.Outcome != wire.NotificationDecisionOutcomeWithdrawn || withdrawn.Record.ID != posted.ID {
		t.Fatal(withdrawn, err)
	}
	f, err := b.readApplication()
	if err != nil || f.Choices[0].NativeOwner != nil || f.Notifications[0].NativeOwner != nil {
		t.Fatal("legacy control relabeled provenance", err)
	}
}

func TestNativeApplicationProvenanceCountsTowardExistingByteShare(t *testing.T) {
	b, clock := ordinaryBook(t)
	root := t.TempDir()
	program := filepath.Join(root, strings.Repeat("p", 3000))
	executable := filepath.Join(root, strings.Repeat("e", 3000))
	v, err := b.BindNativeSubject(identity.Subject{AccountKind: "windows", Account: b.owner, Program: program}, executable)
	if err != nil {
		t.Fatal(err)
	}
	admitted := 0
	for i := 0; i <= programOrdinaryShare; i++ {
		q := ordinaryRequest(*clock, fmt.Sprint("bytes-", i))
		r, e := v.AskChoiceAs(executable, q, nil, listen.Seen{}, Publisher{}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if r.Outcome == wire.ChoiceOutcomeExhausted {
			break
		}
		if r.Outcome != wire.ChoiceOutcomePending {
			t.Fatal(r)
		}
		admitted++
	}
	if admitted == 0 || admitted >= programOrdinaryShare {
		t.Fatal("native provenance escaped existing encoded byte share", admitted)
	}
	f, err := b.readApplication()
	if err != nil || len(f.Choices) != admitted {
		t.Fatal(admitted, err)
	}
	data, err := os.ReadFile(b.path)
	if err != nil || len(data) > MaxApplicationFileBytes {
		t.Fatal("file bound changed", len(data), err)
	}
}

func TestNativeApplicationRetiredOccurrenceKeepsCanonicalReplay(t *testing.T) {
	b, _ := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	first, _ := nativeAsk(t, v, "retired", "example.com")
	if _, err := b.Answer(first.ID, "once"); err != nil {
		t.Fatal(err)
	}
	if err := b.Forget(first.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadApplicationBook(b.path)
	if err != nil {
		t.Fatal(err)
	}
	next := nativeBookView(t, reopened, "v2")
	if _, outcome := nativeAsk(t, next, "retired", "example.com"); outcome != "gone" {
		t.Fatal("retired canonical identity reopened", outcome)
	}
	f, err := reopened.readApplication()
	if err != nil || len(f.Admissions) != 1 || f.Admissions[0].NativeOwner == nil {
		t.Fatal(f.Admissions, err)
	}
}

func TestNativeApplicationUnprovenCanonicalCollisionPreservesLegacyAccess(t *testing.T) {
	b, _ := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	old, outcome, err := b.AskApplication(v.owner.scope(), "legacy", "collision", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	if _, outcome = nativeAsk(t, v, "collision", "example.com"); outcome != "conflict" {
		t.Fatal("missing provenance became safe empty", outcome)
	}
	replay, outcome, err := b.ObserveApplication(v.owner.scope(), "collision")
	if err != nil || outcome != "pending" || replay.ID != old.ID {
		t.Fatal("legacy accepted record lost", replay, outcome, err)
	}
}

func TestNativeApplicationChoiceCollisionKeepsAuthorizationFirst(t *testing.T) {
	b, clock := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	q := ordinaryRequest(*clock, "collision")
	if _, err := v.AskChoiceAs(v.owner.Executable, q, nil, listen.Seen{}, Publisher{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AskChoiceAs(v.owner.legacy(), v.owner.Executable, q, nil, listen.Seen{}, Publisher{}, nil); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("existing admission refusal")
	_, err := v.AskChoiceAs(v.owner.Executable, q, nil, listen.Seen{}, Publisher{}, func() error { return refused })
	if !errors.Is(err, refused) {
		t.Fatal("compatibility lookup masked authorization", err)
	}
}

func TestNativeApplicationRetainedNotificationProfiles(t *testing.T) {
	for _, profile := range []string{notificationApplicationProfile, auditApplicationProfile, shareApplicationProfile, responderApplicationProfile} {
		t.Run(profile, func(t *testing.T) {
			b, clock := ordinaryBook(t)
			program := filepath.Join(t.TempDir(), "legacy.exe")
			q := notificationRequest(*clock, "retained")
			posted, _, err := b.PostNotificationFrom("retained-scope", program, q, listen.Seen{}, Publisher{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if profile != notificationApplicationProfile {
				choice, err := b.AskChoiceAs("retained-choice", program, ordinaryRequest(*clock, "answer"), nil, listen.Seen{}, Publisher{}, nil)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = b.DecideChoiceAuthorized(choice.ID, "keep", program, false, true, nil); err != nil {
					t.Fatal(err)
				}
			}
			f, err := b.readApplication()
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Notifications) != 1 || len(f.Notifiers) != 1 || profile != notificationApplicationProfile && len(f.Answers) != 1 {
				t.Fatal("fixture lacks retained notification/audit data")
			}
			f.Profile = profile
			data, err := json.Marshal(f)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(b.path, data, 0600); err != nil {
				t.Fatal(err)
			}
			reopened, err := LoadApplicationBook(b.path)
			if err != nil {
				t.Fatal("valid retained profile refused", err)
			}
			reopened.now = b.now
			replay, err := reopened.ObserveNotification("retained-scope", q.RequestRef)
			if err != nil || replay.ID != posted.ID || replay.Outcome != wire.NotificationOutcomePosted {
				t.Fatal(replay, err)
			}
			after, err := os.ReadFile(b.path)
			if err != nil || !bytes.Equal(data, after) {
				t.Fatal("read migrated retained profile bytes", err)
			}
		})
	}
}

func TestNativeApplicationNotificationOwnerLimits(t *testing.T) {
	for _, scheduled := range []bool{false, true} {
		t.Run(fmt.Sprint("scheduled-", scheduled), func(t *testing.T) {
			b, clock := ordinaryBook(t)
			v := nativeBookView(t, b, "v1")
			for i := 0; i < MaxLiveNotifications; i++ {
				q := notificationRequest(*clock, fmt.Sprint("legacy-", i))
				if _, _, err := b.PostNotificationFrom(v.owner.legacy(), v.owner.Executable, q, listen.Seen{}, Publisher{}, nil); err != nil {
					t.Fatal(err)
				}
			}
			q := notificationRequest(*clock, "native")
			if scheduled {
				q.Delivery = wire.NotificationDeliveryOperationEnd
				q.OperationID = "owned-operation"
			}
			posted, _, err := v.PostNotificationFrom(v.owner.Subject.Program, q, listen.Seen{}, Publisher{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if scheduled {
				if ok, err := b.PostScheduledNotification(posted.ID); err != nil || !ok {
					t.Fatal(ok, err)
				}
			}
			f, err := b.readApplication()
			if err != nil {
				t.Fatal(err)
			}
			live := 0
			legacyWithdrawn := 0
			for _, a := range f.Notifications {
				if a.Record.State == wire.NotificationStatePosted {
					live++
				}
				if a.Scope == v.owner.legacy() {
					if a.NativeOwner != nil {
						t.Fatal("legacy row promoted")
					}
					if a.Record.State == wire.NotificationStateWithdrawn {
						legacyWithdrawn++
					}
				}
			}
			if live != MaxLiveNotifications || legacyWithdrawn != 1 {
				t.Fatal("exact-current legacy rows escaped native live bound", live, legacyWithdrawn)
			}
		})
	}
	for _, scheduled := range []bool{false, true} {
		t.Run(fmt.Sprint("tag-scheduled-", scheduled), func(t *testing.T) {
			b, clock := ordinaryBook(t)
			v := nativeBookView(t, b, "v1")
			oldq := notificationRequest(*clock, "legacy-tag")
			oldq.Tag = "same-tag"
			old, _, err := b.PostNotificationFrom(v.owner.legacy(), v.owner.Executable, oldq, listen.Seen{}, Publisher{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			foreignq := oldq
			foreignq.RequestRef.Key = "unproven-other-path"
			foreign, _, err := b.PostNotificationFrom("unproven-other-scope", v.owner.Executable, foreignq, listen.Seen{}, Publisher{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			newq := oldq
			newq.RequestRef.Key = "native-tag"
			if scheduled {
				newq.Delivery = wire.NotificationDeliveryOperationEnd
				newq.OperationID = "owned-operation"
			}
			if _, _, err = v.PostNotificationFrom(v.owner.Subject.Program, newq, listen.Seen{}, Publisher{}, nil); err != nil {
				t.Fatal(err)
			}
			previous, found, err := b.NotificationRecord(old.ID)
			if err != nil || !found || previous.State != wire.NotificationStateWithdrawn {
				t.Fatal("tag did not replace exact legacy occurrence", previous, err)
			}
			untouched, found, err := b.NotificationRecord(foreign.ID)
			if err != nil || !found || untouched.State != wire.NotificationStatePosted {
				t.Fatal("unproven legacy scope was adopted", untouched, err)
			}
		})
	}
}

func TestNativeApplicationRetainedScheduledCountsCanonicalWithoutAuthority(t *testing.T) {
	b, clock := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	heldq := notificationRequest(*clock, "old-held")
	heldq.Delivery = wire.NotificationDeliveryOperationEnd
	heldq.OperationID = "old-operation"
	held, _, err := b.PostNotificationFrom(v.owner.legacy(), v.owner.Executable, heldq, listen.Seen{}, Publisher{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var canonical []string
	for i := 0; i < MaxLiveNotifications; i++ {
		q := notificationRequest(*clock, fmt.Sprint("canonical-", i))
		r, _, e := v.PostNotificationFrom(v.owner.Subject.Program, q, listen.Seen{}, Publisher{}, nil)
		if e != nil {
			t.Fatal(e)
		}
		canonical = append(canonical, r.ID)
	}
	before, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	if posted, err := b.PostScheduledNotification(held.ID); err != nil || posted {
		t.Fatal("old delayed effect exceeded full canonical quota", posted, err)
	}
	after, err := os.ReadFile(b.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("deferred old effect mutated stored state", err)
	}
	for _, id := range canonical {
		r, found, e := b.NotificationRecord(id)
		if e != nil || !found || r.State != wire.NotificationStatePosted {
			t.Fatal("old effect mutated canonical occurrence", id, r, e)
		}
	}
	f, err := b.readApplication()
	if err != nil {
		t.Fatal(err)
	}
	if f.Notifications[0].NativeOwner != nil || f.Notifications[0].Scope != v.owner.legacy() {
		t.Fatal("old provenance promoted")
	}
	if _, err = b.DecideNotification(canonical[0], NotificationDismiss, "", v.owner.Subject.Program, nil); err != nil {
		t.Fatal(err)
	}
	if posted, err := b.PostScheduledNotification(held.ID); err != nil || !posted {
		t.Fatal("old effect did not post after capacity returned", posted, err)
	}
}

func TestNativeApplicationRetainedScheduledTagDoesNotMutateCanonical(t *testing.T) {
	b, clock := ordinaryBook(t)
	v := nativeBookView(t, b, "v1")
	q := notificationRequest(*clock, "canonical-tag")
	q.Tag = "same-tag"
	canonical, _, err := v.PostNotificationFrom(v.owner.Subject.Program, q, listen.Seen{}, Publisher{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	oldq := notificationRequest(*clock, "old-held")
	oldq.Tag = q.Tag
	oldq.Delivery = wire.NotificationDeliveryOperationEnd
	oldq.OperationID = "old-operation"
	held, _, err := b.PostNotificationFrom(v.owner.legacy(), v.owner.Executable, oldq, listen.Seen{}, Publisher{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if posted, err := b.PostScheduledNotification(held.ID); err != nil || posted {
		t.Fatal("ambiguous old tag effect posted", posted, err)
	}
	r, found, err := b.NotificationRecord(canonical.ID)
	if err != nil || !found || r.State != wire.NotificationStatePosted {
		t.Fatal("old tag mutated canonical row", r, err)
	}
	view, err := b.ObserveNotification(v.owner.legacy(), q.RequestRef)
	if err != nil || view.Outcome != wire.NotificationOutcomeUnknown {
		t.Fatal("canonical row exposed through old scope", view, err)
	}
	decision, err := b.WithdrawOwnNotification(v.owner.legacy(), q.RequestRef, v.owner.Executable, nil)
	if err != nil || decision.Outcome != wire.NotificationDecisionOutcomeUnknown {
		t.Fatal("old scope gained canonical mutation", decision, err)
	}
	f, err := b.readApplication()
	if err != nil || f.Notifications[1].NativeOwner != nil {
		t.Fatal("old tag provenance changed", err)
	}
}

func TestNativeApplicationRetainedScheduledDoesNotInferAnotherExecutable(t *testing.T) {
	b, clock := ordinaryBook(t)
	old := nativeBookView(t, b, "v1")
	other := nativeBookView(t, b, "v2")
	q := notificationRequest(*clock, "old-held")
	q.Delivery = wire.NotificationDeliveryOperationEnd
	q.OperationID = "old-operation"
	q.Tag = "same-tag"
	held, _, err := b.PostNotificationFrom(old.owner.legacy(), old.owner.Executable, q, listen.Seen{}, Publisher{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var canonical []string
	for i := 0; i < MaxLiveNotifications; i++ {
		fresh := notificationRequest(*clock, fmt.Sprint("canonical-", i))
		if i == 0 {
			fresh.Tag = q.Tag
		}
		r, _, e := other.PostNotificationFrom(other.owner.Subject.Program, fresh, listen.Seen{}, Publisher{}, nil)
		if e != nil {
			t.Fatal(e)
		}
		canonical = append(canonical, r.ID)
	}
	if posted, err := b.PostScheduledNotification(held.ID); err != nil || !posted {
		t.Fatal("another executable was inferred into old quota", posted, err)
	}
	for _, id := range canonical {
		r, found, e := b.NotificationRecord(id)
		if e != nil || !found || r.State != wire.NotificationStatePosted {
			t.Fatal("foreign executable row changed", r, e)
		}
	}
	f, err := b.readApplication()
	if err != nil || f.Notifications[0].NativeOwner != nil {
		t.Fatal("old row promoted across executable", err)
	}
}
