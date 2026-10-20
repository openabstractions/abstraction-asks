//go:build windows

package asks

import (
	"os"
	"path/filepath"
	"testing"

	identity "github.com/openabstractions/abstraction-identity"
)

// TestScriptHostsMatchTheirSignedNames: the inbox script hosts on this
// machine verify trusted, and the OriginalFilename identity reads from each
// is on the reviewed list, so the list's spellings are the signed ones. A
// host this Windows does not ship is skipped.
func TestScriptHostsMatchTheirSignedNames(t *testing.T) {
	root := os.Getenv("SystemRoot")
	for _, path := range []string{
		filepath.Join(root, `System32\WindowsPowerShell\v1.0\powershell.exe`),
		filepath.Join(root, `System32\cmd.exe`),
		filepath.Join(root, `System32\wscript.exe`),
		filepath.Join(root, `System32\cscript.exe`),
		filepath.Join(root, `System32\mshta.exe`),
		filepath.Join(root, `System32\rundll32.exe`),
		filepath.Join(root, `System32\regsvr32.exe`),
		filepath.Join(root, `System32\wsl.exe`),
		filepath.Join(root, `System32\bash.exe`),
		filepath.Join(root, `Microsoft.NET\Framework64\v4.0.30319\MSBuild.exe`),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Logf("skipped %s: %v", path, err)
			continue
		}
		code, _, err := identity.VerifyFile(path, nil)
		publisher := CodePublisher(code)
		if err != nil || !publisher.Trusted || publisher.Description == "" || !ScriptHost(publisher.OriginalName) {
			t.Errorf("%s: %+v (OriginalFilename %q) %v", path, publisher, code.OriginalName, err)
			continue
		}
		t.Logf("%s: %s", path, ScriptHostSentence(publisher.Description, publisher.OriginalName))
	}
}
