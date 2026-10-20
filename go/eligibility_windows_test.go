//go:build windows

package asks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	asks "github.com/openabstractions/abstraction-asks/go"
	"github.com/openabstractions/abstraction-asks/go/scripthostcases"
	"github.com/openabstractions/abstraction-identity/listen"
)

// ASK-R3: a party to the question is refused under any spelling Windows
// resolves to the same program: another letter case and the 8.3 short name.
// The spellings are checked against the runtime, the verified asker and an
// operator program, where no file-identity check applies, so a rule that
// compared plain strings fails here.
func TestAnswererRuleMatchesPartiesUnderWindowsSpellings(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "A Long Folder Name For Short Names")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	party := filepath.Join(dir, "Party Application.exe")
	target := filepath.Join(t.TempDir(), "target-app.exe")
	for _, path := range []string{party, target} {
		if err := os.WriteFile(path, []byte(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	spellings := map[string]string{"case variant": strings.ToUpper(party)}
	if short, ok := scripthostcases.ShortAlias(party); ok && !strings.EqualFold(short, party) {
		spellings["short name"] = short
		spellings["lower-case short name"] = strings.ToLower(short)
	} else {
		t.Logf("8.3 short names are disabled on this volume; checking the case variant only")
	}
	check := func(rule asks.AnswererRule, program string, p asks.Parties) error {
		c, err := rule.Answerer("account", program, asks.Publisher{})
		if err != nil {
			return err
		}
		return c.Party(p)
	}
	for name, spelling := range spellings {
		c, err := (asks.AnswererRule{Account: "account"}).Answerer("account", spelling, asks.Publisher{})
		if err != nil {
			t.Fatal(err)
		}
		if c.PartyWithRelated(asks.Parties{Target: target}, []asks.Parties{{Target: party}}) == nil {
			t.Errorf("a related program answered as its %s %s", name, spelling)
		}
		if check(asks.AnswererRule{Account: "account", Runtime: party}, spelling, asks.Parties{Target: target}) == nil {
			t.Errorf("the runtime answered its own question as its %s %s", name, spelling)
		}
		if check(asks.AnswererRule{Account: "account"}, spelling, asks.Parties{Target: target, Via: listen.Seen{Bound: true, Path: party}}) == nil {
			t.Errorf("the asker answered its own question as its %s %s", name, spelling)
		}
		if check(asks.AnswererRule{Account: "account"}, spelling, asks.Parties{Target: party}) == nil {
			t.Errorf("the target answered its own question as its %s %s", name, spelling)
		}
		if check(asks.AnswererRule{Account: "account", Operators: []string{party}}, spelling, asks.Parties{Target: target}) == nil {
			t.Errorf("an operator program answered as its %s %s", name, spelling)
		}
	}
	if _, ok := spellings["short name"]; !ok {
		t.Skip("8.3 short names are disabled on this volume")
	}
}
