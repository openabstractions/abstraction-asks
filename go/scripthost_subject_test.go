package asks_test

import (
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
	"github.com/openabstractions/abstraction-asks/go/scripthostcases"
	rwire "github.com/openabstractions/abstraction-rights/go/abstraction/rights/api"
)

// ASK-R7: the one script-host rule over the shared table every caller runs:
// msix package forms, publisher evidence, a renamed signed copy, a short 8.3
// alias and ordinary names.
func TestScriptHostSubjectSharedTable(t *testing.T) {
	for _, c := range scripthostcases.Cases(t) {
		if got := asks.ScriptHostSubject(rwire.Subject{Account: "account", Program: c.Program}, c.Evidence); got != c.Host {
			t.Errorf("%s (%s): %t, want %t", c.Name, c.Program, got, c.Host)
		}
	}
}
