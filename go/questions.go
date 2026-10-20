package asks

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnknownQuestion = errors.New("asks: no such question")
	ErrUnknownOption   = errors.New("asks: no such option")
	ErrBadSlot         = errors.New("asks: slot")
)

const maxSlot = 200

type Option struct {
	Name string
	Yes  bool
	Kept bool
}

type Question struct {
	Key     string
	Text    string
	About   string
	Options []Option
	// Closed offers exactly Options, without the Never every other catalogue
	// question adds.
	Closed bool
	// RuntimeBound marks a question whose answer acts only through state held
	// in the memory of the runtime that admitted it (ASK-S1: the protected
	// operation's ticket). A runtime that has just bound its endpoints
	// withdraws every pending one (Book.WithdrawRuntimeBound), so none is
	// answered after the state it was bound to is gone.
	RuntimeBound bool
}

var Never = Option{Name: "never", Kept: true}

var Questions = []Question{
	{Key: "content.replace",
		Text:    "{asker} requests a one-time replacement of {target} with {replacement}. Request {code} expires {until}.",
		About:   "target",
		Options: []Option{{Name: "once", Yes: true}},
	},
	{Key: "rights.register",
		Text:    "{asker} wants to: {rights}",
		Options: []Option{{Name: "allow", Yes: true}, {Name: "refuse"}}},
	// The uncontained OpenCode session owner binds this one-time answer to
	// its exact folder identity and applies the OS effect. No OA rule is made.
	{Key: "opencode.project.edit",
		Text:    "OpenCode requests permission to read, run, create, edit and delete files throughout {folder}, including its subfolders, until this sandbox session exits. Request {code} expires {until}.",
		About:   "folder",
		Options: []Option{{Name: "once", Yes: true}, {Name: "refuse"}}},
	{Key: "download.reach",
		Text:    "{asker} wants to fetch from {host}",
		About:   "host",
		Options: []Option{{Name: "allow", Yes: true, Kept: true}, {Name: "once", Yes: true}, {Name: "refuse"}}},
	// The runtime admits this question as itself when a spending action it
	// gates decides not_granted (research/rights-defaults/DECISION.md §2). The
	// About slot is the program's exact path; the text is "<program> wants to
	// <action> on <resource>", and an action carries no space. The answer is a
	// person's choice: an operator writes the rule, and the question grants
	// nothing. Applications cannot admit it (FirstUseKey).
	{Key: FirstUseKey,
		Text:    "{program} wants to {action} on {resource}",
		About:   "program",
		Options: []Option{{Name: "allow", Yes: true}, {Name: "refuse"}}},
	// The runtime admits this question as itself, once, when a program
	// registered itself under the account-wide registration default (rights
	// RIGHTS-R22). Keep confirms the entry; Remove removes it and blocks the
	// program from registering itself again. Closing or expiry answers
	// nothing and the entry stays unconfirmed. Only the person answers: it
	// vouches for app-written title text, so no answerer may (ASK-R11).
	// Applications cannot admit it (RuntimeOnlyKey). The About slot is the
	// path of the program that registered itself, bound from its connection;
	// SelfRegistered reads the program and title back.
	{Key: SelfRegisteredKey,
		Text:    "{program} added itself to OpenAbstractions as {title}. Keep it?",
		About:   "program",
		Options: []Option{{Name: "keep", Yes: true}, {Name: "remove"}},
		Closed:  true},
	// The runtime admits this question as itself, naming the requesting
	// program as asker, for one protected Security-log page (ASK-S1). Only
	// the person answers it; once authorizes one execution by the installed
	// helper after the platform's administrator prompt and writes no rule.
	// The code binds the text to the frozen request. Applications cannot
	// admit it (RuntimeOnlyKey). Its ticket lives in the admitting runtime's
	// memory, so a restarted runtime withdraws it while pending.
	{Key: SystemSecurityLogKey,
		Text:         "{asker} wants to read one page of the Windows Security log. Windows asks for administrator approval next. Request {code} expires {until}.",
		About:        "code",
		Options:      []Option{{Name: "once", Yes: true}, {Name: "refuse"}},
		RuntimeBound: true},
}

// WithdrawRuntimeBound retires every pending question whose catalogue entry
// is RuntimeBound, as DismissQuestion does (ASK-A4, ASK-S1): the record goes
// and its admission stays, so its key observes gone and a new request admits
// a new question. The runtime calls it once it owns its endpoints and
// before it serves, so no question bound to a previous runtime's memory is
// answered and a runtime that fails to start withdraws nothing.
// Answered and other questions are untouched. It returns the withdrawn
// records and writes nothing when there are none.
func (b *Book) WithdrawRuntimeBound() ([]Record, error) {
	if !b.application {
		return nil, errors.New("asks: application Book required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var withdrawn []Record
	unchanged := errors.New("asks: nothing runtime-bound pending")
	err := b.changeApplication(func(f *book) error {
		kept := f.Asks[:0]
		for _, r := range f.Asks {
			if q, ok := Find(r.Key); ok && q.RuntimeBound && r.Pending() {
				withdrawn = append(withdrawn, *r)
				continue
			}
			kept = append(kept, r)
		}
		if len(withdrawn) == 0 {
			return unchanged
		}
		f.Asks = kept
		return nil
	})
	if errors.Is(err, unchanged) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, r := range withdrawn {
		b.settle(r.ID)
	}
	return withdrawn, nil
}

// SelfRegistered reads the program and title of a self-registration question
// from its About slot and text. ok is false for any other question or a text
// that does not read as the question's own sentence.
func SelfRegistered(key, about, text string) (program, title string, ok bool) {
	if key != SelfRegisteredKey || about == "" {
		return "", "", false
	}
	rest, found := strings.CutPrefix(text, about+" added itself to OpenAbstractions as ")
	if !found {
		return "", "", false
	}
	title, found = strings.CutSuffix(rest, ". Keep it?")
	if !found || title == "" {
		return "", "", false
	}
	return about, title, true
}

// SelfRegisteredKey is the question a runtime admits when a program
// registered itself as an application.
const SelfRegisteredKey = "application.self_registered"

// FirstUseKey is the question a runtime admits for a first-use refusal.
const FirstUseKey = "rights.first_use"

// SystemSecurityLogKey is the system-access question for one protected
// Windows Security log page (ASK-S1, logging LOG-N1).
const SystemSecurityLogKey = "system.securitylog.read"

// RuntimeOnlyKey reports whether only a runtime may admit key: its own
// first-use, typed permission and system-access questions and its question
// about a self-registered app (ASK-R2, ASK-S1, ASK-R11).
func RuntimeOnlyKey(key string) bool {
	return key == FirstUseKey || key == TypedPermissionKey || key == SystemSecurityLogKey || key == SelfRegisteredKey
}

// FirstUse reads the program, action and resource of a first-use question from
// its About slot and text. ok is false for any other question or a text that
// does not read as the question's own sentence.
func FirstUse(key, about, text string) (program, action, resource string, ok bool) {
	if key != FirstUseKey || about == "" {
		return "", "", "", false
	}
	rest, found := strings.CutPrefix(text, about+" wants to ")
	if !found {
		return "", "", "", false
	}
	action, resource, found = strings.Cut(rest, " on ")
	if !found || action == "" || resource == "" || strings.ContainsAny(action, " \t") {
		return "", "", "", false
	}
	return about, action, resource, true
}

var slotRE = regexp.MustCompile(`\{([a-z]+)\}`)

func Find(key string) (Question, bool) {
	if key == TypedPermissionKey {
		return Question{Key: TypedPermissionKey, Text: "OA permission request", About: "program", Options: []Option{{Name: "allow", Yes: true}, {Name: "refuse"}}}, true
	}
	for _, q := range Questions {
		if q.Key == key {
			return q, true
		}
	}
	return Question{}, false
}

func (q Question) Option(name string) (Option, bool) {
	options := q.Options
	if q.Key != TypedPermissionKey && !q.Closed {
		options = append(options, Never)
	}
	for _, o := range options {
		if o.Name == name {
			return o, true
		}
	}
	return Option{}, false
}

func (q Question) OptionNames() []string {
	var out []string
	options := q.Options
	if q.Key != TypedPermissionKey && !q.Closed {
		options = append(options, Never)
	}
	for _, o := range options {
		out = append(out, o.Name)
	}
	return out
}

func (q Question) Render(a Ask) (string, error) {
	slots := map[string]string{"asker": a.Asker}
	for k, v := range a.Slots {
		slots[k] = v
	}
	for k, v := range slots {
		if utf8.RuneCountInString(v) > maxSlot || strings.ContainsAny(v, "\r\n") {
			return "", fmt.Errorf("%w %s: one line, at most 200 characters", ErrBadSlot, k)
		}
	}
	var missing []string
	text := slotRE.ReplaceAllStringFunc(q.Text, func(m string) string {
		v, ok := slots[m[1:len(m)-1]]
		if !ok {
			missing = append(missing, m)
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("%w %s not given", ErrBadSlot, strings.Join(missing, ", "))
	}
	return text, nil
}
