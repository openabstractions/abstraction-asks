package asks

import (
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
)

type PermissionTarget = wire.PermissionTarget
type PermissionRequest = wire.PermissionRequest

const PermissionContentWrite = "abstraction.storage/content.write"
const PermissionContentRead = "abstraction.storage/content.read"

// PairedPermission is the existing closed input/output first-use effect.
// Every secondary target is uniquely determined by this primary tuple.
func PairedPermission(action, resource string) bool {
	return action == PermissionContentWrite && slices.Contains([]string{"input:image", "input:audio", "output:image", "output:speech", "output:video"}, resource)
}

func permissionString(s string, max int) bool {
	return s != "" && len(s) <= max && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}

// NormalizePermissionTarget uses identity's subject spelling, shared by rights.
// Normalization happens at trusted admission and delegation creation, never
// by changing a persisted target during a read.
func NormalizePermissionTarget(t PermissionTarget) (PermissionTarget, error) {
	if !permissionString(t.Account, 128) || !permissionString(t.Program, 4096) ||
		!permissionString(t.Action, 128) || strings.ContainsAny(t.Action, " \t") || !permissionString(t.Resource, 1024) {
		return PermissionTarget{}, errors.New("asks: invalid permission target")
	}
	t.Program = identity.NormalizeSubjectProgram(t.Program)
	if !ValidPermissionTarget(t) || identity.UnresolvedShortName(t.Program) {
		return PermissionTarget{}, errors.New("asks: invalid permission program")
	}
	return t, nil
}

func ValidPermissionTarget(t PermissionTarget) bool {
	return permissionString(t.Account, 128) && permissionString(t.Program, 4096) && identity.ValidSubjectProgram(t.Program) &&
		permissionString(t.Action, 128) && !strings.ContainsAny(t.Action, " \t") && permissionString(t.Resource, 1024)
}

// NewPermissionRequest constructs the complete existing effect from its primary.
func NewPermissionRequest(t PermissionTarget) (PermissionRequest, error) {
	t, err := NormalizePermissionTarget(t)
	if err != nil {
		return PermissionRequest{}, err
	}
	p := PermissionRequest{Targets: []PermissionTarget{t}}
	if PairedPermission(t.Action, t.Resource) {
		paired := t
		paired.Action = PermissionContentRead
		p.Targets = append(p.Targets, paired)
	}
	return p, nil
}

// ValidPermissionRequest checks persisted syntax and the uniquely fixed pair.
// It performs no filesystem normalization and accepts no independent secondary.
func ValidPermissionRequest(p PermissionRequest) bool {
	if len(p.Targets) < 1 || len(p.Targets) > 2 || !ValidPermissionTarget(p.Targets[0]) {
		return false
	}
	primary := p.Targets[0]
	paired := PairedPermission(primary.Action, primary.Resource)
	if paired != (len(p.Targets) == 2) {
		return false
	}
	if paired {
		want := primary
		want.Action = PermissionContentRead
		if p.Targets[1] != want {
			return false
		}
	}
	return true
}

// PermissionForRecord reads trusted occurrence data. Only records without the
// structured field take the explicit legacy English reader. Invalid structured
// data never falls back to presentation text.
func PermissionForRecord(account string, r Record) (PermissionRequest, bool) {
	if r.Key != FirstUseKey {
		return PermissionRequest{}, false
	}
	if r.Permission != nil {
		if !ValidPermissionRequest(*r.Permission) || r.Permission.Targets[0].Account != account {
			return PermissionRequest{}, false
		}
		p := *r.Permission
		p.Targets = slices.Clone(p.Targets)
		return p, true
	}
	if !r.legacyPermission {
		return PermissionRequest{}, false
	}
	program, action, resource, ok := FirstUse(r.Key, r.About, r.Text)
	if !ok {
		return PermissionRequest{}, false
	}
	// Historical tuples retain the program spelling used by their exact
	// delegation resource. Validate stored syntax without re-resolving the file.
	t := PermissionTarget{Account: account, Program: program, Action: action, Resource: resource}
	if !ValidPermissionTarget(t) {
		return PermissionRequest{}, false
	}
	p := PermissionRequest{Targets: []PermissionTarget{t}}
	if PairedPermission(action, resource) {
		paired := t
		paired.Action = PermissionContentRead
		p.Targets = append(p.Targets, paired)
	}
	return p, true
}

// LegacyPermission reads an explicitly selected old wire record. It is for
// compatibility with an older service that refused ReadQuestionAsker, never
// a fallback for missing or malformed structured data from a current service.
func LegacyPermission(account, key, about, text string) (PermissionRequest, bool) {
	return PermissionForRecord(account, Record{Key: key, About: about, Text: text, legacyPermission: true})
}
