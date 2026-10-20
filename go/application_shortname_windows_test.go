package asks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	"golang.org/x/sys/windows"
)

// A first-use question whose asking slots name the program by its 8.3 alias
// is stored with the program's own name in both its content and its
// permission, so the Book reads it back and keeps working.
func TestFirstUseAskByShortAliasReadsBack(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "short alias program")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(folder, "asking program.exe")
	if err := os.WriteFile(program, []byte("inert program"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := windows.UTF16PtrFromString(program)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || n >= uint32(len(buf)) {
		t.Fatalf("short path of %q: %v", program, err)
	}
	short := windows.UTF16ToString(buf[:n])
	if strings.EqualFold(short, program) {
		t.Skip("filesystem has no distinct DOS alias for this file")
	}
	key := identity.NormalizeSubjectProgram(short)

	b, _ := ordinaryBook(t)
	r, outcome, err := b.AskApplication("scope", "asking program", "first", FirstUseKey,
		map[string]string{"program": short, "action": "abstraction.storage/content.write", "resource": "output:image"}, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatal(outcome, err)
	}
	read, found, err := b.ApplicationRecord(r.ID)
	if err != nil || !found {
		t.Fatalf("first-use question read back: %t %v", found, err)
	}
	if read.Content == nil || read.Content.Slots["program"] != key || read.Permission == nil || read.Permission.Targets[0].Program != key {
		t.Fatalf("first-use question names %+v / %+v, want %q", read.Content, read.Permission, key)
	}
}
