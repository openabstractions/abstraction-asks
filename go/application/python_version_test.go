package application

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// asksPyproject declares the Python version this repository's package requires.
var asksPyproject = filepath.Join("..", "..", "py", "pyproject.toml")

var requiresPythonLine = regexp.MustCompile(`(?m)^requires-python\s*=\s*"\s*>=\s*(\d+)\.(\d+)\s*"`)

// requiredPython reads the minimum version a pyproject's requires-python
// declares in its ">=X.Y" form.
func requiredPython(pyproject []byte) ([2]int, error) {
	m := requiresPythonLine.FindSubmatch(pyproject)
	if m == nil {
		return [2]int{}, fmt.Errorf(`no requires-python = ">=X.Y" line`)
	}
	major, _ := strconv.Atoi(string(m[1]))
	minor, _ := strconv.Atoi(string(m[2]))
	return [2]int{major, minor}, nil
}

// interpreterBelow returns the skip reason when an interpreter version
// "X.Y[.Z]" is older than the required one, and "" when it satisfies it.
func interpreterBelow(version string, required [2]int) (string, error) {
	version = strings.TrimSpace(version)
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return "", fmt.Errorf("interpreter version %q is not X.Y", version)
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return "", fmt.Errorf("interpreter version %q is not X.Y", version)
	}
	if major < required[0] || (major == required[0] && minor < required[1]) {
		return fmt.Sprintf("Python %s is older than the asks package's requires-python >=%d.%d", version, required[0], required[1]), nil
	}
	return "", nil
}

// skipBelowRequiresPython skips the test when the interpreter is older than
// the asks Python package's requires-python.
func skipBelowRequiresPython(t *testing.T, python string, args []string) {
	t.Helper()
	data, err := os.ReadFile(asksPyproject)
	if err != nil {
		t.Fatal(err)
	}
	required, err := requiredPython(data)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(python, append(slices.Clone(args), "-c", "import sys;print('%d.%d.%d' % sys.version_info[:3])")...).Output()
	if err != nil {
		t.Skipf("Python interpreter does not run: %v", err)
	}
	reason, err := interpreterBelow(string(out), required)
	if err != nil {
		t.Fatal(err)
	}
	if reason != "" {
		t.Skip(reason)
	}
}

func TestInterpreterBelowRequiresPython(t *testing.T) {
	data, err := os.ReadFile(asksPyproject)
	if err != nil {
		t.Fatal(err)
	}
	required, err := requiredPython(data)
	if err != nil {
		t.Fatal(err)
	}
	if required != [2]int{3, 10} {
		t.Fatalf("requires-python = %v, want >=3.10", required)
	}
	for _, c := range []struct {
		version string
		below   bool
	}{{"3.9.6\n", true}, {"2.7.18", true}, {"3.10.0", false}, {"3.12.4", false}, {"4.0.0", false}} {
		reason, err := interpreterBelow(c.version, required)
		if err != nil {
			t.Fatal(err)
		}
		if (reason != "") != c.below {
			t.Errorf("%q: reason %q, want below=%v", c.version, reason, c.below)
		}
	}
	if _, err := interpreterBelow("garbage", required); err == nil {
		t.Error("unparsable version accepted")
	}
}
