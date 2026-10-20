package client

import (
	asks "github.com/openabstractions/abstraction-asks/go"
	"strings"
	"testing"
)

// Both published projections refuse adding this grammar to Answer.id in this
// window. Keep the compatibility reader's existing field guard in lockstep.
func TestPermissionRequestKeyCompatibilityBounds(t *testing.T) {
	for _, key := range []string{"", "valid", strings.Repeat("x", 128), strings.Repeat("x", 129), "control\n", "\u0085", string([]byte{255})} {
		if operatorWord(key) != asks.ValidRequestKey(key) {
			t.Fatalf("request-key grammar drift for %q", key)
		}
	}
}
