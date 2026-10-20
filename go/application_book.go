package asks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-asks/go/catalogue"
	cas "github.com/openabstractions/abstraction-cas/go"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	watch "github.com/openabstractions/abstraction-watch/go"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Version 10 keeps, with each legacy admission and each acknowledged
// choice, the fingerprint of the program it counts against (ASK-P5), so a
// retired or acknowledged occurrence stays in that program's share; a read
// of an earlier version fills it in from each retained record.
// Version 9 replaces the delegated-answer audit with the answer audit of
// every answer recorded through operator@1 or responder@1 (ASK-O15); a read
// of version 7 or 8 moves the delegated entries into it. Version 8 adds the
// notification partition (ASK-N1). Version 7 keeps an
// ordinary choice's bound requester program, the options its application
// shared with an assistant, its decision route and the delegated-answer
// audit. Version 6 keeps an application question's bound asker program.
// Version 5 retains typed permission occurrences separately from exact first
// use. Versions 1..7 migrate on a real edit; version 3's exact targets retain
// their original meaning and version 4's ordinary partition stays bounded.
// Version 11 records a typed responder's real actor. Older readers explicitly
// refuse this version; versions 1..10 retain their original attribution rules.
// Version 12 retains native canonical ownership on new ordinary admissions.
// Existing legacy rows keep their original scopes and historical provenance.
// Version 13 retains catalogue content with questions and audited answers.
// Old readers refuse the profile; old occurrences keep absent content.
const ApplicationProfile = "asks-application@13"
const nativeApplicationProfile = "asks-application@12"
const responderApplicationProfile = "asks-application@11"
const shareApplicationProfile = "asks-application@10"
const auditApplicationProfile = "asks-application@9"
const notificationApplicationProfile = "asks-application@8"
const delegatedApplicationProfile = "asks-application@7"
const askerProgramApplicationProfile = "asks-application@6"
const typedApplicationProfile = "asks-application@5"
const ordinaryApplicationProfile = "asks-application@4"
const structuredApplicationProfile = "asks-application@3"
const answererApplicationProfile = "asks-application@2"
const previousApplicationProfile = "asks-application@1"
const MaxApplicationBytes = 4 << 20
const MaxApplicationAdmissions = 4096

var ErrApplicationCapacity = errors.New("asks: application capacity reached")

// applicationAdmission is one admitted request key. ProgramHash, from version
// 10, is the fingerprint of the program its record counts against (ASK-P5),
// kept after the record is retired; "" counts against the scope.
type applicationAdmission struct {
	NativeOwner *nativeApplicationOwner `json:"native_owner,omitempty"`
	Scope       string                  `json:"scope"`
	Key         string                  `json:"key"`
	Fingerprint string                  `json:"fingerprint"`
	ID          string                  `json:"id"`
	ProgramHash string                  `json:"program_hash,omitempty"`
}

// LoadApplicationBook opens the separate application-profile file. The host owns
// this file; legacy binaries must never be configured with its path. Native
// Answer/Forget on this returned Book are explicit trusted operator integration.
func LoadApplicationBook(path string) (*Book, error) {
	owner, e := user.Current()
	if e != nil || owner.Uid == "" {
		return nil, errors.New("asks: application account unavailable")
	}
	b := &Book{path: path, application: true, owner: owner.Uid, decided: map[string]chan struct{}{}, gate: watch.NewGate(watch.DefaultWaiters)}
	_, e = b.readApplication()
	return b, e
}
func (b *Book) ApplicationProfile() bool { return b != nil && b.application }

// Retained native records share the Book's strict canonical JSON owner.
func decodeRetainedJSON(data []byte, v any) error   { return catalogue.DecodeRetainedJSON(data, v) }
func canonicalRetainedJSON(data []byte, v any) bool { return catalogue.CanonicalRetainedJSON(data, v) }

func decodeApplication(data []byte) (book, error) {
	if data == nil {
		return book{Profile: ApplicationProfile}, nil
	}
	var f book
	if e := decodeRetainedJSON(data, &f); e != nil {
		return f, e
	}
	content := f.Profile == ApplicationProfile
	nativeOwnership := content || f.Profile == nativeApplicationProfile
	if err := f.validateNativeOwners(nativeOwnership); err != nil {
		return f, err
	}
	typedResponder := nativeOwnership || f.Profile == responderApplicationProfile
	shareHashes := typedResponder || f.Profile == shareApplicationProfile
	audited := shareHashes || f.Profile == auditApplicationProfile
	delegated := audited || f.Profile == notificationApplicationProfile || f.Profile == delegatedApplicationProfile
	askerProgram := delegated || f.Profile == askerProgramApplicationProfile
	typed := askerProgram || f.Profile == typedApplicationProfile
	ordinary := typed || f.Profile == ordinaryApplicationProfile
	structured := ordinary || f.Profile == structuredApplicationProfile
	if (!structured && f.Profile != answererApplicationProfile && f.Profile != previousApplicationProfile) || len(f.Admissions) > MaxApplicationAdmissions || len(f.Asks) > MaxApplicationAdmissions || !ordinary && (len(f.Choices) != 0 || f.ChoiceTimeMs != 0) {
		return f, errors.New("asks: unsupported application state")
	}
	if !delegated && (len(f.Delegated) != 0 || f.DelegatedSinceMs != 0 || slices.ContainsFunc(f.Choices, func(a choiceAdmission) bool {
		return a.AskerProgram != "" || a.Shared != nil || a.Route != ""
	})) {
		return f, errors.New("asks: version 6 and earlier cannot carry shared choices")
	}
	if !audited && (len(f.Answers) != 0 || f.AnswersSinceMs != 0) || audited && (len(f.Delegated) != 0 || f.DelegatedSinceMs != 0) {
		return f, errors.New("asks: only versions 9 and 10 carry the answer audit")
	}
	if !shareHashes && (slices.ContainsFunc(f.Admissions, func(a applicationAdmission) bool { return a.ProgramHash != "" }) ||
		slices.ContainsFunc(f.Choices, func(a choiceAdmission) bool { return a.ProgramHash != "" })) {
		return f, errors.New("asks: only version 10 carries program fingerprints")
	}
	ids := map[string]bool{}
	keys := map[string]bool{}
	for _, r := range f.Asks {
		if r == nil || r.ID == "" || ids[r.ID] {
			return f, errors.New("asks: invalid application record")
		}
		q, ok := Find(r.Key)
		if !validRecordContent(*r) || !content && r.Content != nil {
			return f, errors.New("asks: invalid retained catalogue content")
		}
		if !ok || r.Asked.IsZero() || r.Asker == "" || !validRecordedPublisher(r) {
			return f, errors.New("asks: invalid application question")
		}
		if r.AnsweredBy != "" && r.AnsweredBy != AnsweredByOperator && r.AnsweredBy != AnsweredByResponder {
			return f, errors.New("asks: invalid answerer")
		}
		if r.Answerer != "" && (r.AnsweredBy != AnsweredByResponder || !validAnswerer(r.Answerer)) || r.TypedPermission != nil && r.AnsweredBy == AnsweredByResponder && !validAnswerer(r.Answerer) {
			return f, errors.New("asks: invalid answering program")
		}
		if f.Profile == previousApplicationProfile && r.Answerer != "" {
			return f, errors.New("asks: version 1 cannot carry answering program")
		}
		// The display name is derived from the bound program, never stored apart from it.
		if r.AskerProgram != "" && (!askerProgram || !validAskerProgram(r.AskerProgram) || r.Asker != AskerDisplay(r.AskerProgram)) {
			return f, errors.New("asks: invalid asker program")
		}
		if r.Permission != nil && (!structured || r.Key != FirstUseKey || !ValidPermissionRequest(*r.Permission)) ||
			structured && r.Key == FirstUseKey && r.Permission == nil {
			return f, errors.New("asks: invalid structured permission")
		}
		if r.TypedPermission != nil && (!typed || r.Key != TypedPermissionKey || r.Permission != nil || !ValidTypedPermission(*r.TypedPermission) || r.AnsweredBy == AnsweredByResponder && !typedResponder) || r.Key == TypedPermissionKey && r.TypedPermission == nil {
			return f, errors.New("asks: invalid typed permission")
		}
		if r.TypedPermission != nil && (r.About != r.TypedPermission.Effect.Subject.Program || r.Text != "OA permission request" ||
			!slices.Equal(r.Options, []string{"allow", "refuse"}) && (!content || !typedPermissionOnce(*r))) {
			return f, errors.New("asks: mismatched typed permission projection")
		}
		if r.Pending() {
			if r.Yes || r.Kept || !r.Answered.IsZero() || r.AnsweredBy != "" || r.Answerer != "" {
				return f, errors.New("asks: invalid pending decision")
			}
		} else {
			option, ok := recordedQuestionOption(*r, q, r.Option, r.AnsweredBy)
			if !ok || option.Yes != r.Yes || option.Kept != r.Kept || r.Answered.IsZero() {
				return f, errors.New("asks: invalid recorded decision")
			}
		}
		ids[r.ID] = true
	}
	byID := make(map[string]*Record, len(f.Asks))
	for _, r := range f.Asks {
		byID[r.ID] = r
	}
	for _, a := range f.Admissions {
		key := a.Scope + "\x00" + a.Key
		digest, decodeErr := hex.DecodeString(a.Fingerprint)
		if !ValidRequestKey(a.Scope) || !ValidRequestKey(a.Key) || decodeErr != nil || len(digest) != 32 || a.ID == "" || keys[key] {
			return f, errors.New("asks: invalid application admission")
		}
		if r, retained := byID[a.ID]; a.ProgramHash != "" && (!validChoiceHash(a.ProgramHash) || retained && a.ProgramHash != a.NativeOwner.programFingerprint(a.Scope, r)) {
			return f, errors.New("asks: invalid admission program fingerprint")
		}
		keys[key] = true
	}
	// This version owns a canonical JSON file. Refuse duplicate fields, invalid
	// Unicode and alternate spellings before they can normalize authority data.
	if !canonicalRetainedJSON(data, &f) {
		return f, errors.New("asks: noncanonical application state")
	}
	// Versions 7 and 8 audited only assistants' answers to shared choices;
	// those entries open the answer audit, oldest first (ASK-O15).
	if !audited {
		for _, d := range f.Delegated {
			f.Answers = append(f.Answers, retainedAnswer{AnswerEntry: delegatedEntry(d)})
		}
		f.AnswersSinceMs = f.DelegatedSinceMs
		f.Delegated, f.DelegatedSinceMs = nil, 0
	}
	if e := validateChoicePartition(f); e != nil {
		return f, e
	}
	if !content && slices.ContainsFunc(f.Answers, func(answer retainedAnswer) bool { return answer.Content != nil }) {
		return f, errors.New("asks: legacy profile carries catalogue content")
	}
	if e := validateNotificationPartition(f); e != nil {
		return f, e
	}
	if f.Profile == previousApplicationProfile || f.Profile == answererApplicationProfile {
		for _, r := range f.Asks {
			r.legacyPermission = r.Key == FirstUseKey
		}
	}
	return f, nil
}

func (b *Book) validatePermissionAccounts(f book) error {
	for _, a := range f.Admissions {
		if a.NativeOwner != nil && a.NativeOwner.Subject.Account != b.owner {
			return errors.New("asks: foreign native admission account")
		}
	}
	for _, a := range f.Choices {
		if a.NativeOwner != nil && a.NativeOwner.Subject.Account != b.owner {
			return errors.New("asks: foreign native choice account")
		}
	}
	for _, a := range f.Notifications {
		if a.NativeOwner != nil && a.NativeOwner.Subject.Account != b.owner {
			return errors.New("asks: foreign native notification account")
		}
	}
	for _, r := range f.Asks {
		if r.Permission != nil && r.Permission.Targets[0].Account != b.owner {
			return errors.New("asks: foreign permission account")
		}
		if r.TypedPermission != nil && r.TypedPermission.Effect.Subject.Account != b.owner {
			return errors.New("asks: foreign typed permission account")
		}
	}
	for _, answer := range f.Answers {
		if answer.Content != nil && answer.Content.Permission != nil && answer.Content.Permission.Targets[0].Account != b.owner {
			return errors.New("asks: foreign audited permission account")
		}
	}
	return nil
}

func (b *Book) readApplication() (book, error) {
	if e := b.applicationFile(); e != nil {
		return book{}, e
	}
	data, e := cas.ReadLimit(b.path, MaxApplicationFileBytes)
	if e != nil {
		return book{}, e
	}
	f, e := decodeApplication(data)
	if e == nil {
		if e = b.validatePermissionAccounts(f); e != nil {
			return book{}, e
		}
		// Expired questions are not retained (ASK-A7); the next edit writes that.
		f.expire(b.clock(), b.lifetimeOf)
		f.collectChoices(f.ChoiceTimeMs)
		f.collectNotifications(f.ChoiceTimeMs)
	}
	return f, e
}

// changeApplication applies edit to the current state with expired questions
// removed (ASK-A7). Callers hold b.mu; waiters on an expired question are
// released.
func (b *Book) changeApplication(edit func(*book) error) error {
	if e := b.applicationFile(); e != nil {
		return e
	}
	var expired []string
	var changes questionChanges
	e := cas.ChangeLimit(b.path, MaxApplicationFileBytes, func(data []byte) ([]byte, error) {
		changes = questionChanges{}
		f, e := decodeApplication(data)
		if e != nil {
			return nil, e
		}
		ids, pending := pendingQuestions(&f)
		if e = b.validatePermissionAccounts(f); e != nil {
			return nil, e
		}
		legacy := f.Profile != ApplicationProfile
		snapshot := func() ([]byte, error) {
			if !legacy {
				return nil, nil
			}
			return json.Marshal(&f)
		}
		beforeMaintenance, e := snapshot()
		if e != nil {
			return nil, e
		}
		oldChoiceTime := f.ChoiceTimeMs
		expired = f.expire(b.clock(), b.lifetimeOf)
		choiceTime := f.choiceTime(b.clock().UnixMilli())
		choiceExpired := append(f.collectChoices(choiceTime), f.collectNotifications(choiceTime)...)
		if f.ChoiceTimeMs == 0 || len(choiceExpired) > 0 {
			f.noteChoiceTime(choiceTime)
		}
		expired = append(expired, choiceExpired...)
		maintainedChoiceTime := f.ChoiceTimeMs
		f.ChoiceTimeMs = oldChoiceTime
		afterMaintenance, e := snapshot()
		f.ChoiceTimeMs = maintainedChoiceTime
		if e != nil {
			return nil, e
		}
		// Resolve known legacy first-use records before emitting the new profile.
		for _, r := range f.Asks {
			if r.legacyPermission {
				permission, ok := PermissionForRecord(b.owner, *r)
				if !ok {
					return nil, errors.New("asks: invalid legacy permission")
				}
				r.Permission, r.legacyPermission = &permission, false
			}
		}
		// An admission from before version 10 takes its record's program
		// fingerprint, so a later retirement keeps its share (ASK-P5).
		f.fillProgramHashes()
		beforeEdit, e := snapshot()
		if e != nil {
			return nil, e
		}
		if e = edit(&f); e != nil {
			return nil, e
		}
		afterEdit, e := snapshot()
		if e != nil {
			return nil, e
		}
		// A replay may inspect a legacy Book without changing it. Commit the
		// new representation only with an operation or expiry-maintenance edit.
		if legacy && bytes.Equal(beforeMaintenance, afterMaintenance) && bytes.Equal(beforeEdit, afterEdit) {
			return data, nil
		}
		changes = diffQuestions(&f, ids, pending, expired)
		f.Profile = ApplicationProfile
		if e := validateChoicePartition(f); e != nil {
			return nil, e
		}
		if e := validateNotificationPartition(f); e != nil {
			return nil, e
		}
		return json.Marshal(&f)
	})
	for _, id := range expired {
		b.settle(id)
	}
	if e == nil {
		b.emit(changes)
	}
	return e
}
func (b *Book) applicationFile() error {
	info, e := os.Lstat(b.path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return errors.New("asks: application state must be a regular file")
	}
	return nil
}
func ValidRequestKey(key string) bool {
	return len(key) > 0 && len(key) <= 128 && utf8.ValidString(key) && strings.IndexFunc(key, unicode.IsControl) < 0
}
func applicationQuestion(display, key string, slots map[string]string) (Ask, string, error) {
	a, text, err := catalogue.ApplicationQuestion(display, key, slots)
	return Ask{Asker: a.Asker, Key: a.Key, Slots: a.Slots}, text, err
}

// AskApplication receives only receiver-derived scope/display/evidence. It is a
// provider method; the application service supplies these after native binding.
// It records no publisher; AskApplicationFrom does.
func (b *Book) AskApplication(scope, display, requestKey, key string, slots map[string]string, via listen.Seen) (Record, string, error) {
	return b.AskApplicationFrom(scope, display, requestKey, key, slots, via, Publisher{})
}

// AskApplicationFrom is AskApplication recording publisher, the requester's
// code signer the admitting service verified at admission (ASK-A6), with a new
// question. A replay or a shared question keeps the evidence it was admitted with.
func (b *Book) AskApplicationFrom(scope, display, requestKey, key string, slots map[string]string, via listen.Seen, publisher Publisher) (Record, string, error) {
	return b.askApplicationFrom(nil, scope, display, "", requestKey, key, slots, via, publisher, nil)
}

// AskApplicationAs is AskApplicationFrom for the bound program the receiver
// derived scope from (ASK-A1). The new record keeps that program and shows
// AskerDisplay of it as its asker (ASK-A2). A replay or a shared question
// keeps the record it was admitted with.
func (b *Book) AskApplicationAs(scope, program, requestKey, key string, slots map[string]string, via listen.Seen, publisher Publisher) (Record, string, error) {
	if !validAskerProgram(program) {
		return Record{}, "invalid", nil
	}
	return b.askApplicationFrom(nil, scope, AskerDisplay(program), program, requestKey, key, slots, via, publisher, nil)
}

// AskerDisplay is the display name of a bound asker program: its file name.
// It is the one derivation of Record.Asker from Record.AskerProgram (ASK-A2).
// A display name confers no authority and identifies no program.
func AskerDisplay(program string) string { return filepath.Base(program) }

// validAskerProgram reports whether program is a bound program an application
// record may keep as its asker (ASK-A1): a clean absolute executable path.
func validAskerProgram(program string) bool {
	return validAnswerer(program) && !strings.HasPrefix(program, identity.PackagedProgramPrefix)
}

// AskPermissionFrom is trusted runtime integration. No application service
// request carries permission data; the runtime derives it from its own refusal.
func (b *Book) AskPermissionFrom(scope, display, requestKey string, permission PermissionRequest, via listen.Seen, publisher Publisher) (Record, string, error) {
	if !ValidPermissionRequest(permission) || permission.Targets[0].Account != b.owner {
		return Record{}, "invalid", nil
	}
	provider, pairedProvider := ProviderPermissionName(permission)
	permission, err := NewPermissionRequest(permission.Targets[0])
	if err != nil {
		return Record{}, "invalid", nil
	}
	if pairedProvider {
		companion := permission.Targets[0]
		companion.Action = "abstraction.inference/complete"
		companion.Resource = "host:" + provider
		permission.Targets = append(permission.Targets, companion)
	}
	return b.askApplicationFrom(nil, scope, display, "", requestKey, FirstUseKey, nil, via, publisher, &permission)
}

func (b *Book) askApplicationFrom(native *nativeApplicationOwner, scope, display, program, requestKey, key string, slots map[string]string, via listen.Seen, publisher Publisher, permission *PermissionRequest) (Record, string, error) {
	if key == TypedPermissionKey {
		return Record{}, "invalid", nil
	}
	if !b.application {
		return Record{}, "", errors.New("asks: application Book required")
	}
	if !ValidRequestKey(scope) || !ValidRequestKey(requestKey) {
		return Record{}, "invalid", nil
	}
	var a Ask
	var text string
	var e error
	if permission == nil {
		a, text, e = applicationQuestion(display, key, slots)
	} else {
		t := permission.Targets[0]
		a = Ask{Asker: display, Key: FirstUseKey, Slots: map[string]string{"program": t.Program}}
		// Keep the legacy metadata projection complete for older readers.
		text = t.Program + " wants to " + t.Action + " on " + t.Resource
	}
	if e != nil {
		return Record{}, "invalid", nil
	}
	var recordedPermission = permission
	if key == FirstUseKey && permission == nil {
		p, err := NewPermissionRequest(PermissionTarget{Account: b.owner, Program: a.Slots["program"], Action: a.Slots["action"], Resource: a.Slots["resource"]})
		if err != nil {
			return Record{}, "invalid", nil
		}
		recordedPermission = &p
		// The retained content names the program as its permission names it
		// (ValidCatalogueContent): the subject spelling, so an 8.3 alias in the
		// asking slots becomes the program's own name before it is stored.
		if program := p.Targets[0].Program; program != slots["program"] {
			slots = maps.Clone(slots)
			slots["program"] = program
			if a, text, e = applicationQuestion(display, key, slots); e != nil {
				return Record{}, "invalid", nil
			}
		}
	}
	canonical, _ := json.Marshal(struct {
		Key        string
		Slots      map[string]string
		Permission *PermissionRequest `json:",omitempty"`
	}{key, slots, permission})
	sum := sha256.Sum256(canonical)
	fingerprint := hex.EncodeToString(sum[:])
	b.mu.Lock()
	defer b.mu.Unlock()
	var out Record
	outcome := ""
	e = b.changeApplication(func(f *book) error {
		selected, nativeErr := f.nativeScope(native, scope, "application", requestKey)
		if nativeErr != nil {
			return nativeErr
		}
		if !ValidRequestKey(selected) {
			return errors.New("asks: invalid application scope")
		}
		scope = selected

		for _, ad := range f.Admissions {
			if ad.Scope == scope && ad.Key == requestKey {
				if ad.Fingerprint != fingerprint {
					outcome = "conflict"
					return nil
				}
				i := f.index(ad.ID)
				if i < 0 {
					outcome = "gone"
					return nil
				}
				out = *f.Asks[i]
				outcome = recordOutcome(out)
				return nil
			}
		}
		if len(f.Admissions) >= MaxApplicationAdmissions {
			return ErrApplicationCapacity
		}
		for _, ad := range f.Admissions {
			if ad.Scope != scope || ad.Fingerprint != fingerprint {
				continue
			}
			i := f.index(ad.ID)
			if i >= 0 && (f.Asks[i].Pending() || f.Asks[i].Kept) {
				out = *f.Asks[i]
				break
			}
		}
		if out.ID == "" {
			if len(f.Asks) >= MaxApplicationAdmissions {
				return ErrApplicationCapacity
			}
			q, _ := Find(key)
			publisher = publisher.recorded()
			out = Record{ID: random(16), Asker: display, AskerProgram: program, Key: key, About: a.Slots[q.About], Text: text, Options: q.OptionNames(), Asked: b.clock().UTC(), Via: via,
				Publisher: publisher.Subject, PublisherTrusted: publisher.Trusted,
				ProgramDescription: publisher.Description, ProgramOriginalName: publisher.OriginalName, Permission: recordedPermission}
			if key == ProgramReviewKey {
				out.Options = ProgramReviewOptions(slots)
			}
			out.Content = cloneCatalogueContent(&wire.CatalogueContent{Slots: slots, Permission: recordedPermission})
			if f.nativeApplicationShareFull(native, scope, &out, true) {
				return ErrApplicationCapacity // one program's share (ASK-P5)
			}
			copy := out
			f.Asks = append(f.Asks, &copy)
		} else if f.nativeApplicationShareFull(native, scope, &out, false) {
			return ErrApplicationCapacity
		}
		f.Admissions = append(f.Admissions, applicationAdmission{NativeOwner: native.recorded(scope), Scope: scope, Key: requestKey, Fingerprint: fingerprint, ID: out.ID, ProgramHash: native.programFingerprint(scope, &out)})
		outcome = recordOutcome(out)
		return nil
	})
	return out, outcome, e
}

// ApplicationRecord reads one retained question by id. found is false when no
// retained question has that id; a storage failure is an error.
func (b *Book) ApplicationRecord(id string) (Record, bool, error) {
	if !b.application {
		return Record{}, false, errors.New("asks: application Book required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	f, e := b.readApplication()
	if e != nil {
		return Record{}, false, e
	}
	if i := f.index(id); i >= 0 {
		return *f.Asks[i], true, nil
	}
	return Record{}, false, nil
}
func recordOutcome(r Record) string {
	if r.Pending() {
		return "pending"
	}
	return "answered"
}

// ReleaseAnsweredApplication removes the question scope's key names when it
// is answered with option, with every admission naming it and no tombstone,
// the way expiry removes a pending question (ASK-A7): the key then admits a
// new question. A runtime releases a first-use question whose applied allow
// no longer holds, or whose refuse is older than its interval (ASK-R2). A
// pending, dismissed or otherwise answered question stays. It reports
// whether a question was removed.
func (b *Book) ReleaseAnsweredApplication(scope, key, option string) (bool, error) {
	return b.ReleaseAnsweredApplicationID(scope, key, "", option)
}

// ReleaseAnsweredApplicationID also checks the observed occurrence under the
// Book edit lock. A replacement answer under the same key cannot be released.
func (b *Book) nativeReleaseAnsweredApplicationID(native *nativeApplicationOwner, scope, key, expectedID, option string) (bool, error) {
	if !b.application {
		return false, fmt.Errorf("asks: application Book required")
	}
	if scope == "" || !ValidRequestKey(key) || option == "" {
		return false, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	released := false
	unchanged := errors.New("asks: release without edit")
	err := b.changeApplication(func(f *book) error {
		selected, nativeErr := f.nativeScope(native, scope, "application", key)
		if nativeErr != nil {
			return nativeErr
		}
		scope = selected

		id := ""
		for _, a := range f.Admissions {
			if a.Scope == scope && a.Key == key {
				id = a.ID
				break
			}
		}
		i := f.index(id)
		if id == "" || i < 0 || expectedID != "" && id != expectedID || f.Asks[i].Pending() || f.Asks[i].Option != option {
			return unchanged
		}
		f.Asks = slices.Delete(f.Asks, i, i+1)
		f.Admissions = slices.DeleteFunc(f.Admissions, func(a applicationAdmission) bool { return a.ID == id })
		released = true
		return nil
	})
	if errors.Is(err, unchanged) {
		return false, nil
	}
	return released, err
}

// ReleaseUnsavedAllow removes a person's Allow, answered through the
// operator, whose permission its effect owner could not save, with every
// admission naming it. Only a first-use or typed permission question is
// released. The asker's next request, under the same key included, then asks
// afresh. The answer audit keeps the person's click.
//
// keep runs under the Book lock, in the Book-then-effect order of
// WithApplicationRecordContext, and reports whether the effect owner saved
// id after all (its completion receipt). A save pinned through that method
// either finished before keep reads it, or finds no record afterwards. An
// error from keep keeps the Allow. It reports whether a question was removed.
func (b *Book) ReleaseUnsavedAllow(id string, keep func() (bool, error)) (bool, error) {
	if !b.application {
		return false, fmt.Errorf("asks: application Book required")
	}
	if !ValidRequestKey(id) || keep == nil {
		return false, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	released := false
	unchanged := errors.New("asks: release without edit")
	err := b.changeApplication(func(f *book) error {
		i := f.index(id)
		if i < 0 {
			return unchanged
		}
		r := f.Asks[i]
		if r.Key != FirstUseKey && r.Key != TypedPermissionKey || r.Pending() || r.Option != "allow" || r.AnsweredBy != AnsweredByOperator {
			return unchanged
		}
		if saved, err := keep(); err != nil || saved {
			return unchanged
		}
		f.Asks = slices.Delete(f.Asks, i, i+1)
		f.Admissions = slices.DeleteFunc(f.Admissions, func(a applicationAdmission) bool { return a.ID == id })
		released = true
		return nil
	})
	if errors.Is(err, unchanged) {
		return false, nil
	}
	if err == nil {
		b.settle(id)
	}
	return released, err
}

// WithApplicationRecordContext pins a retained occurrence and its admission
// through a trusted separate-owner effect. Lock order is Book then effect;
// the effect must never reenter Book. Every retained ID is supplied for safe
// orphan-proof pruning. The read pin writes no Book content.
func (b *Book) WithApplicationRecordContext(ctx context.Context, id string, use func(Record, string, []string) error) (bool, error) {
	for !b.mu.TryLock() {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	defer b.mu.Unlock()
	if !b.application || use == nil {
		return false, errors.New("asks: application Book and trusted effect required")
	}
	if err := b.applicationFile(); err != nil {
		return false, err
	}
	found := false
	err := cas.ChangeLimitContext(ctx, b.path, MaxApplicationFileBytes, func(data []byte) ([]byte, error) {
		f, err := decodeApplication(data)
		if err != nil {
			return nil, err
		}
		if err = b.validatePermissionAccounts(f); err != nil {
			return nil, err
		}
		f.expire(b.clock(), b.lifetimeOf)
		i := f.index(id)
		if i < 0 {
			return data, nil
		}
		scope := ""
		for _, ad := range f.Admissions {
			if ad.ID == id {
				scope = ad.Scope
				break
			}
		}
		retained := make([]string, 0, len(f.Asks))
		for _, r := range f.Asks {
			retained = append(retained, r.ID)
		}
		found = true
		if err := use(*f.Asks[i], scope, retained); err != nil {
			return nil, err
		}
		return data, nil
	})
	return found, err
}

func (b *Book) nativeObserveApplication(native *nativeApplicationOwner, scope, key string) (Record, string, error) {
	if !b.application {
		return Record{}, "", fmt.Errorf("asks: application Book required")
	}
	if scope == "" || !ValidRequestKey(key) {
		return Record{}, "invalid", nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	f, e := b.readApplication()
	if e != nil {
		return Record{}, "", e
	}

	selected, nativeErr := f.nativeScope(native, scope, "application", key)
	if nativeErr != nil {
		return Record{}, "", nativeErr
	}
	scope = selected

	for _, a := range f.Admissions {
		if a.Scope == scope && a.Key == key {
			i := f.index(a.ID)
			if i < 0 {
				return Record{}, "gone", nil
			}
			r := *f.Asks[i]
			return r, recordOutcome(r), nil
		}
	}
	return Record{}, "unknown", nil
}
