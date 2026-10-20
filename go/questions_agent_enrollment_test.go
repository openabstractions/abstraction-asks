package asks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentEnrollmentKeepsProgramInDetails(t *testing.T) {
	program := filepath.Join(t.TempDir(), "native-agent.exe")
	a, text, err := applicationQuestion("runtime", AgentEnrollmentKey, map[string]string{"program": program, "title": "Fixture assistant"})
	if err != nil || strings.Contains(text, program) || a.Slots["program"] != program || text != "Connect Fixture assistant?" {
		t.Fatal(a, text, err)
	}
	if !RuntimeOnlyKey(AgentEnrollmentKey) {
		t.Fatal("caller could admit trusted agent question")
	}
	_, connection, err := applicationQuestion("runtime", AgentEnrollmentKey, map[string]string{"program": program, "title": "OpenCode"})
	if err != nil || connection != "Connect OpenCode?" {
		t.Fatal(connection, err)
	}
	if _, _, err := applicationQuestion("runtime", AgentEnrollmentKey, map[string]string{"title": "Fixture assistant"}); err == nil {
		t.Fatal("missing bound details accepted")
	}
	if _, _, err := applicationQuestion("runtime", AgentEnrollmentKey, map[string]string{"program": program, "title": "Fixture assistant", "class": "agent"}); err == nil {
		t.Fatal("extra caller class accepted")
	}
}
