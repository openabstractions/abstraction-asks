package asks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/openabstractions/abstraction-asks/go/typedpermission"
	"slices"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

const MaxOrdinaryAdmissions = 1024
const MaxOrdinaryBytes = 1 << 20

// MaxDelegatedAnswers and MaxDelegatedBytes bound the answer audit, which
// holds assistants' answers to shared choices (ASK-O13) beside every other
// recorded answer (ASK-O15); DelegatedRetentionMilliseconds is how long an
// entry is kept after its answer.
const MaxDelegatedAnswers = 256
const MaxDelegatedBytes = 256 << 10
const DelegatedRetentionMilliseconds int64 = 7 * 24 * 60 * 60 * 1000
const MaxApplicationFileBytes = MaxApplicationBytes + MaxOrdinaryBytes + MaxDelegatedBytes + MaxNotificationBytes + maxProgramHashBytes + 4096

// maxProgramHashBytes bounds the legacy admissions' program fingerprints
// (ASK-P5): the 82 bytes of `,"program_hash":"<64 hex>"` for each of at
// most MaxApplicationAdmissions admissions, 328 KiB.
const maxProgramHashBytes = MaxApplicationAdmissions * 82
const MaxChoiceReplayMilliseconds int64 = typedpermission.MaxChoiceReplayMilliseconds

var ErrOrdinaryCapacity = errors.New("asks: ordinary choice capacity reached")

// ErrChoiceNotShared refuses an assistant's answer to a choice, or an option,
// its application did not share (ASK-R9).
var ErrChoiceNotShared = errors.New("asks: choice or option not shared with an assistant")

// The route of a recorded ordinary decision (ASK-O12).
const (
	ChoiceRouteRequester = "requester"
	ChoiceRouteOperator  = "operator"
	ChoiceRouteResponder = "responder"
)

// choiceAdmission is separate from the legacy indefinite-key partition. A
// acknowledged occurrence retains only its reference, fingerprints and ID.
// AskerProgram is the bound requester program, Shared the options its
// application shared with an assistant and Route the service route of the
// recorded decision; all three go with the record at acknowledgement.
// ProgramHash, from version 10, is the bound requester's fingerprint an
// acknowledgement keeps, so the tombstone stays in its share (ASK-P5).
type choiceAdmission struct {
	NativeOwner  *nativeApplicationOwner `json:"native_owner,omitempty"`
	Scope        string                  `json:"scope"`
	Reference    wire.RequestRef         `json:"reference"`
	Fingerprint  string                  `json:"fingerprint"`
	ID           string                  `json:"id"`
	Record       *wire.ChoiceRecord      `json:"record,omitempty"`
	AnswerHash   string                  `json:"answer_hash,omitempty"`
	Via          *listen.Seen            `json:"via,omitempty"`
	AskerProgram string                  `json:"asker_program,omitempty"`
	Shared       []string                `json:"shared,omitempty"`
	Route        string                  `json:"route,omitempty"`
	ProgramHash  string                  `json:"program_hash,omitempty"`
}

// ChoiceCandidate is a pending shared choice as the trusted answerer check
// sees it: the record, the bound requester program, the shared options and
// the requester's native binding evidence.
type ChoiceCandidate struct {
	Record       wire.ChoiceRecord
	AskerProgram string
	Shared       []string
	Via          listen.Seen
}

// ValidAssistantOptions reports whether shared names 1..3 distinct options
// the request offers (ASK-A12).
func ValidAssistantOptions(q wire.ChoiceRequest, shared []string) bool {
	return typedpermission.ValidAssistantOptions(q, shared)
}

// choiceContent is the immutable content of a reference: the request alone
// for Ask, and the request with its shared options for ShareChoice.
func choiceContent(q wire.ChoiceRequest, shared []string) string {
	if len(shared) == 0 {
		return choiceFingerprint(q)
	}
	return choiceFingerprint(struct {
		Request wire.ChoiceRequest
		Shared  []string
	}{q, shared})
}

func choiceText(s string) bool                      { return typedpermission.ChoiceText(s) }
func ValidChoiceOptionID(s string) bool             { return typedpermission.ValidChoiceOptionID(s) }
func ValidChoiceReference(ref wire.RequestRef) bool { return typedpermission.ValidChoiceReference(ref) }
func ValidChoiceAnswer(answer wire.ChoiceAnswer) bool {
	return typedpermission.ValidChoiceAnswer(answer, permissionValidators())
}

// ValidChoiceRequest checks immutable shape independently of the service clock.
func ValidChoiceRequest(q wire.ChoiceRequest) bool { return typedpermission.ValidChoiceRequest(q) }
func choiceFingerprint(v any) string {
	data, _ := json.Marshal(v)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func validChoiceHash(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == s
}
func choiceOption(q wire.ChoiceRequest, id string) bool { return typedpermission.ChoiceOffers(q, id) }
func cloneChoice(r wire.ChoiceRecord) wire.ChoiceRecord {
	r.Request.Options = append([]wire.ChoiceOption{}, r.Request.Options...)
	if r.Answer != nil {
		a := *r.Answer
		r.Answer = &a
	}
	return r
}
func ValidChoiceRecord(r wire.ChoiceRecord) bool {
	return typedpermission.ValidChoiceRecord(r, permissionValidators())
}

func validateChoicePartition(f book) error {
	legacy := f
	legacy.Choices = nil
	legacy.ChoiceTimeMs = 0
	legacy.Delegated, legacy.DelegatedSinceMs = nil, 0
	legacy.Answers, legacy.AnswersSinceMs = nil, 0
	legacy.Notifications, legacy.Notifiers = nil, nil
	// Program fingerprints have their own bound (maxProgramHashBytes) and the
	// version label is the file's, so a full legacy partition still migrates
	// to version 10.
	legacy.Profile = ""
	legacy.Admissions = slices.Clone(f.Admissions)
	for i := range legacy.Admissions {
		legacy.Admissions[i].ProgramHash = ""
	}
	raw, e := json.Marshal(legacy)
	if e != nil || len(raw) > MaxApplicationBytes {
		return ErrApplicationCapacity
	}
	raw, e = json.Marshal(struct {
		Choices []choiceAdmission
		TimeMs  int64
	}{f.Choices, f.ChoiceTimeMs})
	if e != nil || len(raw) > MaxOrdinaryBytes || len(f.Choices) > MaxOrdinaryAdmissions {
		return ErrOrdinaryCapacity
	}
	if f.ChoiceTimeMs < 0 || len(f.Choices) > 0 && f.ChoiceTimeMs == 0 {
		return errors.New("asks: invalid ordinary time")
	}
	refs, ids := map[string]bool{}, map[string]bool{}
	for _, a := range f.Choices {
		key := a.Scope + "\x00" + choiceFingerprint(a.Reference)
		if !ValidRequestKey(a.Scope) || !ValidChoiceReference(a.Reference) || !validChoiceHash(a.Fingerprint) || !ValidRequestKey(a.ID) || refs[key] || ids[a.ID] {
			return errors.New("asks: invalid ordinary admission")
		}
		if a.Record == nil {
			if !validChoiceHash(a.AnswerHash) || a.AskerProgram != "" || a.Shared != nil || a.Route != "" || a.ProgramHash != "" && !validChoiceHash(a.ProgramHash) {
				return errors.New("asks: invalid ordinary acknowledgement")
			}
		} else if a.AnswerHash != "" || a.ProgramHash != "" || !ValidChoiceRecord(*a.Record) || a.Record.AskedMs > f.ChoiceTimeMs || a.Record.Answer != nil && a.Record.Answer.AnsweredMs > f.ChoiceTimeMs || a.Record.ID != a.ID || a.Record.Request.RequestRef != a.Reference || choiceContent(a.Record.Request, a.Shared) != a.Fingerprint {
			return errors.New("asks: invalid ordinary record")
		} else if !validChoiceAttribution(a) {
			return errors.New("asks: invalid ordinary attribution")
		}
		refs[key], ids[a.ID] = true, true
	}
	return validateAnswers(f)
}

// validChoiceAttribution checks the version 7 fields of a retained record:
// the requester program derives the asker, a shared choice names its
// requester, and the route agrees with the recorded decision.
func validChoiceAttribution(a choiceAdmission) bool {
	r := a.Record
	if a.AskerProgram != "" && (!validAnswerer(a.AskerProgram) || r.Asker != AskerDisplay(a.AskerProgram)) {
		return false
	}
	if a.Shared != nil && (a.AskerProgram == "" || !ValidAssistantOptions(r.Request, a.Shared)) {
		return false
	}
	decided := r.State == wire.ChoiceStateAnswered || r.State == wire.ChoiceStateDismissed
	switch a.Route {
	case "":
		return true
	case ChoiceRouteRequester, ChoiceRouteOperator:
		return decided
	case ChoiceRouteResponder:
		return r.State == wire.ChoiceStateAnswered && slices.Contains(a.Shared, r.Answer.Option)
	}
	return false
}

// validateAnswers checks the answer audit's bounds and order. A decoded
// version 7 or 8 delegated list has already moved into it.
func validateAnswers(f book) error {
	raw, e := json.Marshal(struct {
		Answers []retainedAnswer
		SinceMs int64
	}{f.Answers, f.AnswersSinceMs})
	if e != nil || len(raw) > MaxDelegatedBytes || len(f.Answers) > MaxDelegatedAnswers || f.AnswersSinceMs < 0 || len(f.Delegated) != 0 || f.DelegatedSinceMs != 0 {
		return errors.New("asks: invalid answer audit")
	}
	ids := map[string]bool{}
	for i, d := range f.Answers {
		if !ValidAnswerEntry(d.AnswerEntry) || d.Content != nil && (d.Kind != wire.AnswerKindQuestion || !ValidCatalogueContent(d.Key, d.About, *d.Content)) || ids[d.ID] || d.AnsweredMs > f.ChoiceTimeMs || i > 0 && d.AnsweredMs < f.Answers[i-1].AnsweredMs {
			return errors.New("asks: invalid audited answer")
		}
		ids[d.ID] = true
	}
	return nil
}

// MaxAnswerTitleBytes bounds a catalogue question's text in the answer audit:
// a first-use sentence names a program path, an action and a resource.
const MaxAnswerTitleBytes = typedpermission.MaxAnswerTitleBytes

// ValidAnswerEntry reports whether e is a well-formed answer audit entry
// (ASK-O15): a catalogue question's key, About, text and option, or an
// ordinary choice's title, option and label, and the route's answerer.
func ValidAnswerEntry(e wire.AnswerEntry) bool {
	return typedpermission.ValidAnswerEntry(e, permissionValidators())
}

// delegatedEntry is a version 7 or 8 delegated answer as an audit entry.
func delegatedEntry(d legacyDelegatedAnswer) wire.AnswerEntry {
	return wire.AnswerEntry{ID: d.ID, Kind: wire.AnswerKindChoice, Asker: d.Asker, AskerProgram: d.AskerProgram, Publisher: d.Publisher, PublisherTrusted: d.PublisherTrusted,
		Title: d.Title, Option: d.Option, Label: d.Label, AnsweredBy: wire.AnswererResponder, Answerer: d.Answerer, AnsweredMs: d.AnsweredMs}
}

// collectAnswers drops audit entries older than their retention.
func (f *book) collectAnswers(now int64) {
	f.Answers = slices.DeleteFunc(f.Answers, func(d retainedAnswer) bool { return now-d.AnsweredMs >= DelegatedRetentionMilliseconds })
	if len(f.Answers) == 0 {
		f.Answers = nil
	}
	// An evicted entry would have aged out by now as well.
	if f.AnswersSinceMs != 0 && now-f.AnswersSinceMs >= DelegatedRetentionMilliseconds {
		f.AnswersSinceMs = 0
	}
}

// recordAnswer appends one audit entry at the ordinary time floor, evicting
// the oldest while either bound would be exceeded and recording the eviction
// time. An entry that is not well formed is not recorded: the audit never
// refuses the answer it describes.
func (f *book) recordAnswer(d wire.AnswerEntry, now int64, content ...*wire.CatalogueContent) {
	d.AnsweredMs = f.choiceTime(now)
	if !ValidAnswerEntry(d) || slices.ContainsFunc(f.Answers, func(e retainedAnswer) bool { return e.ID == d.ID }) {
		return
	}
	f.noteChoiceTime(d.AnsweredMs)
	entry := retainedAnswer{AnswerEntry: d}
	if len(content) != 0 {
		entry.Content = cloneCatalogueContent(content[0])
	}
	f.Answers = append(f.Answers, entry)
	for len(f.Answers) > 1 {
		raw, e := json.Marshal(struct {
			Answers []retainedAnswer
			SinceMs int64
		}{f.Answers, f.AnswersSinceMs})
		if e == nil && len(raw) <= MaxDelegatedBytes-64 && len(f.Answers) <= MaxDelegatedAnswers {
			return
		}
		f.AnswersSinceMs = max(f.AnswersSinceMs, f.Answers[0].AnsweredMs)
		f.Answers = slices.Delete(f.Answers, 0, 1)
	}
}

// questionAnswerEntry is a catalogue question's answer as an audit entry. A
// typed permission's text is a fixed projection, so its trusted bundle label
// stands for it when it has one.
func questionAnswerEntry(r Record) wire.AnswerEntry {
	title := r.Text
	if r.TypedPermission != nil && permissionString(r.TypedPermission.Display.BundleLabel, MaxAnswerTitleBytes) {
		title = r.TypedPermission.Display.BundleLabel
	}
	by := wire.AnswererOperator
	if r.AnsweredBy == AnsweredByResponder {
		by = wire.AnswererResponder
	}
	return wire.AnswerEntry{ID: r.ID, Kind: wire.AnswerKindQuestion, Key: r.Key, Asker: r.Asker, AskerProgram: r.AskerProgram, Publisher: r.Publisher, PublisherTrusted: r.PublisherTrusted,
		About: r.About, Title: title, Option: r.Option, Label: r.Option, AnsweredBy: by, Answerer: r.Answerer}
}

func (f *book) choiceTime(now int64) int64 { return max(now, f.ChoiceTimeMs) }
func (f *book) noteChoiceTime(now int64)   { f.ChoiceTimeMs = f.choiceTime(now) }

// readChoices commits an observed expiry before exposing its terminal state.
// Unexpired reads retain the durable floor without advancing it or writing.
// Callers hold b.mu. Legacy lifecycle still uses its existing wall-clock rule.
func (b *Book) readChoices() (book, error) {
	f, err := b.readApplication()
	if err != nil {
		return book{}, err
	}
	now := f.choiceTime(b.clock().UnixMilli())
	if now <= 0 {
		return book{}, errors.New("asks: ordinary service time unavailable")
	}
	settledChoices, settledNotifications := f.collectChoices(now), f.collectNotifications(now)
	if len(settledChoices) == 0 && len(settledNotifications) == 0 {
		return f, nil
	}
	if err = b.changeApplication(func(f *book) error { f.noteChoiceTime(now); return nil }); err != nil {
		return book{}, err
	}
	return b.readApplication()
}

func (f *book) collectChoices(now int64) []string {
	var settled []string
	kept := f.Choices[:0]
	for _, a := range f.Choices {
		if now >= a.Reference.ReplayUntilMs {
			settled = append(settled, a.ID)
			continue
		}
		if a.Record != nil && a.Record.State == wire.ChoiceStatePending && now >= a.Record.Request.PendingUntilMs {
			a.Record.State = wire.ChoiceStateExpired
			settled = append(settled, a.ID)
		}
		kept = append(kept, a)
	}
	f.Choices = kept
	f.collectAnswers(now)
	return settled
}
func choiceObservation(a choiceAdmission) wire.ChoiceObservation {
	if a.Record == nil {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeGone}
	}
	r := cloneChoice(*a.Record)
	state, _ := wire.ParseChoiceOutcome(r.State.String())
	result := wire.ChoiceObservation{Outcome: state}
	if r.State != wire.ChoiceStateExpired {
		result.ID, result.Answer = r.ID, r.Answer
	}
	return result
}
func (f *book) choiceIndex(scope string, ref wire.RequestRef) int {
	return slices.IndexFunc(f.Choices, func(a choiceAdmission) bool { return a.Scope == scope && a.Reference == ref })
}
func (f *book) choiceID(id string) int {
	return slices.IndexFunc(f.Choices, func(a choiceAdmission) bool { return a.ID == id })
}

// AskChoiceFrom accepts only service-derived scope, display and attribution.
// Trusted authorization is checked inside the same edit that admits the choice.
func (b *Book) AskChoiceFrom(scope, display string, q wire.ChoiceRequest, via listen.Seen, publisher Publisher, authorize func() error) (wire.ChoiceObservation, error) {
	return b.askChoice(nil, scope, display, "", q, nil, via, publisher, authorize)
}

// AskChoiceAs is AskChoiceFrom for the bound program the receiver derived
// scope from (ASK-A1): the record keeps that program and shows AskerDisplay
// of it. A nonempty shared lists the options an enrolled assistant may
// choose (ASK-A12); shared and request together are the reference's
// immutable content.
func (b *Book) AskChoiceAs(scope, program string, q wire.ChoiceRequest, shared []string, via listen.Seen, publisher Publisher, authorize func() error) (wire.ChoiceObservation, error) {
	if shared != nil && (!validAnswerer(program) || !ValidAssistantOptions(q, shared)) {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeInvalid}, nil
	}
	display := AskerDisplay(program)
	if !validAnswerer(program) {
		// An unshared choice keeps its earlier display-only attribution.
		program = ""
	}
	return b.askChoice(nil, scope, display, program, q, slices.Clone(shared), via, publisher, authorize)
}

func (b *Book) askChoice(native *nativeApplicationOwner, scope, display, program string, q wire.ChoiceRequest, shared []string, via listen.Seen, publisher Publisher, authorize func() error) (wire.ChoiceObservation, error) {
	if !b.application || !ValidRequestKey(scope) || display == "" || !ValidChoiceRequest(q) {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeInvalid}, nil
	}
	content := choiceContent(q, shared)
	b.mu.Lock()
	defer b.mu.Unlock()
	result := wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeUnknown}
	err := b.changeApplication(func(f *book) error {
		if authorize != nil {
			if e := authorize(); e != nil {
				return e
			}
		}
		selected, nativeErr := f.nativeScope(native, scope, "choice", q.RequestRef)
		if nativeErr != nil {
			return nativeErr
		}
		scope = selected

		now := f.choiceTime(b.clock().UnixMilli())
		if now <= 0 {
			return errors.New("asks: ordinary service time unavailable")
		}
		if now >= q.RequestRef.ReplayUntilMs {
			f.noteChoiceTime(now)
			result.Outcome = wire.ChoiceOutcomeExpired
			return nil
		}
		i := f.choiceIndex(scope, q.RequestRef)
		if i >= 0 {
			if f.Choices[i].Fingerprint != content {
				result.Outcome = wire.ChoiceOutcomeConflict
			} else {
				result = choiceObservation(f.Choices[i])
			}
			return nil
		}
		if q.PendingUntilMs <= now {
			f.noteChoiceTime(now)
			result.Outcome = wire.ChoiceOutcomeExpired
			return nil
		}
		if q.RequestRef.ReplayUntilMs-now > MaxChoiceReplayMilliseconds {
			result.Outcome = wire.ChoiceOutcomeInvalid
			return nil
		}
		if len(f.Choices) >= MaxOrdinaryAdmissions {
			return ErrOrdinaryCapacity
		}
		publisher = publisher.recorded()
		record := wire.ChoiceRecord{ID: random(16), Asker: display, Request: q, State: wire.ChoiceStatePending, AskedMs: now,
			Publisher: publisher.Subject, PublisherTrusted: publisher.Trusted, ProgramDescription: publisher.Description, ProgramOriginalName: publisher.OriginalName}
		record = cloneChoice(record)
		a := choiceAdmission{NativeOwner: native.recorded(scope), Scope: scope, Reference: q.RequestRef, Fingerprint: content, ID: record.ID, Record: &record, Via: &via, AskerProgram: program, Shared: shared}
		if f.ordinaryShareFull(a) {
			return ErrOrdinaryCapacity // one program's share (ASK-P5)
		}
		f.Choices = append(f.Choices, a)
		f.noteChoiceTime(now)
		result = choiceObservation(a)
		return nil
	})
	if errors.Is(err, ErrOrdinaryCapacity) {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeExhausted}, nil
	}
	return result, err
}
func (b *Book) nativeObserveChoice(native *nativeApplicationOwner, scope string, ref wire.RequestRef) (wire.ChoiceObservation, error) {
	if !b.application || !ValidRequestKey(scope) || !ValidChoiceReference(ref) {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeInvalid}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := b.readChoices()
	if err != nil {
		return wire.ChoiceObservation{}, err
	}

	selected, nativeErr := f.nativeScope(native, scope, "choice", ref)
	if nativeErr != nil {
		return wire.ChoiceObservation{}, nativeErr
	}
	scope = selected

	now := f.choiceTime(b.clock().UnixMilli())
	if now >= ref.ReplayUntilMs {
		if now > f.ChoiceTimeMs {
			if err := b.changeApplication(func(f *book) error { f.noteChoiceTime(now); return nil }); err != nil {
				return wire.ChoiceObservation{}, err
			}
		}
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeExpired}, nil
	}
	if i := f.choiceIndex(scope, ref); i >= 0 {
		return choiceObservation(f.Choices[i]), nil
	}
	return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeUnknown}, nil
}
func (b *Book) nativeAcknowledgeChoice(native *nativeApplicationOwner, scope string, ref wire.RequestRef, answer wire.ChoiceAnswer, authorize func() error) (wire.ChoiceAcknowledgement, error) {
	if !b.application || !ValidRequestKey(scope) || !ValidChoiceReference(ref) || !ValidChoiceAnswer(answer) {
		return wire.ChoiceAcknowledgement{Outcome: wire.ChoiceAckOutcomeInvalid}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	result := wire.ChoiceAcknowledgement{Outcome: wire.ChoiceAckOutcomeUnknown}
	err := b.changeApplication(func(f *book) error {
		if authorize != nil {
			if e := authorize(); e != nil {
				return e
			}
		}
		selected, nativeErr := f.nativeScope(native, scope, "choice", ref)
		if nativeErr != nil {
			return nativeErr
		}
		scope = selected

		now := f.choiceTime(b.clock().UnixMilli())
		if now <= 0 {
			return errors.New("asks: ordinary service time unavailable")
		}
		if now >= ref.ReplayUntilMs {
			f.noteChoiceTime(now)
			result.Outcome = wire.ChoiceAckOutcomeExpired
			return nil
		}
		i := f.choiceIndex(scope, ref)
		if i < 0 {
			return nil
		}
		a := &f.Choices[i]
		hash := choiceFingerprint(answer)
		if a.ID != answer.ID || a.Record == nil && a.AnswerHash != hash || a.Record != nil && (a.Record.Answer == nil || *a.Record.Answer != answer) {
			result.Outcome = wire.ChoiceAckOutcomeConflict
			return nil
		}
		if a.Record != nil {
			if a.AskerProgram != "" {
				a.ProgramHash = programHash(a.AskerProgram) // stays in its share (ASK-P5)
				if a.NativeOwner != nil {
					a.ProgramHash = programHash(a.NativeOwner.Subject.Program)
				}
			}
			a.Record, a.AnswerHash, a.Via = nil, hash, nil
			a.AskerProgram, a.Shared, a.Route = "", nil, ""
			f.noteChoiceTime(now)
		}
		result.Outcome = wire.ChoiceAckOutcomeAcknowledged
		return nil
	})
	return result, err
}

func (b *Book) ChoiceHistory() ([]wire.ChoiceRecord, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return nil, "", errors.New("asks: application Book required")
	}
	f, err := b.readChoices()
	if err != nil {
		return nil, "", err
	}
	rows := []wire.ChoiceRecord{}
	for _, a := range f.Choices {
		if a.Record != nil {
			rows = append(rows, cloneChoice(*a.Record))
		}
	}
	return rows, choiceFingerprint(f.Choices), nil
}
func (b *Book) ChoiceRecord(id string) (wire.ChoiceRecord, bool, error) {
	rows, _, err := b.ChoiceHistory()
	for _, r := range rows {
		if r.ID == id {
			return r, true, err
		}
	}
	return wire.ChoiceRecord{}, false, err
}
func (b *Book) choicePending(id string) bool {
	if !b.application {
		return false
	}
	f, err := b.readApplication()
	i := f.choiceID(id)
	return err == nil && i >= 0 && f.Choices[i].Record != nil && f.Choices[i].Record.State == wire.ChoiceStatePending
}

// DecideChoiceAuthorized binds the actor from the proven operator. A replay
// returns the original actor and timestamp; no terminal decision is rewritten.
func (b *Book) DecideChoiceAuthorized(id, option, actor string, dismiss, ifPending bool, authorize func() error) (wire.ChoiceDecision, error) {
	if !ValidRequestKey(id) {
		return wire.ChoiceDecision{Outcome: wire.ChoiceDecisionOutcomeInvalid}, nil
	}
	return b.decideChoiceAuthorized(nil, id, "", nil, option, actor, dismiss, ifPending, authorize)
}

// DecideOwnChoiceAuthorized addresses only the proven requester's full reference.
// Ownership lookup and authorization run inside the same atomic decision edit.
func (b *Book) nativeDecideOwnChoiceAuthorized(native *nativeApplicationOwner, scope string, ref wire.RequestRef, option, actor string, dismiss, ifPending bool, authorize func() error) (wire.ChoiceDecision, error) {
	if !ValidRequestKey(scope) || !ValidChoiceReference(ref) {
		return wire.ChoiceDecision{Outcome: wire.ChoiceDecisionOutcomeInvalid}, nil
	}
	return b.decideChoiceAuthorized(native, "", scope, &ref, option, actor, dismiss, ifPending, authorize)
}

func (b *Book) decideChoiceAuthorized(native *nativeApplicationOwner, id, scope string, ref *wire.RequestRef, option, actor string, dismiss, ifPending bool, authorize func() error) (wire.ChoiceDecision, error) {
	if !b.application || !validAnswerer(actor) || dismiss && option != "" || !dismiss && !ValidChoiceOptionID(option) {
		return wire.ChoiceDecision{Outcome: wire.ChoiceDecisionOutcomeInvalid}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	result := wire.ChoiceDecision{Outcome: wire.ChoiceDecisionOutcomeUnknown}
	err := b.changeApplication(func(f *book) error {
		if authorize != nil {
			if e := authorize(); e != nil {
				return e
			}
		}
		if ref != nil {
			selected, nativeErr := f.nativeScope(native, scope, "choice", *ref)
			if nativeErr != nil {
				return nativeErr
			}
			scope = selected
		}

		i, now := -1, int64(0)
		if ref != nil {
			now = f.choiceTime(b.clock().UnixMilli())
			if now <= 0 {
				return errors.New("asks: ordinary service time unavailable")
			}
			if now >= ref.ReplayUntilMs {
				f.noteChoiceTime(now)
				result.Outcome = wire.ChoiceDecisionOutcomeExpired
				return nil
			}
			i = f.choiceIndex(scope, *ref)
		} else {
			i = f.choiceID(id)
		}
		if i < 0 {
			return nil
		}
		a := &f.Choices[i]
		if ref == nil {
			now = f.choiceTime(b.clock().UnixMilli())
		}
		if now <= 0 {
			return errors.New("asks: ordinary service time unavailable")
		}
		if now >= a.Reference.ReplayUntilMs {
			f.noteChoiceTime(now)
			result.Outcome = wire.ChoiceDecisionOutcomeExpired
			return nil
		}
		if a.Record == nil {
			result.Outcome = wire.ChoiceDecisionOutcomeGone
			return nil
		}
		r := a.Record
		if r.State == wire.ChoiceStateExpired || now >= r.Request.PendingUntilMs && r.State == wire.ChoiceStatePending {
			r.State = wire.ChoiceStateExpired
			f.noteChoiceTime(now)
			result.Outcome = wire.ChoiceDecisionOutcomeExpired
			return nil
		}
		if !dismiss && !choiceOption(r.Request, option) {
			result.Outcome = wire.ChoiceDecisionOutcomeInvalid
			return nil
		}
		state := wire.ChoiceStateAnswered
		if dismiss {
			state = wire.ChoiceStateDismissed
		}
		if r.State != wire.ChoiceStatePending {
			if ifPending || r.State != state || r.Answer.Option != option {
				result.Outcome = wire.ChoiceDecisionOutcomeConflict
				return nil
			}
		} else {
			r.State = state
			r.Answer = &wire.ChoiceAnswer{ID: a.ID, Option: option, AnsweredMs: now, AnsweredBy: actor}
			a.Route = ChoiceRouteOperator
			if ref != nil {
				a.Route = ChoiceRouteRequester
			}
			f.noteChoiceTime(now)
			// An operator's answer joins the answer audit (ASK-O15); a
			// dismissal and the application's own decision do not.
			if a.Route == ChoiceRouteOperator && !dismiss {
				label := ""
				for _, o := range r.Request.Options {
					if o.ID == option {
						label = o.Label
					}
				}
				f.recordAnswer(wire.AnswerEntry{ID: a.ID, Kind: wire.AnswerKindChoice, Asker: r.Asker, AskerProgram: a.AskerProgram, Publisher: r.Publisher, PublisherTrusted: r.PublisherTrusted,
					Title: r.Request.Title, Option: option, Label: label, AnsweredBy: wire.AnswererOperator}, now)
			}
		}
		parsed, _ := wire.ParseChoiceDecisionOutcome(state.String())
		copy := cloneChoice(*r)
		result = wire.ChoiceDecision{Outcome: parsed, Record: &copy}
		return nil
	})
	if err == nil && result.Record != nil {
		b.settle(result.Record.ID)
	}
	return result, err
}
