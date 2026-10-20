// Package scripthostcases is the one table of script-host cases that every
// caller of identity's script-host facts runs in its own tests (asks ASK-A8,
// ASK-R7, rights RIGHTS-R22): the defaults exclusion and the prompt read
// Host (asks.ScriptHostSubject), and the answerer refusals, the enrollment
// refusal and the Panel's assistant chooser read Foreign
// (asks.AnswererRule), so a caller that bypasses either rule fails here.
package scripthostcases

import (
	"os"
	"path/filepath"
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
	"github.com/openabstractions/abstraction-identity/hostfixture"
)

// Case is one program a caller decides about, with the publisher evidence a
// receiving peer would carry (zero for a caller that has none), whether it is
// a reviewed script host (a shared interpreter) and whether it runs foreign
// code, which every script host does.
type Case struct {
	Name     string
	Program  string
	Evidence asks.Publisher
	Host     bool
	Foreign  bool
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
	// Images that run foreign code without being shared interpreters (TD83),
	// and an interpreter's name that a Bun graph does not excuse.
	images := filepath.Join(dir, "images")
	opencode := hostfixture.Bun(t, images, "opencode.exe", true)
	bunResource := asks.Publisher{Subject: "Anomaly Innovations, Inc", Trusted: true, OriginalName: "bun.exe", Description: "Bun"}
	cases := []Case{
		{Name: "stock OpenCode, a Bun standalone executable", Program: opencode, Foreign: true},
		{Name: "stock OpenCode with Bun's version resource", Program: opencode, Evidence: bunResource, Foreign: true},
		{Name: "an Electron program with RunAsNode on", Program: hostfixture.Electron(t, images, "Code.exe", true), Foreign: true},
		{Name: "python.exe carrying a Bun graph", Program: hostfixture.Bun(t, filepath.Join(images, "graph"), "python.exe", false), Host: true},
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
	cases = append(cases, platformCases(t, dir)...)
	for i := range cases {
		cases[i].Foreign = cases[i].Foreign || cases[i].Host
	}
	return cases
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
