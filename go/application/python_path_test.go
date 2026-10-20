package application

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// privateInput is a path, relative to this package, that a test reads from
// the private source workspace because its check spans repositories: the
// file or module lives in a repository scripts/split.manifest publishes apart
// from this one. A published checkout lacks it and the test skips there; the
// private gate runs it. scripts/fixture_reads_manifest.py refuses a test path
// outside the package that private source holds and this repository does not
// publish unless it is named here, and refuses a name here that private
// source lacks.
func privateInput(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(parts...)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("%s is private-workspace input this repository does not publish; the private gate runs this check", filepath.ToSlash(path))
		}
		t.Fatal(err)
	}
	return path
}

// generatedClientPythonPath is the PYTHONPATH that runs this repository's
// generated Python client. Its codec imports abstraction.rights, the
// abstraction-rights-protocol package py/pyproject.toml declares as its
// dependency. An interpreter that has that package installed runs the
// client as an installed asks package runs; a PYTHONPATH the caller set
// counts, which is how the asks build.yml installs it (pip --target).
// Otherwise the rights source beside this repository in the private
// workspace stands in for it.
func generatedClientPythonPath(t *testing.T, python string, args []string) string {
	t.Helper()
	own := filepath.Join("..", "..", "py")
	if inherited := os.Getenv("PYTHONPATH"); inherited != "" {
		own += string(os.PathListSeparator) + inherited
	}
	probe := exec.Command(python, append(slices.Clone(args), "-c", "import abstraction.rights.api._codec")...)
	probe.Env = append(os.Environ(), "PYTHONPATH="+own)
	if probe.Run() == nil {
		return own
	}
	return own + string(os.PathListSeparator) + privateInput(t, "..", "..", "..", "abstraction-rights", "py")
}
