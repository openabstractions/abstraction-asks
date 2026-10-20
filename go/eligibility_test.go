package asks

import (
	"errors"
	identity "github.com/openabstractions/abstraction-identity"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/openabstractions/abstraction-identity/hostfixture"
	"github.com/openabstractions/abstraction-identity/listen"
)

func TestAnswererDeclaredHostsPreserveNativeCeilingAndAliases(t *testing.T) {
	dir := t.TempDir()
	program := writeProgram(t, dir, "own-host.exe")
	alias := filepath.Join(dir, "alias.exe")
	if err := os.Link(program, alias); err != nil {
		t.Fatal(err)
	}
	// An unkeyed client keeps compiling against the published three fields.
	rule := AnswererRule{"account", "", nil}
	if _, err := rule.Answerer("account", program, Publisher{}); err != nil {
		t.Fatal(err)
	}
	hosts := []DeclaredHost{{Program: program, FileKey: identity.ProgramKey(program)}}
	for _, candidate := range []string{program, alias} {
		if _, err := rule.AnswererWithDeclaredHosts("account", candidate, Publisher{}, hosts); !errors.Is(err, ErrAnswererForbidden) {
			t.Fatal(candidate, err)
		}
	}
	if err := os.Remove(program); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(program, []byte("replacement"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := rule.AnswererWithDeclaredHosts("account", alias, Publisher{}, hosts); !errors.Is(err, ErrAnswererForbidden) {
		t.Fatal("retained file alias", err)
	}
	if _, err := rule.AnswererWithDeclaredHosts("account", filepath.Join(dir, "python.exe"), Publisher{}, nil); !errors.Is(err, ErrAnswererForbidden) {
		t.Fatal("native ceiling", err)
	}
}

// writeProgram writes a small regular file standing in for an executable.
func writeProgram(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("program "+name), 0o700); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// answerCheck runs the whole rule for one answerer and one question.
func answerCheck(rule AnswererRule, account, program string, publisher Publisher, p Parties) error {
	c, err := rule.Answerer(account, program, publisher)
	if err != nil {
		return err
	}
	return c.Party(p)
}

func TestAnswerCheckRelatedPartiesUseTheSameIdentityRule(t *testing.T) {
	dir := t.TempDir()
	a, b := writeProgram(t, dir, "agent-a.exe"), writeProgram(t, dir, "agent-b.exe")
	other := writeProgram(t, dir, "unrelated.exe")
	rule := AnswererRule{Account: "account"}
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		check, err := rule.Answerer("account", pair[0], Publisher{})
		if err != nil {
			t.Fatal(err)
		}
		question := Parties{Target: pair[1]}
		if err := check.Party(question); err != nil {
			t.Fatal("existing independent programs", err)
		}
		if err := check.PartyWithRelated(question, []Parties{{Target: pair[0]}}); !errors.Is(err, ErrAnswererForbidden) {
			t.Fatal("live relation", err)
		}
		if err := check.PartyWithRelated(question, nil); err != nil {
			t.Fatal("revoked relation", err)
		}
		if err := check.PartyWithRelated(question, []Parties{{Target: other}}); err != nil {
			t.Fatal("unrelated", err)
		}
	}
	alias := filepath.Join(dir, "agent-alias.exe")
	if err := os.Link(a, alias); err != nil {
		t.Fatal(err)
	}
	check, err := rule.Answerer("account", alias, Publisher{})
	if err != nil {
		t.Fatal(err)
	}
	if err := check.PartyWithRelated(Parties{Target: b}, []Parties{{Target: a}}); !errors.Is(err, ErrAnswererForbidden) {
		t.Fatal("related file alias", err)
	}
	check, err = rule.Answerer("account", other, Publisher{Subject: "CN=Agent Vendor", Trusted: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := check.PartyWithRelated(Parties{Target: b}, []Parties{{Target: a, Publisher: "CN=Agent Vendor", PublisherTrusted: true}}); !errors.Is(err, ErrAnswererForbidden) {
		t.Fatal("related trusted publisher", err)
	}
	if err := check.PartyWithRelated(Parties{Target: b}, []Parties{{Target: a, Publisher: "CN=Agent Vendor"}}); err != nil {
		t.Fatal("untrusted publisher", err)
	}
}

// ASK-R3, ASK-R5, ASK-R7, ASK-R9 (asks P0-2): the answerer rule as one
// table, without the runtime: the target program, the target's file under
// another path, a program of the target's trusted publisher, the bound
// asker, the runtime, an operator program, a shared interpreter and a
// program that runs foreign code (the stock OpenCode binary) are refused;
// another account is refused; an unrelated program is admitted.
func TestAnswererRuleTable(t *testing.T) {
	dir := t.TempDir()
	target := writeProgram(t, dir, "target-app.exe")
	runtimeProgram := writeProgram(t, dir, "openabstractions.exe")
	panel := writeProgram(t, dir, "openabstractions-panel.exe")
	asker := writeProgram(t, dir, "asking-app.exe")
	other := writeProgram(t, dir, "assistant.exe")
	python := writeProgram(t, dir, "python.exe")
	opencode := hostfixture.Bun(t, dir, "opencode.exe", true)
	electron := hostfixture.Electron(t, dir, "Code.exe", true)
	link := filepath.Join(dir, "helper-link.exe")
	hardLinks := os.Link(target, link) == nil
	rule := AnswererRule{Account: "account", Runtime: runtimeProgram, Operators: []string{panel}}
	plain := Parties{Target: target}
	signed := Parties{Target: target, Publisher: "CN=Example Apps", PublisherTrusted: true}
	bound := Parties{Target: target, Via: listen.Seen{Bound: true, Path: asker}}
	bunResource := Publisher{Subject: "Anomaly Innovations, Inc", Trusted: true, OriginalName: "bun.exe", Description: "Bun"}
	rows := []struct {
		name      string
		account   string
		program   string
		publisher Publisher
		parties   Parties
		refused   bool
	}{
		{"an unrelated program", "account", other, Publisher{}, plain, false},
		{"the target program", "account", target, Publisher{}, plain, true},
		{"the runtime", "account", runtimeProgram, Publisher{}, plain, true},
		{"an operator program", "account", panel, Publisher{}, plain, true},
		{"the bound asker", "account", asker, Publisher{}, bound, true},
		{"an unbound Via path refuses nothing", "account", asker, Publisher{}, Parties{Target: target, Via: listen.Seen{Path: asker}}, false},
		{"the target's trusted publisher", "account", other, Publisher{Subject: "CN=Example Apps", Trusted: true}, signed, true},
		{"the same subject, untrusted", "account", other, Publisher{Subject: "CN=Example Apps"}, signed, false},
		{"another trusted publisher", "account", other, Publisher{Subject: "CN=Someone Else", Trusted: true}, signed, false},
		{"an untrusted target publisher", "account", other, Publisher{Subject: "CN=Example Apps", Trusted: true}, Parties{Target: target, Publisher: "CN=Example Apps"}, false},
		{"a shared interpreter by name", "account", python, Publisher{}, plain, true},
		{"a shared interpreter by signed name", "account", other, Publisher{Subject: "OpenJS Foundation", Trusted: true, OriginalName: "node.exe"}, plain, true},
		{"the stock OpenCode binary", "account", opencode, bunResource, plain, true},
		{"an Electron program with RunAsNode on", "account", electron, Publisher{}, plain, true},
		{"another account", "elsewhere", other, Publisher{}, plain, true},
		{"no account", "", other, Publisher{}, plain, true},
	}
	if hardLinks {
		rows = append(rows, struct {
			name      string
			account   string
			program   string
			publisher Publisher
			parties   Parties
			refused   bool
		}{"the target's file under another path", "account", link, Publisher{}, plain, true})
	} else {
		t.Log("hard links unavailable here; the same-file row is skipped")
	}
	for _, row := range rows {
		err := answerCheck(rule, row.account, row.program, row.publisher, row.parties)
		if refused := errors.Is(err, ErrAnswererForbidden); refused != row.refused || err != nil && !refused {
			t.Errorf("%s: %v, want refused %t", row.name, err, row.refused)
		}
	}
	if c, err := rule.Answerer("account", runtimeProgram, Publisher{}); err != nil || !c.Runtime() {
		t.Errorf("the runtime is not recognised as itself: %v", err)
	}
	if c, err := rule.Answerer("account", other, Publisher{}); err != nil || c.Runtime() {
		t.Errorf("an app reads as the runtime: %v", err)
	}
}

// ASK-R3: a copy of the target's executable is a different file, and a
// missing file matches nothing.
func TestAnswererRuleSeparatesCopies(t *testing.T) {
	dir := t.TempDir()
	target := writeProgram(t, dir, "target-app.exe")
	copied := writeProgram(t, dir, "copied-app.exe")
	rule := AnswererRule{Account: "account"}
	if err := answerCheck(rule, "account", copied, Publisher{}, Parties{Target: target}); err != nil {
		t.Errorf("a separate file was refused as the target: %v", err)
	}
	if err := answerCheck(rule, "account", filepath.Join(dir, "missing.exe"), Publisher{}, Parties{Target: target}); err != nil {
		t.Errorf("a missing file matched the target: %v", err)
	}
}

// One answer check reads each program's file identity at most once, however
// many parties and operators it compares (programMatcher), so a check stays
// as cheap as a decision. The reads are counted at the matcher's one read
// point, on every platform.
func TestAnswerCheckReadsEachFileIdentityOnce(t *testing.T) {
	dir := t.TempDir()
	answerer := writeProgram(t, dir, "assistant.exe")
	var operators []string
	for _, name := range []string{"panel-a.exe", "panel-b.exe", "panel-c.exe"} {
		operators = append(operators, writeProgram(t, dir, name))
	}
	rule := AnswererRule{Account: "account", Runtime: writeProgram(t, dir, "openabstractions.exe"), Operators: operators}
	parties := Parties{Target: writeProgram(t, dir, "target.exe"), Via: listen.Seen{Bound: true, Path: writeProgram(t, dir, "asker.exe")}}
	var mu sync.Mutex
	reads := map[string]int{}
	real := matcherProgramKey
	matcherProgramKey = func(program string) string {
		mu.Lock()
		reads[program]++
		mu.Unlock()
		return real(program)
	}
	t.Cleanup(func() { matcherProgramKey = real })
	if err := answerCheck(rule, "account", answerer, Publisher{}, parties); err != nil {
		t.Fatal(err)
	}
	for program, n := range reads {
		if n > 1 {
			t.Errorf("%s read %d times in one check", program, n)
		}
	}
	if len(reads) == 0 {
		t.Error("the check read no file identity")
	}
}
