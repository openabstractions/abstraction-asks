//go:build !windows

package scripthostcases

import "testing"

// platformCases adds nothing where programs carry no signed version resource
// and no short names.
func platformCases(testing.TB, string) []Case { return nil }
