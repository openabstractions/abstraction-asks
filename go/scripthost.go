package asks

import (
	"path/filepath"

	identity "github.com/openabstractions/abstraction-identity"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

// Identity owns the script-host evidence and its reviewed lists (identity
// ID-I1, TD83); this package keeps the prompt sentence (ASK-A8) and the
// answerer rule that consumes the evidence (ASK-R7).

// ScriptHost reports whether originalName, a program's signed
// OriginalFilename as recorded with its publisher (ASK-A6), names a shared
// interpreter on identity's reviewed list. The comparison ignores case.
//
// Deprecated: use ScriptHostSubject, the one rule the prompt's sentence,
// level and icon read (ASK-A8). ScriptHost is removed in the asks
// application window (W2-02a; vocabulary D30).
func ScriptHost(originalName string) bool {
	return identity.SharedInterpreterName(originalName)
}

// ScriptHostSentence is the one sentence a question adds beside its trusted
// publisher when its recorded program is a shared interpreter (ASK-A8), or
// "" otherwise: the same ScriptHostSubject rule the prompt's level and icon
// read, so a program that runs foreign code without being a shared
// interpreter, such as stock OpenCode, gets none. The host is named by the
// recorded description, else its original file name, else the program's
// file name.
func ScriptHostSentence(program string, publisher Publisher) string {
	if !publisher.Trusted || !ScriptHostSubject(rwire.Subject{Program: program}, publisher) {
		return ""
	}
	host := publisher.Description
	if host == "" {
		host = publisher.OriginalName
	}
	if host == "" {
		host = filepath.Base(program)
	}
	return host + " runs scripts; its publisher covers " + host + " and the script it runs is unverified."
}

// ScriptHostFileName reports whether a program's file name, or its path with
// either separator, names a shared interpreter on identity's reviewed list
// on any platform (identity.SharedInterpreterFileName).
func ScriptHostFileName(program string) bool {
	return identity.SharedInterpreterFileName(program)
}

// ScriptHostPackage reports whether an MSIX package family name,
// <Name>_<PublisherId>, is a reviewed shared interpreter's
// (identity.SharedInterpreterPackage).
func ScriptHostPackage(family string) bool {
	return identity.SharedInterpreterPackage(family)
}

// ScriptHostSubject is the one shared-interpreter rule for a decision subject
// (ASK-A8, ASK-R7; rights RIGHTS-R22's default exclusion): identity's
// SharedInterpreter for the program the subject names, with evidence, the
// publisher a receiving peer or a question recorded, as its code. Such a
// program never answers, never enrolls and receives no account-wide
// default. A program that runs foreign code without being a shared
// interpreter, such as stock OpenCode, keeps its defaults; the answerer
// rule refuses it (AnswererRule, ASK-R7).
func ScriptHostSubject(subject rwire.Subject, evidence Publisher) bool {
	return identity.SharedInterpreterProgram(subject.Program, evidence.code())
}

// code is the publisher evidence as identity reads it: its original file
// name counts only beside a trusted signature.
func (p Publisher) code() identity.Code {
	return identity.Code{Subject: p.Subject, Trusted: p.Trusted, Description: p.Description, OriginalName: p.OriginalName}
}
