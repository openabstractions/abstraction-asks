package typedpermission

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	rightswire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
	"github.com/openabstractions/abstraction-storage/go/digest"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const Key = "rights.permission@2"
const MaxPermissionSelections = 32
const MaxPermissionTargetBytes = 4096

type Permission struct {
	Effect  rightswire.PermissionEffect `json:"effect"`
	Digest  string                      `json:"digest"`
	Origin  string                      `json:"origin"`
	Purpose string                      `json:"purpose"`
	Display wire.PermissionDisplay      `json:"display"`
}

func ValidPermission(p Permission, validators Validators) bool {
	if validators.NativeProgram == nil || validators.RemoteProgram == nil {
		return false
	}
	e := p.Effect
	if !permissionString(e.Subject.Account, 128) || !permissionString(e.Subject.Program, 4096) ||
		!(validators.NativeProgram(e.Subject.Program) || validators.RemoteProgram(e.Subject.Program)) ||
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
		if !permissionString(b.BundleRef, 128) || !strings.HasPrefix(b.DefinitionDigest, "sha256:") || !digest.Valid(b.DefinitionDigest) || len(b.Selections) != len(e.Selections) {
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

func ClonePermission(p Permission) Permission {
	raw, _ := json.Marshal(p)
	var out Permission
	_ = json.Unmarshal(raw, &out)
	return out
}

func operatorWord(s string) bool {
	return len(s) > 0 && len(s) <= 128 && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}

func validOperatorRecord(r wire.RecordMetadata) bool {
	if !operatorWord(r.ID) || r.Asker == "" || r.Key == "" || r.Text == "" || len(r.Options) == 0 {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, r.Asked); err != nil {
		return false
	}
	if r.Option == "" {
		return r.Answered == "" && !r.Yes && !r.Kept
	}
	if _, err := time.Parse(time.RFC3339Nano, r.Answered); err != nil {
		return false
	}
	for _, option := range r.Options {
		if r.Option == option {
			return true
		}
	}
	return false
}

func validateQuestionRead(result wire.OperatorQuestion, id string) error {
	found := result.Outcome == wire.OperatorQuestionOutcomeFound
	program := result.ProgramDescription != "" || result.ProgramOriginalName != ""
	if found != (result.Record != nil) || !found && (result.Publisher != "" || result.PublisherTrusted || result.AnsweredBy != nil || program) ||
		program && !result.PublisherTrusted ||
		found && result.Record.Option == "" && result.AnsweredBy != nil || result.AnsweredBy != nil && !result.AnsweredBy.Known() {
		return errors.New("asks: malformed question read")
	}
	if r := result.Record; r != nil && (!validOperatorRecord(*r) || r.ID != id) {
		return errors.New("asks: mismatched question read")
	}
	return nil
}

func CheckDetails(v wire.TypedPermissionDetails, account, id string, validators Validators) (wire.TypedPermissionDetails, error) {
	if validators.RemoteProgram == nil || validators.NativeProgram == nil {
		return wire.TypedPermissionDetails{}, errors.New("asks: missing native permission validator")
	}
	q := v.Question.Question
	if err := validateQuestionRead(q, id); err != nil {
		return wire.TypedPermissionDetails{}, err
	}
	if q.Outcome != wire.OperatorQuestionOutcomeFound {
		if q.Outcome != wire.OperatorQuestionOutcomeUnknown && q.Outcome != wire.OperatorQuestionOutcomeInvalid && q.Outcome != wire.OperatorQuestionOutcomeForbidden && q.Outcome != wire.OperatorQuestionOutcomeUnavailable || q.Record != nil || q.Publisher != "" || q.AnsweredBy != nil || v.Question.Answerer != "" || v.Effect != nil || v.Display != nil || v.EffectDigest != "" || v.Origin != "" || v.Purpose != "" {
			return wire.TypedPermissionDetails{}, errors.New("asks: malformed typed details refusal")
		}
		return v, nil
	}
	if q.Record == nil || !validPermissionRecord(*q.Record) || q.Record.ID != id || v.Effect == nil || v.Display == nil || v.Effect.Subject.Account != account || q.Record.About != v.Effect.Subject.Program || q.Record.Option == "" && q.AnsweredBy != nil || q.Record.Option != "" && (q.AnsweredBy == nil || *q.AnsweredBy != wire.AnswererOperator && *q.AnsweredBy != wire.AnswererResponder) {
		return wire.TypedPermissionDetails{}, errors.New("asks: malformed or foreign typed details")
	}
	responder := q.AnsweredBy != nil && *q.AnsweredBy == wire.AnswererResponder
	if slices.Equal(q.Record.Options, Options(true)) && (v.Origin != "service" || responder) {
		return wire.TypedPermissionDetails{}, errors.New("asks: one-call permission requires the service and person")
	}
	if responder && (len(v.Question.Answerer) > 4096 || !validators.NativeProgram(v.Question.Answerer)) || !responder && v.Question.Answerer != "" {
		return wire.TypedPermissionDetails{}, errors.New("asks: invalid typed answer actor")
	}
	p := Permission{Effect: *v.Effect, Digest: v.EffectDigest, Origin: v.Origin, Purpose: v.Purpose, Display: *v.Display}
	if !ValidPermission(p, validators) {
		return wire.TypedPermissionDetails{}, errors.New("asks: invalid frozen permission effect")
	}
	return ClonePermissionDetails(v), nil
}
func validPermissionRecord(r wire.RecordMetadata) bool {
	return validOperatorRecord(r) && r.Key == Key && r.Text == "OA permission request" &&
		(slices.Equal(r.Options, Options(false)) || slices.Equal(r.Options, Options(true))) && !r.Kept &&
		(r.Option == "" && !r.Yes || (r.Option == "allow" || r.Option == "once") && r.Yes || r.Option == "refuse" && !r.Yes)
}

// Options is the closed choice set for a typed permission occurrence. The
// service-only variant binds a third choice to this exact call.
func Options(once bool) []string {
	if once {
		return []string{"once", "allow", "refuse"}
	}
	return []string{"allow", "refuse"}
}
func ClonePermissionDetails(v wire.TypedPermissionDetails) wire.TypedPermissionDetails {
	raw, _ := json.Marshal(v)
	var out wire.TypedPermissionDetails
	_ = json.Unmarshal(raw, &out)
	return out
}

type Validators struct {
	PackagedProgramPrefix string
	RemoteProgram         func(string) bool
	NativeProgram         func(string) bool
}
type PermissionDetailsReader interface {
	ReadPermissionDetailsContext(context.Context, string) (wire.TypedPermissionDetails, error)
}

// ReadPermissionDetailsContext verifies authority display at the presenter boundary.
// Native syntax remains with the supplied existing Identity owner; nil refuses.
func ReadPermissionDetailsContext(ctx context.Context, r PermissionDetailsReader, account, id string, validators Validators) (wire.TypedPermissionDetails, error) {
	v, err := r.ReadPermissionDetailsContext(ctx, id)
	if err != nil {
		return wire.TypedPermissionDetails{}, err
	}
	return CheckDetails(v, account, id, validators)
}
func permissionString(s string, max int) bool {
	return s != "" && len(s) <= max && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}
