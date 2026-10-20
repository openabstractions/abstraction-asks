// Package scripthostcases is the one table of script-host cases that every
// caller of asks.ScriptHostSubject runs in its own tests (asks ASK-R7, rights
// RIGHTS-R22): the defaults exclusion, the answerer refusals and the Panel's
// assistant chooser read the same rows, so a caller that bypasses the rule
// fails here.
package scripthostcases

import (
	"os"
	"path/filepath"
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
)

// Case is one program a caller decides about, with the publisher evidence a
// receiving peer would carry (zero for a caller that has none), and whether it
// is a reviewed script host.
type Case struct {
	Name     string
	Program  string
	Evidence asks.Publisher
	Host     bool
}

// Cases builds the table: msix package forms, publisher evidence, ordinary
// names, and, where this machine provides them, a renamed copy of a signed
// script host and a short 8.3 alias of a host's name. Files are made under
// t.TempDir.
func Cases(t testing.TB) []Case {
	t.Helper()
	dir := t.TempDir()
	app := filepath.Join(dir, "editor.exe")
	unsigned := filepath.Join(dir, "unsigned-helper.exe")
	for _, path := range []string{app, unsigned} {
		if err := os.WriteFile(path, []byte("not a script host"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cases := []Case{
		{Name: "msix PowerShell", Program: "msix:Microsoft.PowerShell_8wekyb3d8bbwe", Host: true},
		{Name: "msix PowerShell Preview", Program: "msix:Microsoft.PowerShellPreview_8wekyb3d8bbwe", Host: true},
		{Name: "msix Python", Program: "msix:PythonSoftwareFoundation.Python.3.12_qbz5n2kfra8p0", Host: true},
		{Name: "msix WSL", Program: "msix:MicrosoftCorporationII.WindowsSubsystemForLinux_8wekyb3d8bbwe", Host: true},
		{Name: "msix Ubuntu", Program: "msix:CanonicalGroupLimited.Ubuntu24.04LTS_79rhkp1fndgsc", Host: true},
		{Name: "msix Calculator", Program: "msix:Microsoft.WindowsCalculator_8wekyb3d8bbwe", Host: false},
		{Name: "python by name", Program: filepath.Join(dir, "python.exe"), Host: true},
		{Name: "node by name", Program: filepath.Join(dir, "node.exe"), Host: true},
		{Name: "an app", Program: app, Host: false},
		{Name: "an unsigned renamed copy passes", Program: unsigned, Host: false},
		{Name: "evidence names PowerShell 7", Program: filepath.Join(dir, "renamed.exe"),
			Evidence: asks.Publisher{Subject: "Microsoft Corporation", Trusted: true, OriginalName: "pwsh.dll"}, Host: true},
		{Name: "untrusted evidence names nothing", Program: filepath.Join(dir, "renamed.exe"),
			Evidence: asks.Publisher{Subject: "Someone", OriginalName: "pwsh.dll"}, Host: false},
	}
	return append(cases, platformCases(t, dir)...)
}

// WithoutEvidence is the table for a caller that decides from the program
// alone: every row whose answer does not depend on evidence.
func WithoutEvidence(cases []Case) []Case {
	var out []Case
	for _, c := range cases {
		if c.Evidence == (asks.Publisher{}) {
			out = append(out, c)
		}
	}
	return out
}
