package asks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
	rightswire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

const TypedPermissionKey = "rights.permission@2"
const MaxPermissionSelections = 32
const MaxPermissionTargetBytes = 4096

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
	e := p.Effect
	if !permissionString(e.Subject.Account, 128) || !permissionString(e.Subject.Program, 4096) ||
		!ValidPermissionTarget(PermissionTarget{Account: e.Subject.Account, Program: e.Subject.Program, Action: "abstraction.asks/typed", Resource: "syntax"}) ||
		len(e.Selections) < 1 || len(e.Selections) > MaxPermissionSelections || (p.Origin != "application" && p.Origin != "service") ||
		p.Purpose != "" && !permissionString(p.Purpose, 800) || len(p.Display.Selections) != len(e.Selections) {
		return false
	}
	for i, d := range p.Display.Selections {
		if !permissionString(d.Operation, 512) || !permissionString(d.Target, 1024) || d.Collection != "" && !permissionString(d.Collection, 1024) || d.AllEffect != "" && !permissionString(d.AllEffect, 2048) || d.Disclosure != "" && !permissionString(d.Disclosure, 2048) {
			return false
		}
		if e.Selections[i].Selector.Kind == rightswire.ResourceSelectorKindAll && (d.Collection == "" || d.AllEffect == "") {
			return false
		}
	}
	if p.Display.BundleLabel != "" && !permissionString(p.Display.BundleLabel, 512) || p.Display.BundleDisclosure != "" && !permissionString(p.Display.BundleDisclosure, 2048) {
		return false
	}
	seen := map[string]bool{}
	for _, s := range e.Selections {
		if !permissionString(s.Permission.Action, 128) || !permissionString(s.Permission.ResourceProfile, 128) {
			return false
		}
		switch s.Selector.Kind {
		case rightswire.ResourceSelectorKindOne:
			if len(s.Selector.Target) == 0 || len(s.Selector.Target) > MaxPermissionTargetBytes {
				return false
			}
		case rightswire.ResourceSelectorKindAll:
			if s.Selector.Target != nil {
				return false
			}
		default:
			return false
		}
		raw, err := json.Marshal(s)
		if err != nil || seen[string(raw)] {
			return false
		}
		seen[string(raw)] = true
	}
	if b := e.Bundle; b != nil {
		if p.Display.BundleLabel == "" || p.Display.BundleDisclosure == "" {
			return false
		}
		if !permissionString(b.BundleRef, 128) || !strings.HasPrefix(b.DefinitionDigest, "sha256:") || len(b.DefinitionDigest) != 71 || len(b.Selections) != len(e.Selections) {
			return false
		}
		if digest, err := hex.DecodeString(strings.TrimPrefix(b.DefinitionDigest, "sha256:")); err != nil || len(digest) != 32 {
			return false
		}
		for i, s := range b.Selections {
			left, _ := json.Marshal(s)
			right, _ := json.Marshal(e.Selections[i])
			if string(left) != string(right) {
				return false
			}
			if s.Selector.Kind != b.Selector.Kind || !slices.Equal(s.Selector.Target, b.Selector.Target) {
				return false
			}
		}
	}
	return p.Digest == PermissionEffectDigest(e)
}

func PermissionEffectDigest(e rightswire.PermissionEffect) (digest string) {
	defer func() {
		if recover() != nil {
			digest = ""
		}
	}()
	raw := rightswire.EncodePermissionEffect(&e)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func CloneTypedPermission(p TypedPermission) TypedPermission {
	raw, _ := json.Marshal(p)
	var out TypedPermission
	_ = json.Unmarshal(raw, &out)
	return out
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
	out, outcome, err := b.askTypedPermission(scope, display, requestKey, p, via, publisher, authorize)
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

func (b *Book) askTypedPermission(scope, display, requestKey string, p TypedPermission, via listen.Seen, publisher Publisher, authorize func() error) (Record, string, error) {
	if !b.application || !ValidTypedPermission(p) || p.Effect.Subject.Account != b.owner || !ValidRequestKey(scope) || !ValidRequestKey(requestKey) || display == "" {
		return Record{}, "invalid", nil
	}
	p = CloneTypedPermission(p)
	fingerprint := typedFingerprint(p)
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
				outcome = recordOutcome(out)
				return nil
			}
		}
		// New request keys share the existing pending occurrence. A recent Refuse
		// stands for the bounded interval even if an application varies its purpose.
		for _, r := range f.Asks {
			if r.TypedPermission == nil || r.TypedPermission.Digest != p.Digest {
				continue
			}
			if r.Pending() || r.Option == "refuse" && b.clock().Before(r.Answered.Add(10*time.Minute)) {
				if len(f.Admissions) >= MaxApplicationAdmissions {
					return ErrApplicationCapacity
				}
				out = *r
				f.Admissions = append(f.Admissions, applicationAdmission{scope, requestKey, fingerprint, out.ID})
				outcome = recordOutcome(out)
				return nil
			}
		}
		if len(f.Admissions) >= MaxApplicationAdmissions || len(f.Asks) >= MaxApplicationAdmissions {
			return ErrApplicationCapacity
		}
		publisher = publisher.recorded()
		out = Record{ID: random(16), Asker: display, Key: TypedPermissionKey, About: p.Effect.Subject.Program, Text: "OA permission request", Options: []string{"allow", "refuse"}, Asked: b.clock().UTC(), Via: via, Publisher: publisher.Subject, PublisherTrusted: publisher.Trusted, ProgramDescription: publisher.Description, ProgramOriginalName: publisher.OriginalName, TypedPermission: &p}
		copy := out
		f.Asks = append(f.Asks, &copy)
		f.Admissions = append(f.Admissions, applicationAdmission{scope, requestKey, fingerprint, out.ID})
		outcome = "pending"
		return nil
	})
	return out, outcome, err
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
