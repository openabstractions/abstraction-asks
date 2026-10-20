package asks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	identity "github.com/openabstractions/abstraction-identity"
)

// AnswerAction is the rights action for answering: on one exact delegated
// first-use target and option (ASK-R3), or on the resource "apps" for every
// app's routine questions (ASK-R5).
const AnswerAction = "abstraction.asks/question.answer"

// validAnswerer reports whether program is a subject program a responder
// answer may record as its actor (ASK-O7).
func validAnswerer(program string) bool {
	return len(program) <= 4096 && utf8.ValidString(program) && identity.ValidSubjectProgram(program)
}

// FirstUseAnswerResource names one target program, action, resource and choice.
// The receiving account is implicit. The same helper is used by the operator
// setting a rule and by the answering service checking it.
func FirstUseAnswerResource(program, action, resource, option string) (string, error) {
	if option != "allow" && option != "refuse" {
		return "", errors.New("asks: invalid first-use answer option")
	}
	target, err := NormalizePermissionTarget(PermissionTarget{Account: "implicit", Program: program, Action: action, Resource: resource})
	if err != nil {
		return "", err
	}
	// Preserve every previously representable tuple's exact resource bytes.
	if name, err := legacyFirstUseAnswerResource(program, action, resource, option); err == nil {
		return name, nil
	}
	encoded, err := json.Marshal([4]string{target.Program, target.Action, target.Resource, option})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return "first-use@2:" + hex.EncodeToString(sum[:]), nil
}

func legacyFirstUseAnswerResource(program, action, resource, option string) (string, error) {
	valid := func(s string, max int) bool {
		return s != "" && len(s) <= max && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
	}
	if !valid(program, 800) || utf8.RuneCountInString(program) > maxSlot ||
		!valid(action, 128) || strings.ContainsAny(action, " \t") ||
		!valid(resource, 800) || utf8.RuneCountInString(resource) > maxSlot ||
		(option != "allow" && option != "refuse") {
		return "", errors.New("asks: invalid first-use answer scope")
	}
	encoded, err := json.Marshal([4]string{program, action, resource, option})
	if err != nil {
		return "", err
	}
	name := "first-use@1:" + string(encoded)
	if len(name) > 1024 {
		return "", errors.New("asks: first-use answer scope exceeds rights resource bound")
	}
	return name, nil
}
