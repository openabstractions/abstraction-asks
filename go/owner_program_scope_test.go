package asks

import (
	"path/filepath"
	"testing"

	identity "github.com/openabstractions/abstraction-identity"
)

func TestApplicationOwnerProgramScopePreservesProfileBytes(t *testing.T) {
	const executable = `C:\OA\runtime.exe`
	const want = "asks-owner-program@1:e0efdec99442674dba476aaf2e48bb62ee78075f757cb8e71f10c20431e10825"
	if got := ApplicationOwnerProgramScope("windows", "S-1-test", executable); got != want {
		t.Fatalf("scope %q, want %q", got, want)
	}
	owner := &nativeApplicationOwner{Subject: identity.Subject{AccountKind: "windows", Account: "S-1-test", Program: "package-family:OA"}, Executable: executable}
	if got := owner.legacy(); got != want {
		t.Fatalf("native legacy scope %q, want %q", got, want)
	}
	unclean := filepath.Dir(executable) + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(executable)
	if got := ApplicationOwnerProgramScope("windows", "S-1-test", unclean); got != ApplicationOwnerProgramScope("windows", "S-1-test", filepath.Clean(unclean)) {
		t.Fatal("unclean observed executable changed scope")
	}
}
