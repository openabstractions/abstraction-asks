package asks

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

// ErrAnswererForbidden is the answerer rule's refusal (ASK-R3, ASK-R5,
// ASK-R7, ASK-R9). The responder service reads it as forbidden.
var ErrAnswererForbidden = errors.New("asks: responder forbidden")

// AnswererRule is the answerer eligibility rule this package owns (ASK-R3,
// ASK-R5, ASK-R7, ASK-R9) with the facts a host composes it from: the
// receiving account, the runtime's own program, which asks first-use
// questions as itself, and the operator programs, which answer as the
// person. A host keeps only the rights decisions (a current enrollment, an
// exact delegation) and the question's tier; every rule about who the
// answerer is and whom a question concerns is here, so every host of the
// responder service applies the same one.
type AnswererRule struct {
	Account   string
	Runtime   string
	Operators []string
}

// Parties is whom a question or choice concerns, as its admission recorded
// them: the target program (the program a first-use permission is for, or
// the program that asked an ordinary choice), the publisher recorded for it
// (ASK-A6) and the peer the admission bound. It is the one party rule every
// answerer check reads.
type Parties struct {
	Target           string
	Publisher        string
	PublisherTrusted bool
	Via              listen.Seen
}

// FirstUseParties are the parties to the runtime's first-use question about
// target.
func FirstUseParties(record Record, target string) Parties {
	return Parties{Target: target, Publisher: record.Publisher, PublisherTrusted: record.PublisherTrusted, Via: record.Via}
}

// ChoiceParties are the parties to a shared ordinary choice: the requester is
// its target.
func ChoiceParties(c ChoiceCandidate) Parties {
	return Parties{Target: c.AskerProgram, Publisher: c.Record.Publisher, PublisherTrusted: c.Record.PublisherTrusted, Via: c.Via}
}

// AnswerCheck is one answerer held against the rule. It reads each
// program's spelling and file identity once, for one answerer check.
type AnswerCheck struct {
	rule      AnswererRule
	program   string
	publisher Publisher
	m         *programMatcher
}

// Answerer checks the part of the rule that needs no question: the answerer
// is a program of the receiving account; it runs no foreign code, so it is
// neither a shared interpreter nor an image identity reads RunsForeignCode
// for (identity ID-I1, TD83: such a program never answers for other
// applications); and it is no operator program. publisher is the answering
// peer's verified code (ASK-A6). It returns the check the party rule reads,
// or ErrAnswererForbidden.
func (r AnswererRule) Answerer(account, program string, publisher Publisher) (*AnswerCheck, error) {
	return r.AnswererWithDeclaredHosts(account, program, publisher, nil)
}

// AnswererWithDeclaredHosts also applies trusted, complete receiving-host
// declarations. A declared host is a conservative veto, compared by the same
// subject-program identity rule as operators. The caller must refuse unavailable
// declaration metadata before calling; an empty slice represents verified none.
func (r AnswererRule) AnswererWithDeclaredHosts(account, program string, publisher Publisher, declaredHosts []DeclaredHost) (*AnswerCheck, error) {
	if account == "" || account != r.Account || program == "" {
		return nil, ErrAnswererForbidden
	}
	if identity.ClassifyProgram(program, publisher.code()).RunsForeignCode {
		return nil, ErrAnswererForbidden
	}
	c := &AnswerCheck{rule: r, program: program, publisher: publisher, m: newProgramMatcher()}
	if slices.ContainsFunc(r.Operators, func(operator string) bool { return c.m.same(operator, program) }) {
		return nil, ErrAnswererForbidden
	}
	if slices.ContainsFunc(declaredHosts, func(host DeclaredHost) bool {
		if c.m.same(host.Program, program) {
			return true
		}
		return host.FileKey != "" && host.FileKey == c.m.key(c.m.normalized(program))
	}) {
		return nil, ErrAnswererForbidden
	}
	return c, nil
}

// DeclaredHost is one trusted receiving directory declaration and the native
// file identity retained with it. FileKey keeps hard-link aliases vetoed even
// when the originally registered path has since been replaced.
type DeclaredHost struct {
	Program string
	FileKey string
}

// Runtime reports whether the answerer is the runtime's own program, which
// never enrolls as an answerer (ASK-R10).
func (c *AnswerCheck) Runtime() bool {
	return c.m.same(c.program, c.rule.Runtime)
}

// Party refuses an answerer that is a party to the question or choice
// (ASK-R3, ASK-R9): the target program or an asker (the runtime, or the
// peer the admission bound), compared by the shared subject-program rules;
// a program whose trusted publisher is the one recorded for the target; or
// the target's own executable file under another path. It returns nil for a
// program that is no party.
func (c *AnswerCheck) Party(p Parties) error {
	return c.PartyWithRelated(p, nil)
}

// PartyWithRelated also refuses parties supplied by the host's current trusted
// relationships. A live messaging permit in either direction makes each program
// a party to the other's questions and choices (agent-bridge DESIGN section 5).
// The host must refuse unavailable relationship metadata before calling; nil
// represents verified none. Caller-supplied relationship claims are not facts.
func (c *AnswerCheck) PartyWithRelated(p Parties, related []Parties) error {
	if c.partyTo(p) {
		return ErrAnswererForbidden
	}
	for _, party := range related {
		if c.partyTo(party) {
			return ErrAnswererForbidden
		}
	}
	return nil
}

func (c *AnswerCheck) partyTo(p Parties) bool {
	if p.PublisherTrusted && p.Publisher != "" && c.publisher.Trusted && c.publisher.Subject == p.Publisher {
		return true
	}
	if c.m.same(c.program, p.Target) || c.m.same(c.program, c.rule.Runtime) {
		return true
	}
	if p.Via.Bound && p.Via.Path != "" && c.m.same(c.program, p.Via.Path) {
		return true
	}
	return c.m.sameFile(c.program, p.Target)
}

// programMatcher compares subject programs by the shared identity rules
// (identity.SameNormalizedSubjectProgramKeyed) and files by file identity
// (identity.ProgramKey), reading each program's spelling and file identity
// once.
type programMatcher struct {
	normal map[string]string
	keys   map[string]string
}

// matcherProgramKey is the matcher's one read of a program's file identity;
// a test counts reads through it.
var matcherProgramKey = identity.ProgramKey

func newProgramMatcher() *programMatcher {
	return &programMatcher{normal: map[string]string{}, keys: map[string]string{}}
}

func (m *programMatcher) normalized(program string) string {
	n, ok := m.normal[program]
	if !ok {
		n = identity.NormalizeSubjectProgram(program)
		m.normal[program] = n
	}
	return n
}

func (m *programMatcher) key(normalized string) string {
	k, ok := m.keys[normalized]
	if !ok {
		k = matcherProgramKey(normalized)
		m.keys[normalized] = k
	}
	return k
}

// same is identity.SameSubjectProgram: relative or invalid input never
// matches.
func (m *programMatcher) same(a, b string) bool {
	if !filepath.IsAbs(a) && !strings.HasPrefix(a, identity.PackagedProgramPrefix) || !filepath.IsAbs(b) && !strings.HasPrefix(b, identity.PackagedProgramPrefix) {
		return false
	}
	na, nb := m.normalized(a), m.normalized(b)
	return identity.SameNormalizedSubjectProgramKeyed(na, nb, func() string { return m.key(na) }, func() string { return m.key(nb) })
}

// sameFile reports whether two program paths name one existing local file
// by file identity: the same volume and file ID on Windows, the same device
// and inode elsewhere, so a hard link or another spelling of the target's
// executable is the target. A path that is not a readable local file is no
// match.
func (m *programMatcher) sameFile(a, b string) bool {
	if !filepath.IsAbs(a) || !filepath.IsAbs(b) {
		return false
	}
	ka, kb := m.key(m.normalized(a)), m.key(m.normalized(b))
	return ka != "" && ka == kb
}
