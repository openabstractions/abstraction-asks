package asks

import (
	"path/filepath"
	"strings"
	"sync"

	identity "github.com/openabstractions/abstraction-identity"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

// scriptHosts is the reviewed list of programs that run scripts, keyed on the
// signed OriginalFilename of their version resource, lower-cased (VISION
// 2026-10-01). This file owns the list; CONTRACT.md (ASK-A8) and VISION cite
// it. A signature on one of them covers the host and says nothing
// about the script it was given. Spellings were read from this machine's
// files on 2026-10-01 where present (feedback/prompt-program-names-2026-10-01.md);
// the case does not matter. PowerShell 7's pwsh.exe is a .NET application
// host that carries its assembly's resource, pwsh.dll, and both spellings are
// listed.
var scriptHosts = map[string]bool{
	"powershell.exe": true, // Windows PowerShell
	"pwsh.exe":       true, // PowerShell 7
	"pwsh.dll":       true, // PowerShell 7's application host
	"cmd.exe":        true, // Windows Command Processor
	"wscript.exe":    true, // Windows Script Host
	"cscript.exe":    true, // Windows Script Host, console
	"mshta.exe":      true, // HTML Application host
	"rundll32.exe":   true, // runs an export of any library
	"regsvr32.exe":   true, // runs a library's registration code, scriptlets included
	"msbuild.exe":    true, // runs inline tasks in a project file
	"wsl.exe":        true, // Windows Subsystem for Linux
	"bash.exe":       true, // WSL's bash launcher
	"python.exe":     true,
	"pythonw.exe":    true,
	"node.exe":       true,
	"bun.exe":        true,
	// Git for Windows runs hooks and "!" aliases, and its bin\bash.exe
	// launcher carries git.exe's resource ("Git for Windows", git.exe).
	"git.exe": true,
}

// ScriptHost reports whether originalName, a program's signed
// OriginalFilename as recorded with its publisher (ASK-A6), names a reviewed
// script host. The comparison ignores case.
func ScriptHost(originalName string) bool {
	return scriptHosts[strings.ToLower(originalName)]
}

// ScriptHostSentence is the one sentence a question from a script host adds
// beside its publisher (ASK-A8), or "" for any other program. The host is
// named by its signed description, or by its original file name when the
// resource carries no description.
func ScriptHostSentence(description, originalName string) string {
	if !ScriptHost(originalName) {
		return ""
	}
	host := description
	if host == "" {
		host = originalName
	}
	return host + " runs scripts; its publisher covers " + host + " and the script it runs is unverified."
}

// scriptHostFileNames is the reviewed list of script hosts by file name, the
// form macOS and Linux programs carry since they have no signed
// OriginalFilename resource. A name is compared after ScriptHostFileName
// lower-cases it and removes ".exe" or ".dll" and a trailing version, so python3,
// python3.12, perl5.36 and Python.app's Contents/MacOS/Python all read as
// their stem. Every Windows entry of scriptHosts is listed here by stem too.
var scriptHostFileNames = map[string]bool{
	// Windows names of scriptHosts, without ".exe" or ".dll".
	"powershell": true, "pwsh": true, "cmd": true, "wscript": true, "cscript": true,
	"mshta": true, "rundll32": true, "regsvr32": true, "msbuild": true, "wsl": true, "git": true,
	// Shells.
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "mksh": true,
	"csh": true, "tcsh": true, "fish": true, "ash": true, "busybox": true,
	// macOS Open Scripting Architecture.
	"osascript": true,
	// Language runtimes.
	"python": true, "pythonw": true, "pypy": true, "node": true, "nodejs": true, "bun": true,
	"deno": true, "perl": true, "ruby": true, "php": true, "lua": true, "luajit": true,
	"tclsh": true, "wish": true, "expect": true, "java": true, "javaw": true, "jshell": true,
	"dotnet": true, "julia": true, "rscript": true,
	"awk": true, "gawk": true, "mawk": true, "nawk": true,
}

// ScriptHostFileName reports whether a program's file name, or its path with
// either separator, names a reviewed script host on any platform: the stem
// that remains after lower-casing, removing ".exe" or ".dll" and removing trailing
// version digits and dots (python3.12, perl5.36, node18) is on the list,
// or the name before that removal is (rundll32, regsvr32).
// It is a pure function of the name, so every platform's names test anywhere.
func ScriptHostFileName(program string) bool {
	base := program
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	base = strings.ToLower(base)
	base = strings.TrimSuffix(strings.TrimSuffix(base, ".exe"), ".dll")
	return scriptHostFileNames[base] || scriptHostFileNames[strings.TrimRight(base, "0123456789.")]
}

// scriptHostPackages are the reviewed script hosts Windows runs as MSIX
// packages, whose decision subject is msix:<family> (identity
// SubjectProgram), keyed on the package name before the family's publisher
// id as its manifest spells it, which identity reports unchanged.
// scriptHostPackagePrefixes covers families that carry a version in the name
// (PythonSoftwareFoundation.Python.3.12).
var scriptHostPackages = map[string]bool{
	"Microsoft.PowerShell":                            true, // PowerShell 7 from the Microsoft Store
	"Microsoft.PowerShellPreview":                     true,
	"MicrosoftCorporationII.WindowsSubsystemForLinux": true, // WSL
	"TheDebianProject.DebianGNULinux":                 true, // a WSL distribution's shell
}

var scriptHostPackagePrefixes = []string{
	"PythonSoftwareFoundation.Python.", // Python from the Microsoft Store
	"CanonicalGroupLimited.Ubuntu",     // Ubuntu for WSL, every release
}

// ScriptHostPackage reports whether an MSIX package family name,
// <Name>_<PublisherId>, is a reviewed script host's.
func ScriptHostPackage(family string) bool {
	name, _, _ := strings.Cut(family, "_")
	if scriptHostPackages[name] {
		return true
	}
	for _, prefix := range scriptHostPackagePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// signedHosts caches, by file identity (identity.ProgramKey), whether a
// program file's own signed version resource names a reviewed script host.
// A replaced file has another identity and is read again.
var signedHosts = struct {
	mu   sync.Mutex
	seen map[string]bool
}{seen: map[string]bool{}}

// maxSignedHosts bounds the cache; a full cache starts over.
const maxSignedHosts = 512

// verifyProgramFile reads a program file's signature; tests replace it.
var verifyProgramFile = identity.VerifyFile

// signedScriptHostFile reports whether the file at program is signed and its
// trusted OriginalFilename names a reviewed script host, so a renamed signed
// copy is still caught. A missing file, a non-local path and a platform
// without file verification read as no evidence.
func signedScriptHostFile(program string) bool {
	if !filepath.IsAbs(program) || identity.LocalPath(program) != nil {
		return false
	}
	key := identity.ProgramKey(program)
	if key == "" {
		return false
	}
	signedHosts.mu.Lock()
	host, ok := signedHosts.seen[key]
	signedHosts.mu.Unlock()
	if ok {
		return host
	}
	code, _, err := verifyProgramFile(program, nil)
	if err != nil {
		// A failed read is no evidence and is not remembered: the next
		// check reads the file again.
		return false
	}
	host = code.Trusted && ScriptHost(code.OriginalName)
	signedHosts.mu.Lock()
	if len(signedHosts.seen) >= maxSignedHosts {
		signedHosts.seen = map[string]bool{}
	}
	signedHosts.seen[key] = host
	signedHosts.mu.Unlock()
	return host
}

// ScriptHostSubject is the one script-host rule (ASK-A8, ASK-R7; rights
// RIGHTS-R22's default exclusion): whether the program a decision subject
// names is a reviewed script host, whose every script would act as it. It
// reads, in order:
//   - an msix:<family> subject by its package family (ScriptHostPackage);
//   - evidence, the publisher a receiving peer or a question recorded, by its
//     trusted OriginalFilename;
//   - the program's own name, canonicalized first so a short 8.3 alias reads
//     as its long name (ScriptHostFileName);
//   - the program file's own signature, verified once per file identity, so a
//     renamed signed copy is caught by its OriginalFilename.
//
// An answerer enrollment, every answer and every account-wide default refuse
// such a program. This is a boundary between cooperating programs of one
// account (RIGHTS-R15): an unsigned renamed copy of a host passes.
func ScriptHostSubject(subject rwire.Subject, evidence Publisher) bool {
	program := subject.Program
	if family, ok := strings.CutPrefix(program, identity.PackagedProgramPrefix); ok {
		return ScriptHostPackage(family)
	}
	if evidence.Trusted && ScriptHost(evidence.OriginalName) {
		return true
	}
	if ScriptHostFileName(program) || ScriptHostFileName(identity.CanonicalProgramPath(program)) {
		return true
	}
	return signedScriptHostFile(program)
}
