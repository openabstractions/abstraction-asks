package asks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	cas "github.com/openabstractions/abstraction-cas/go"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

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
const ApplicationProfile = "asks-application@9"
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

type applicationAdmission struct {
	Scope       string `json:"scope"`
	Key         string `json:"key"`
	Fingerprint string `json:"fingerprint"`
	ID          string `json:"id"`
}

// LoadApplicationBook opens the separate application-profile file. The host owns
// this file; legacy binaries must never be configured with its path. Native
// Answer/Forget on this returned Book are explicit trusted operator integration.
func LoadApplicationBook(path string) (*Book, error) {
	owner, e := user.Current()
	if e != nil || owner.Uid == "" {
		return nil, errors.New("asks: application account unavailable")
	}
	b := &Book{path: path, application: true, owner: owner.Uid, decided: map[string]chan struct{}{}}
	_, e = b.readApplication()
	return b, e
}
func (b *Book) ApplicationProfile() bool { return b != nil && b.application }
func decodeApplication(data []byte) (book, error) {
	if data == nil {
		return book{Profile: ApplicationProfile}, nil
	}
	var f book
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(&f); e != nil {
		return f, e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return f, errors.New("asks: trailing application state")
	}
	audited := f.Profile == ApplicationProfile
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
		return f, errors.New("asks: only version 9 carries the answer audit")
	}
	ids := map[string]bool{}
	keys := map[string]bool{}
	for _, r := range f.Asks {
		if r == nil || r.ID == "" || ids[r.ID] {
			return f, errors.New("asks: invalid application record")
		}
		q, ok := Find(r.Key)
		if !ok || r.Asked.IsZero() || r.Asker == "" || !validRecordedPublisher(r) {
			return f, errors.New("asks: invalid application question")
		}
		if r.AnsweredBy != "" && r.AnsweredBy != AnsweredByOperator && r.AnsweredBy != AnsweredByResponder {
			return f, errors.New("asks: invalid answerer")
		}
		if r.Answerer != "" && (r.AnsweredBy != AnsweredByResponder || !validAnswerer(r.Answerer)) {
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
		if r.TypedPermission != nil && (!typed || r.Key != TypedPermissionKey || r.Permission != nil || !ValidTypedPermission(*r.TypedPermission) || r.AnsweredBy == AnsweredByResponder) || r.Key == TypedPermissionKey && r.TypedPermission == nil {
			return f, errors.New("asks: invalid typed permission")
		}
		if r.TypedPermission != nil && (r.About != r.TypedPermission.Effect.Subject.Program || r.Text != "OA permission request" || !slices.Equal(r.Options, []string{"allow", "refuse"})) {
			return f, errors.New("asks: mismatched typed permission projection")
		}
		if r.Pending() {
			if r.Yes || r.Kept || !r.Answered.IsZero() || r.AnsweredBy != "" || r.Answerer != "" {
				return f, errors.New("asks: invalid pending decision")
			}
		} else {
			option, ok := q.Option(r.Option)
			if !ok || option.Yes != r.Yes || option.Kept != r.Kept || r.Answered.IsZero() {
				return f, errors.New("asks: invalid recorded decision")
			}
		}
		ids[r.ID] = true
	}
	for _, a := range f.Admissions {
		key := a.Scope + "\x00" + a.Key
		digest, decodeErr := hex.DecodeString(a.Fingerprint)
		if !ValidRequestKey(a.Scope) || !ValidRequestKey(a.Key) || decodeErr != nil || len(digest) != 32 || a.ID == "" || keys[key] {
			return f, errors.New("asks: invalid application admission")
		}
		keys[key] = true
	}
	// This version owns a canonical JSON file. Refuse duplicate fields, invalid
	// Unicode and alternate spellings before they can normalize authority data.
	canonical, e := json.Marshal(&f)
	if e != nil || !bytes.Equal(data, canonical) {
		return f, errors.New("asks: noncanonical application state")
	}
	// Versions 7 and 8 audited only assistants' answers to shared choices;
	// those entries open the answer audit, oldest first (ASK-O15).
	if !audited {
		for _, d := range f.Delegated {
			f.Answers = append(f.Answers, delegatedEntry(d))
		}
		f.AnswersSinceMs = f.DelegatedSinceMs
		f.Delegated, f.DelegatedSinceMs = nil, 0
	}
	if e := validateChoicePartition(f); e != nil {
		return f, e
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
	for _, r := range f.Asks {
		if r.Permission != nil && r.Permission.Targets[0].Account != b.owner {
			return errors.New("asks: foreign permission account")
		}
		if r.TypedPermission != nil && r.TypedPermission.Effect.Subject.Account != b.owner {
			return errors.New("asks: foreign typed permission account")
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
		f.expire(b.clock(), b.pendingLifetime())
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
	e := cas.ChangeLimit(b.path, MaxApplicationFileBytes, func(data []byte) ([]byte, error) {
		f, e := decodeApplication(data)
		if e != nil {
			return nil, e
		}
		if e = b.validatePermissionAccounts(f); e != nil {
			return nil, e
		}
		expired = f.expire(b.clock(), b.pendingLifetime())
		choiceTime := f.choiceTime(b.clock().UnixMilli())
		choiceExpired := append(f.collectChoices(choiceTime), f.collectNotifications(choiceTime)...)
		if f.ChoiceTimeMs == 0 || len(choiceExpired) > 0 {
			f.noteChoiceTime(choiceTime)
		}
		expired = append(expired, choiceExpired...)
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
		if e = edit(&f); e != nil {
			return nil, e
		}
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
	q, ok := Find(key)
	if !ok {
		return Ask{}, "", ErrUnknownQuestion
	}
	required := map[string]bool{}
	for _, m := range slotRE.FindAllStringSubmatch(q.Text, -1) {
		if m[1] != "asker" {
			required[m[1]] = true
		}
	}
	if len(slots) != len(required) {
		return Ask{}, "", ErrBadSlot
	}
	for k, v := range slots {
		if !required[k] || !utf8.ValidString(v) || utf8.RuneCountInString(v) > maxSlot || strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return Ask{}, "", ErrBadSlot
		}
	}
	a := Ask{Asker: display, Key: key, Slots: slots}
	text, e := q.Render(a)
	return a, text, e
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
	return b.askApplicationFrom(scope, display, "", requestKey, key, slots, via, publisher, nil)
}

// AskApplicationAs is AskApplicationFrom for the bound program the receiver
// derived scope from (ASK-A1). The new record keeps that program and shows
// AskerDisplay of it as its asker (ASK-A2). A replay or a shared question
// keeps the record it was admitted with.
func (b *Book) AskApplicationAs(scope, program, requestKey, key string, slots map[string]string, via listen.Seen, publisher Publisher) (Record, string, error) {
	if !validAskerProgram(program) {
		return Record{}, "invalid", nil
	}
	return b.askApplicationFrom(scope, AskerDisplay(program), program, requestKey, key, slots, via, publisher, nil)
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
	permission, err := NewPermissionRequest(permission.Targets[0])
	if err != nil {
		return Record{}, "invalid", nil
	}
	return b.askApplicationFrom(scope, display, "", requestKey, FirstUseKey, nil, via, publisher, &permission)
}

func (b *Book) askApplicationFrom(scope, display, program, requestKey, key string, slots map[string]string, via listen.Seen, publisher Publisher, permission *PermissionRequest) (Record, string, error) {
	if key == TypedPermissionKey {
		return Record{}, "invalid", nil
	}
	if !b.application {
		return Record{}, "", errors.New("asks: application Book required")
	}
	if scope == "" || !ValidRequestKey(requestKey) {
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
			copy := out
			f.Asks = append(f.Asks, &copy)
		}
		f.Admissions = append(f.Admissions, applicationAdmission{scope, requestKey, fingerprint, out.ID})
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
func (b *Book) ReleaseAnsweredApplicationID(scope, key, expectedID, option string) (bool, error) {
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
		f.expire(b.clock(), b.pendingLifetime())
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

func (b *Book) ObserveApplication(scope, key string) (Record, string, error) {
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
