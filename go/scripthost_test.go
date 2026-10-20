package asks

import (
	"path/filepath"
	"testing"

	"github.com/openabstractions/abstraction-identity/hostfixture"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

// TestScriptHostSentence: a question whose recorded program is a shared
// interpreter, with a trusted publisher, adds one sentence naming the host by
// its signed description; any other program adds none (ASK-A8). The rule is
// ScriptHostSubject's, so the trusted OriginalFilename (compared without
// case) or the program's own name decides, and stock OpenCode, a Bun
// standalone executable that keeps Bun's resource, adds none.
func TestScriptHostSentence(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "renamed.exe")
	for _, tc := range []struct {
		program                   string
		description, originalName string
		trusted                   bool
		want                      string
	}{
		{app, "Windows PowerShell", "PowerShell.EXE", true,
			"Windows PowerShell runs scripts; its publisher covers Windows PowerShell and the script it runs is unverified."},
		{app, "Windows Command Processor", "Cmd.Exe", true,
			"Windows Command Processor runs scripts; its publisher covers Windows Command Processor and the script it runs is unverified."},
		{app, "", "node.exe", true, "node.exe runs scripts; its publisher covers node.exe and the script it runs is unverified."},
		{app, "Git for Windows", "git.exe", true,
			"Git for Windows runs scripts; its publisher covers Git for Windows and the script it runs is unverified."},
		{filepath.Join(dir, "python.exe"), "", "", true, "python.exe runs scripts; its publisher covers python.exe and the script it runs is unverified."},
		{app, "Windows PowerShell", "PowerShell.EXE", false, ""},
		{app, "Notepad", "NOTEPAD.EXE", true, ""},
		{app, "Windows PowerShell", "", true, ""},
		{app, "", "", true, ""},
		{app, "Windows PowerShell", "PowerShell.EXE.MUI", true, ""},
		{hostfixture.Bun(t, dir, "opencode.exe", true), "Bun", "bun.exe", true, ""},
	} {
		publisher := Publisher{Subject: "Publisher", Trusted: tc.trusted, Description: tc.description, OriginalName: tc.originalName}
		if got := ScriptHostSentence(tc.program, publisher); got != tc.want {
			t.Errorf("ScriptHostSentence(%q, %+v) = %q, want %q", tc.program, publisher, got, tc.want)
		}
	}
	for _, name := range []string{"PowerShell.EXE", "pwsh.exe", "pwsh.dll", "Cmd.Exe", "wscript.exe", "cscript.exe", "MSHTA.EXE",
		"RUNDLL32.EXE", "REGSVR32.EXE", "MSBuild.exe", "wsl.exe", "bash.exe", "python.exe", "pythonw.exe", "node.exe", "bun.exe", "git.exe"} {
		if !ScriptHost(name) {
			t.Errorf("%s must be a script host", name)
		}
	}
}

// scriptHostNameCases checks ScriptHostFileName and ScriptHostSubject with no
// publisher evidence over one platform's names, so each group runs anywhere.
func scriptHostNameCases(t *testing.T, cases map[string]bool) {
	t.Helper()
	for program, host := range cases {
		if got := ScriptHostFileName(program); got != host {
			t.Errorf("ScriptHostFileName(%q) = %t, want %t", program, got, host)
		}
		if got := ScriptHostSubject(rwire.Subject{Program: program}, Publisher{}); got != host {
			t.Errorf("ScriptHostSubject(%q) = %t, want %t", program, got, host)
		}
	}
}

// TestScriptHostNamesWindows: Windows file names, any case, a versioned
// python3.12.exe included (ASK-R7).
func TestScriptHostNamesWindows(t *testing.T) {
	scriptHostNameCases(t, map[string]bool{
		`C:\Windows\System32\WindowsPowerShell\v1.0\PowerShell.EXE`: true, `C:\Windows\System32\cmd.exe`: true,
		`C:\Python312\python.exe`: true, `C:\Python312\pythonw.exe`: true, `C:\py\python3.12.exe`: true,
		`C:\nodejs\node.exe`: true, `C:\Program Files\Git\bin\bash.exe`: true, `mshta.exe`: true,
		`C:\Program Files\dotnet\dotnet.exe`: true, `C:\jdk\bin\javaw.exe`: true, `C:\Julia-1.11\bin\julia.exe`: true,
		`C:\R\R-4.4.1\bin\Rscript.exe`: true, `C:\Program Files\PowerShell\7\pwsh.dll`: true,
		`C:\Program Files\Editor\editor.exe`: false, `C:\Windows\notepad.exe`: false, "": false,
	})
}

// TestScriptHostNamesMac: macOS shells, osascript, interpreters and
// Python.app's bundle executable (ASK-R7).
func TestScriptHostNamesMac(t *testing.T) {
	scriptHostNameCases(t, map[string]bool{
		"/bin/zsh": true, "/bin/sh": true, "/bin/bash": true, "/bin/ksh": true, "/usr/bin/osascript": true,
		"/usr/bin/python3": true, "/opt/homebrew/bin/python3.12": true,
		"/Library/Frameworks/Python.framework/Versions/3.12/Resources/Python.app/Contents/MacOS/Python": true,
		"/usr/bin/perl5.34": true, "/usr/bin/ruby": true, "/opt/homebrew/bin/node": true,
		"/Users/a/.bun/bin/bun": true, "/opt/homebrew/bin/deno": true, "/opt/homebrew/bin/fish": true,
		"/Applications/OpenCode.app/Contents/MacOS/opencode":     false,
		"/Applications/Calculator.app/Contents/MacOS/Calculator": false,
	})
}

// TestScriptHostNamesLinux: Linux shells and versioned interpreters (ASK-R7).
func TestScriptHostNamesLinux(t *testing.T) {
	scriptHostNameCases(t, map[string]bool{
		"/bin/dash": true, "/usr/bin/bash": true, "/usr/bin/fish": true, "/bin/busybox": true,
		"/usr/bin/python3.11": true, "/usr/bin/perl5.36.0": true, "/usr/bin/php8.2": true,
		"/usr/bin/lua5.4": true, "/usr/bin/nodejs": true, "/usr/bin/tclsh8.6": true, "/usr/bin/gawk": true,
		"/usr/bin/dotnet": true, "/usr/local/bin/julia": true, "/usr/bin/Rscript": true,
		"/usr/bin/opencode": false, "/usr/bin/vim": false, "/usr/bin/shellcheck": false,
	})
}
