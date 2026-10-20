package asks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

var errNativeApplicationCollision = errors.New("asks: conflicting native and legacy occurrences")

type nativeApplicationOwner struct {
	Subject    identity.Subject `json:"subject"`
	Executable string           `json:"executable"`
}

// NativeApplicationBook is an immutable native-subject view over one Book.
// It shares that Book's state, bounds, transaction and observation lifetime.
type NativeApplicationBook struct {
	book  *Book
	owner *nativeApplicationOwner
}

// BindNativeSubject binds trusted host composition inputs to this Book.
// These values are not authentication or proof. A receiving host independently
// derives SubjectOf and the observed native executable; neither comes from a
// wire request. Existing Book methods retain their legacy scope semantics.
func (b *Book) BindNativeSubject(subject identity.Subject, observedExecutable string) (*NativeApplicationBook, error) {
	owner := &nativeApplicationOwner{subject, filepath.Clean(observedExecutable)}
	if b == nil || !b.application || !owner.valid() || subject.Account != b.owner {
		return nil, errors.New("asks: valid native subject and application Book required")
	}
	return &NativeApplicationBook{b, owner}, nil
}

func (n *nativeApplicationOwner) valid() bool {
	return n != nil && (n.Subject.AccountKind == "windows" || n.Subject.AccountKind == "posix") && n.Subject.Account != "" && len(n.Subject.Account) <= 1024 && utf8.ValidString(n.Subject.Account) && validAnswerer(n.Subject.Program) && !strings.HasPrefix(n.Subject.Program, "remote:") && validAskerProgram(n.Executable)
}
func nativeApplicationScope(profile string, values ...string) string {
	data, _ := json.Marshal(append([]string{profile}, values...))
	sum := sha256.Sum256(data)
	return profile + ":" + hex.EncodeToString(sum[:])
}

// ApplicationOwnerProgramScope derives the application admission scope from
// trusted receiving account and observed executable evidence. The executable
// is the platform-proved path, independent of the native subject's Program.
func ApplicationOwnerProgramScope(accountKind, account, observedExecutable string) string {
	return nativeApplicationScope("asks-owner-program@1", accountKind, account, filepath.Clean(observedExecutable))
}
func (n *nativeApplicationOwner) scope() string {
	return nativeApplicationScope("asks-native-program@1", n.Subject.AccountKind, n.Subject.Account, n.Subject.Program)
}
func (n *nativeApplicationOwner) legacy() string {
	return ApplicationOwnerProgramScope(n.Subject.AccountKind, n.Subject.Account, n.Executable)
}
func (n *nativeApplicationOwner) recorded(scope string) *nativeApplicationOwner {
	if n == nil || scope != n.scope() {
		return nil
	}
	copy := *n
	return &copy
}

func (n *nativeApplicationOwner) programFingerprint(scope string, r *Record) string {
	if n != nil && scope == n.scope() {
		return programHash(n.Subject.Program)
	}
	return recordProgramHash(r)
}

func (n *nativeApplicationOwner) recordShare(scope string, r *Record) string {
	if n != nil && scope == n.scope() {
		return programShare(n.Subject.Program, scope)
	}
	return recordShare(r, scope)
}

func (f *book) nativeApplicationShareFull(n *nativeApplicationOwner, scope string, r *Record, newRecord bool) bool {
	if n == nil {
		return f.applicationShareFull(recordShare(r, scope), newRecord)
	}
	share := n.recordShare(scope, r)
	ids := map[string]bool{}
	admissions := 0
	for _, a := range f.Admissions {
		if shareKey(a.ProgramHash, a.Scope) == share || a.Scope == n.legacy() {
			admissions++
			ids[a.ID] = true
		}
	}
	records := 0
	for _, record := range f.Asks {
		if ids[record.ID] {
			records++
		}
	}
	return admissions >= programApplicationShare || newRecord && records >= programApplicationShare
}
func (n *nativeApplicationOwner) sameSubject(other *nativeApplicationOwner) bool {
	return n != nil && other != nil && n.Subject == other.Subject
}

// owns retains exact-current-executable compatibility without assigning native
// provenance to an older row. The captured subject also spans new native rows.
func (n *nativeApplicationOwner) owns(scope string, owner *nativeApplicationOwner) bool {
	return n != nil && (scope == n.scope() && n.sameSubject(owner) || scope == n.legacy() && owner == nil)
}

func nativeNotificationOwner(n *nativeApplicationOwner, selectedScope string, candidate notificationAdmission) bool {
	if n == nil {
		return candidate.Scope == selectedScope
	}
	return n.owns(candidate.Scope, candidate.NativeOwner)
}

// quota ownership counts saved native provenance for an exact retained scope.
// It confers no authority to enumerate or change the candidate.
func nativeNotificationQuotaOwner(n *nativeApplicationOwner, selectedScope string, candidate notificationAdmission) bool {
	return nativeNotificationOwner(n, selectedScope, candidate) ||
		n == nil && candidate.NativeOwner != nil && candidate.NativeOwner.owns(selectedScope, nil)
}

func (f *book) validateNativeOwners(enabled bool) error {
	check := func(scope string, n *nativeApplicationOwner) error {
		if n == nil {
			return nil
		}
		if !enabled || !n.valid() || n.scope() != scope {
			return errors.New("asks: invalid native ownership provenance")
		}
		return nil
	}
	for _, a := range f.Admissions {
		if e := check(a.Scope, a.NativeOwner); e != nil {
			return e
		}
		if a.NativeOwner != nil && a.ProgramHash != programHash(a.NativeOwner.Subject.Program) {
			return errors.New("asks: invalid native program fingerprint")
		}
	}
	for _, a := range f.Choices {
		if e := check(a.Scope, a.NativeOwner); e != nil {
			return e
		}
		if a.NativeOwner != nil && a.Record == nil && a.ProgramHash != programHash(a.NativeOwner.Subject.Program) {
			return errors.New("asks: invalid native choice fingerprint")
		}
	}
	for _, a := range f.Notifications {
		if e := check(a.Scope, a.NativeOwner); e != nil {
			return e
		}
	}
	return nil
}

// nativeScope runs under the same read snapshot or CAS transaction as its
// consumer. An old row is reachable only through this exact executable. It is
// never relabeled with present-day package provenance.
func (f *book) nativeScope(n *nativeApplicationOwner, legacyScope, partition string, key any) (string, error) {
	if n == nil {
		return legacyScope, nil
	}
	selected, id := "", ""
	consider := func(scope, recordID string, owner *nativeApplicationOwner) error {
		if scope == n.scope() && !n.sameSubject(owner) {
			return errNativeApplicationCollision
		}
		candidate := n.owns(scope, owner)
		if !candidate {
			return nil
		}
		if id != "" && (id != recordID || selected != scope) {
			return errNativeApplicationCollision
		}
		selected, id = scope, recordID
		return nil
	}
	switch partition {
	case "application":
		for _, a := range f.Admissions {
			if a.Key == key.(string) {
				if e := consider(a.Scope, a.ID, a.NativeOwner); e != nil {
					return "", e
				}
			}
		}
	case "choice":
		for _, a := range f.Choices {
			if a.Reference == key.(wire.RequestRef) {
				if e := consider(a.Scope, a.ID, a.NativeOwner); e != nil {
					return "", e
				}
			}
		}
	case "notification":
		for _, a := range f.Notifications {
			if a.Reference == key.(wire.RequestRef) {
				if e := consider(a.Scope, a.ID, a.NativeOwner); e != nil {
					return "", e
				}
			}
		}
	}
	if selected != "" {
		return selected, nil
	}
	return n.scope(), nil
}

func (v *NativeApplicationBook) AskApplicationAs(program, requestKey, key string, slots map[string]string, via listen.Seen, publisher Publisher) (Record, string, error) {
	if !validAskerProgram(program) {
		return Record{}, "invalid", nil
	}
	r, o, e := v.book.askApplicationFrom(v.owner, v.owner.scope(), AskerDisplay(program), program, requestKey, key, slots, via, publisher, nil)
	if errors.Is(e, errNativeApplicationCollision) {
		return Record{}, "conflict", nil
	}
	return r, o, e
}
func (v *NativeApplicationBook) AskChoiceAs(program string, q wire.ChoiceRequest, shared []string, via listen.Seen, publisher Publisher, authorize func() error) (wire.ChoiceObservation, error) {
	if !validAnswerer(program) || shared != nil && !ValidAssistantOptions(q, shared) {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeInvalid}, nil
	}
	display := AskerDisplay(program)
	r, e := v.book.askChoice(v.owner, v.owner.scope(), display, program, q, slices.Clone(shared), via, publisher, authorize)
	if errors.Is(e, errNativeApplicationCollision) {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeConflict}, nil
	}
	return r, e
}
