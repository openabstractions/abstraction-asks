package asks

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	identity "github.com/openabstractions/abstraction-identity"
)

// A signature read that fails is no evidence and is not remembered: the next
// check of the same file reads it again. A successful read is remembered by
// file identity.
func TestSignedScriptHostFileRetriesAFailedRead(t *testing.T) {
	program := filepath.Join(t.TempDir(), "renamed-tool.exe")
	if err := os.WriteFile(program, []byte("copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if identity.ProgramKey(program) == "" {
		t.Skip("no file identity on this platform")
	}
	reads, failing := 0, true
	saved := verifyProgramFile
	verifyProgramFile = func(string, *identity.Options) (identity.Code, identity.Proof, error) {
		reads++
		if failing {
			return identity.Code{}, 0, errors.New("file busy")
		}
		return identity.Code{Trusted: true, OriginalName: "powershell.exe"}, 0, nil
	}
	t.Cleanup(func() { verifyProgramFile = saved })
	if signedScriptHostFile(program) {
		t.Fatal("a failed read reads as a script host")
	}
	failing = false
	if !signedScriptHostFile(program) || reads != 2 {
		t.Fatalf("after a failed read the file was not read again: reads=%d", reads)
	}
	if !signedScriptHostFile(program) || reads != 2 {
		t.Fatalf("a successful read is not remembered: reads=%d", reads)
	}
}
