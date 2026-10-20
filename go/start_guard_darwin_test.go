//go:build darwin

package asks

import (
	"os"
	"testing"
)

// A macOS socket can never prove the calling program: Start returns the
// ceiling refusal and writes nothing into its state folder.
func TestStartRefusesDarwinSocketAndWritesNothing(t *testing.T) {
	state := t.TempDir()
	s, err := Start(testEndpoint(t), state)
	if err == nil {
		s.Close()
		t.Fatal("Start on a macOS socket started")
	}
	if entries, _ := os.ReadDir(state); len(entries) != 0 {
		t.Fatalf("a refused Start wrote %v", entries)
	}
}
