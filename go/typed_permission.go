package asks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	typedpermission "github.com/openabstractions/abstraction-asks/go/typedpermission"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/fileid"
	"github.com/openabstractions/abstraction-identity/remote"
	"log/slog"
	"slices"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
	rightswire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

const TypedPermissionKey = typedpermission.Key
const MaxPermissionSelections = typedpermission.MaxPermissionSelections
const MaxPermissionTargetBytes = typedpermission.MaxPermissionTargetBytes

var ErrTypedPermissionRequired = errors.New("asks: typed permission operator required")

// TypedPermission is frozen admission data. Only trusted capability preparation
// establishes its domain meaning. The Book validates retained closed syntax.
type TypedPermission struct {
	Effect  rightswire.PermissionEffect `json:"effect"`
	Digest  string                      `json:"digest"`
	Origin  string                      `json:"origin"`
	Purpose string                      `json:"purpose"`
	Display wire.PermissionDisplay      `json:"display"`
}

func ValidTypedPermission(p TypedPermission) bool {
	return typedpermission.ValidPermission(typedpermission.Permission(p), typedpermission.Validators{PackagedProgramPrefix: identity.PackagedProgramPrefix, NativeProgram: fileid.ValidSubjectProgram, RemoteProgram: remote.ValidSubjectProgram})
}
func PermissionEffectDigest(e rightswire.PermissionEffect) string {
	return typedpermission.PermissionEffectDigest(e)
}
func CloneTypedPermission(p TypedPermission) TypedPermission {
	return TypedPermission(typedpermission.ClonePermission(typedpermission.Permission(p)))
}

func typedFingerprint(p TypedPermission) string {
	raw, _ := json.Marshal(struct {
		Effect          rightswire.PermissionEffect
		Origin, Purpose string
	}{p.Effect, p.Origin, p.Purpose})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Replay compares authority content without refreshing retained display text.
// A changed compiled explanation never rewrites the occurrence the person saw.
func (b *Book) ReplayTypedPermission(scope, key string, p TypedPermission) (Record, string, error) {
	if !b.application || !ValidRequestKey(scope) || !ValidRequestKey(key) || !ValidTypedPermission(p) || p.Effect.Subject.Account != b.owner {
		return Record{}, "invalid", nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := b.readApplication()
	if err != nil {
		return Record{}, "unavailable", err
	}
	for _, ad := range f.Admissions {
		if ad.Scope != scope || ad.Key != key {
			continue
		}
		if ad.Fingerprint != typedFingerprint(p) {
			return Record{}, "conflict", nil
		}
		i := f.index(ad.ID)
		if i < 0 {
			return Record{}, "gone", nil
		}
		r := *f.Asks[i]
		if r.TypedPermission == nil {
			return Record{}, "conflict", nil
		}
		return r, recordOutcome(r), nil
	}
	return Record{}, "unknown", nil
}

// AskTypedPermissionFrom accepts only an already prepared immutable effect.
// Public handlers bind its subject and capability validation before calling it.
func (b *Book) AskTypedPermissionFrom(scope, display, requestKey string, p TypedPermission, via listen.Seen, publisher Publisher) (Record, string, error) {
	return b.AskTypedPermissionAuthorizedFrom(scope, display, requestKey, p, via, publisher, nil)
}

func (b *Book) AskTypedPermissionAuthorizedFrom(scope, display, requestKey string, p TypedPermission, via listen.Seen, publisher Publisher, authorize func() error) (Record, string, error) {
	out, outcome, err := b.askTypedPermission(scope, display, requestKey, p, via, publisher, authorize, true, false)
	if err == nil && (outcome == "invalid" || outcome == "conflict") {
		// Bounded: program, key and outcome only; request text and resources stay out of logs.
		slog.Info("permission admission refused", "program", boundedLogValue(display), "key", boundedLogValue(requestKey), "outcome", outcome)
	}
	return out, outcome, err
}

func boundedLogValue(v string) string {
	if len(v) > 256 {
		return v[:256]
	}
	return v
}

// AskTypedPermissionOccurrenceAuthorizedFrom is trusted host composition for
// finite service occurrences whose fresh request key must ask after Refuse.
// Same-key replay and current pending coalescing remain unchanged; generic
// typed admission keeps its ten-minute refusal pacing.
func (b *Book) AskTypedPermissionOccurrenceAuthorizedFrom(scope, display, requestKey string, p TypedPermission, via listen.Seen, publisher Publisher, authorize func() error) (Record, string, error) {
	if authorize == nil || p.Origin != "service" {
		return Record{}, "invalid", nil
	}
	return b.askTypedPermission(scope, display, requestKey, p, via, publisher, authorize, false, false)
}

// AskTypedPermissionWithOnceAuthorizedFrom binds Allow once to one trusted
// service request key. Only the person can answer it. Neither its decision
// nor its pending slot can satisfy a different call by coalescing.
func (b *Book) AskTypedPermissionWithOnceAuthorizedFrom(scope, display, requestKey string, p TypedPermission, via listen.Seen, publisher Publisher, authorize func() error) (Record, string, error) {
	if authorize == nil || p.Origin != "service" {
		return Record{}, "invalid", nil
	}
	return b.askTypedPermission(scope, display, requestKey, p, via, publisher, authorize, false, true)
}

func (b *Book) askTypedPermission(scope, display, requestKey string, p TypedPermission, via listen.Seen, publisher Publisher, authorize func() error, paceRefusal, once bool) (Record, string, error) {
	if !b.application || !ValidTypedPermission(p) || p.Effect.Subject.Account != b.owner || !ValidRequestKey(scope) || !ValidRequestKey(requestKey) || display == "" {
		return Record{}, "invalid", nil
	}
	p = CloneTypedPermission(p)
	fingerprint := typedFingerprint(p)
	options := typedpermission.Options(once)
	b.mu.Lock()
	defer b.mu.Unlock()
	var out Record
	outcome := "unknown"
	err := b.changeApplication(func(f *book) error {
		if authorize != nil {
			if err := authorize(); err != nil {
				return err
			}
		}
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
				if !slices.Equal(out.Options, options) {
					out, outcome = Record{}, "conflict"
					return nil
				}
				outcome = recordOutcome(out)
				return nil
			}
		}
		// New request keys share the existing pending occurrence. A recent Refuse
		// stands for the bounded interval even if an application varies its purpose.
		for _, r := range f.Asks {
			if once || r.TypedPermission == nil || r.TypedPermission.Digest != p.Digest || !slices.Equal(r.Options, options) {
				continue
			}
			if r.Pending() || paceRefusal && r.Option == "refuse" && b.clock().Before(r.Answered.Add(10*time.Minute)) {
				if len(f.Admissions) >= MaxApplicationAdmissions || f.applicationShareFull(recordShare(r, scope), false) {
					return ErrApplicationCapacity
				}
				out = *r
				f.Admissions = append(f.Admissions, applicationAdmission{Scope: scope, Key: requestKey, Fingerprint: fingerprint, ID: out.ID, ProgramHash: recordProgramHash(r)})
				outcome = recordOutcome(out)
				return nil
			}
		}
		if len(f.Admissions) >= MaxApplicationAdmissions || len(f.Asks) >= MaxApplicationAdmissions ||
			f.applicationShareFull(programShare(p.Effect.Subject.Program, scope), true) {
			return ErrApplicationCapacity // the partition, or one program's share (ASK-P5)
		}
		publisher = publisher.recorded()
		out = Record{ID: random(16), Asker: display, Key: TypedPermissionKey, About: p.Effect.Subject.Program, Text: "OA permission request", Options: options, Asked: b.clock().UTC(), Via: via, Publisher: publisher.Subject, PublisherTrusted: publisher.Trusted, ProgramDescription: publisher.Description, ProgramOriginalName: publisher.OriginalName, TypedPermission: &p}
		copy := out
		f.Asks = append(f.Asks, &copy)
		f.Admissions = append(f.Admissions, applicationAdmission{Scope: scope, Key: requestKey, Fingerprint: fingerprint, ID: out.ID, ProgramHash: recordProgramHash(&out)})
		outcome = "pending"
		return nil
	})
	return out, outcome, err
}

func typedPermissionOnce(r Record) bool {
	return r.TypedPermission != nil && r.TypedPermission.Origin == "service" && slices.Equal(r.Options, typedpermission.Options(true))
}

func recordedQuestionOption(r Record, q Question, option, by string) (Option, bool) {
	if r.TypedPermission != nil {
		if !slices.Contains(r.Options, option) || typedPermissionOnce(r) && by != AnsweredByOperator {
			return Option{}, false
		}
		if typedPermissionOnce(r) && option == "once" {
			return Option{Name: "once", Yes: true}, true
		}
	}
	return q.Option(option)
}

// PendingTypedPermissions lists the pending typed occurrences in Book order.
func (b *Book) PendingTypedPermissions() ([]Record, error) {
	if !b.application {
		return nil, errors.New("asks: application Book required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := b.readApplication()
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, r := range f.Asks {
		if r.TypedPermission != nil && r.Pending() {
			out = append(out, *r)
		}
	}
	return out, nil
}

// WithdrawSettledPermission removes the pending typed occurrence id, with
// every admission naming it and no tombstone, as expiry does (ASK-A7), when
// settled, called inside the Book edit with its immutable effect, reports
// that the owner's current decision no longer needs the person: a rule or
// default now permits it, or a Block refuses it (ASK-A13). Waiting observers
// are released; the requester's next Observe of the key reads unknown, and a
// new Request reads the current decision. An answered occurrence stays.
func (b *Book) WithdrawSettledPermission(id string, settled func(TypedPermission) bool) (bool, error) {
	if !b.application || !ValidRequestKey(id) || settled == nil {
		return false, errors.New("asks: application Book, occurrence and decision required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	withdrawn := false
	unchanged := errors.New("asks: nothing withdrawn")
	err := b.changeApplication(func(f *book) error {
		i := f.index(id)
		if i < 0 || f.Asks[i].TypedPermission == nil || !f.Asks[i].Pending() || !settled(CloneTypedPermission(*f.Asks[i].TypedPermission)) {
			return unchanged
		}
		f.Asks = slices.Delete(f.Asks, i, i+1)
		f.Admissions = slices.DeleteFunc(f.Admissions, func(a applicationAdmission) bool { return a.ID == id })
		withdrawn = true
		return nil
	})
	if errors.Is(err, unchanged) {
		return false, nil
	}
	if err == nil && withdrawn {
		b.settle(id)
	}
	return withdrawn, err
}

// Conditional typed operator decision. Legacy provider methods refuse these
// records; no delegated responder path is added.
func (b *Book) AnswerTypedPermissionIfPendingAuthorized(id, option string, authorize func(Record, string) error) (Record, string, error) {
	return b.answerApplication(id, option, true, AnsweredByOperator, "", func(r Record, scope string) error {
		if r.TypedPermission == nil || r.Key != TypedPermissionKey {
			return ErrTypedPermissionRequired
		}
		if authorize != nil {
			return authorize(r, scope)
		}
		return ErrTypedPermissionRequired
	})
}

// AnswerTypedPermissionAsResponderAuthorized records only a trusted host's
// checked typed answer. The host owns capability eligibility; the Book keeps
// the actual responder and audit, with no operator relabel or policy effect.
func (b *Book) AnswerTypedPermissionAsResponderAuthorized(id, option, answerer string, authorize func(Record, string) error) (Record, string, error) {
	if !validAnswerer(answerer) || authorize == nil {
		return Record{}, "invalid", nil
	}
	return b.answerApplication(id, option, false, AnsweredByResponder, answerer, func(r Record, scope string) error {
		if r.TypedPermission == nil || r.Key != TypedPermissionKey {
			return ErrTypedPermissionRequired
		}
		return authorize(r, scope)
	})
}

func (b *Book) AnswerLegacyApplicationAuthorized(id, option string, pending bool, authorize func() error) (Record, string, error) {
	return b.answerApplication(id, option, pending, AnsweredByOperator, "", func(r Record, _ string) error {
		if r.TypedPermission != nil {
			return ErrTypedPermissionRequired
		}
		if authorize != nil {
			return authorize()
		}
		return nil
	})
}

func permissionValidators() typedpermission.Validators {
	return typedpermission.Validators{NativeProgram: fileid.ValidSubjectProgram, RemoteProgram: remote.ValidSubjectProgram, PackagedProgramPrefix: identity.PackagedProgramPrefix}
}
