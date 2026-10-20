//go:build windows

package scripthostcases

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// signedHosts are signed script hosts that may be installed here; the first
// present one is copied under a new name.
var signedHosts = []string{
	`C:\Program Files\Git\cmd\git.exe`,
	`C:\Program Files\PowerShell\7\pwsh.exe`,
	`C:\Program Files\nodejs\node.exe`,
}

func platformCases(t testing.TB, dir string) []Case {
	t.Helper()
	var cases []Case
	for _, source := range signedHosts {
		if _, err := os.Stat(source); err != nil {
			continue
		}
		renamed := filepath.Join(dir, "signed-helper.exe")
		if err := copyFile(source, renamed); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, Case{Name: "a renamed copy of signed " + filepath.Base(source), Program: renamed, Host: true})
		break
	}
	if len(cases) == 0 {
		t.Log("no signed script host installed here; the renamed signed copy row is skipped")
	}
	long := filepath.Join(dir, "Long Folder Name", "powershell.exe")
	if err := os.MkdirAll(filepath.Dir(long), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(long, []byte("a host by name"), 0o755); err != nil {
		t.Fatal(err)
	}
	if short, ok := shortAlias(long); ok && filepath.Base(short) != filepath.Base(long) {
		cases = append(cases, Case{Name: "a short 8.3 alias of powershell.exe", Program: short, Host: true})
	} else {
		t.Log("this volume makes no distinct short name; the short alias row is skipped")
	}
	return cases
}

func shortAlias(path string) (string, bool) {
	long, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", false
	}
	buf := make([]uint16, 1024)
	n, err := syscall.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return "", false
	}
	return syscall.UTF16ToString(buf[:n]), true
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
