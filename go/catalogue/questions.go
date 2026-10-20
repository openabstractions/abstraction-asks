package catalogue

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrUnknownQuestion = errors.New("asks: no such question")
	ErrUnknownOption   = errors.New("asks: no such option")
	ErrBadSlot         = errors.New("asks: slot")
)

const maxSlot = 200
const projectEditQuestionKey = "opencode.project.edit"

func validQuestionSlot(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maxSlot && strings.IndexFunc(value, unicode.IsControl) < 0
}

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
	{Key: ProgramReviewKey, Text: "Review updated programs (native presentation from retained image facts).", About: "occurrence", Options: []Option{{Name: "keep", Yes: true}, {Name: "not_now"}, {Name: "repair"}}, Closed: true, RuntimeBound: true},
	{Key: SetupModelsKey,
		Text: "Set up models (native presentation from retained Setup facts).", About: "occurrence", Options: []Option{{Name: "setup", Yes: true}, {Name: "not_now"}}, Closed: true},
	{Key: AgentUpdateKey,
		Text:  "Update the {members}{membersmore}{memberslast} connections? Keep these updated connections as your assistants. Each access permission is reviewed separately. Restart {members}{membersmore}{memberslast} to finish. Request {code}.",
		About: "code", Options: []Option{{Name: "update", Yes: true}, {Name: "not_now"}}, Closed: true},
	// Native application mediation binds this runtime-only occurrence to
	// current peer, manifest, instance and receipt state. Its answer writes
	// no policy; standing permits have their separate permission owner.
	{Key: ApplicationCommandOnceKey,
		Text:  "{operation}{operationmore}? {program} requests this once. Command {command}. Request {code} expires {until}.",
		About: "program", Options: []Option{{Name: "once", Yes: true}, {Name: "refuse"}}, Closed: true, RuntimeBound: true},
	// Facade binds this runtime-only choice to an exact registration
	// generation. Each access permission is a separate question.
	{Key: AgentEnrollmentKey,
		Text:    "Connect {title}?",
		About:   "program",
		Options: []Option{{Name: "keep", Yes: true}, {Name: "refuse"}},
		Closed:  true},
	// Owner-approved directional copy. The runtime binds these display slots
	// to its exact frozen agreement; the answer alone grants nothing.
	{Key: PairingKey,
		Text:         "Pair these computers? Check that both computers show {check}. This computer allows: {given} {givenmore} The other computer allows: {received} {receivedmore} Permissions stay until removed in the Panel. {givenreplacement} {receivedreplacement} Each computer approves separately. Details: Verified peer key {peer}. Local binding {binding}. This computer's source: {givensource}. The other computer's source: {receivedsource}. Request {code} expires {until}.",
		About:        "code",
		Options:      []Option{{Name: "once", Yes: true}, {Name: "refuse"}},
		Closed:       true,
		RuntimeBound: true},
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
	{Key: projectEditQuestionKey,
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
	// RIGHTS-R22). Allow (recorded as keep) confirms the entry; Don't allow (recorded as remove) removes it and blocks the
	// program from registering itself again. Closing or expiry answers
	// nothing and the entry stays unconfirmed. Only the person answers: it
	// vouches for app-written title text, so no answerer may (ASK-R11).
	// Applications cannot admit it (RuntimeOnlyKey). The About slot is the
	// path of the program that registered itself, bound from its connection;
	// SelfRegistered reads the program and title back.
	{Key: SelfRegisteredKey,
		Text:    "Let {title} work with OpenAbstractions?",
		About:   "program",
		Options: []Option{{Name: "keep", Yes: true}, {Name: "remove"}},
		Closed:  true},
	// The runtime admits this question as itself for one typed rights change
	// an operator program other than the Panel window requested (rights
	// RIGHTS-O17, architect ruling A of 2026-11-22). Only the person answers
	// it: no responder lists it, and the runtime applies the change only on an
	// operator Allow from the Panel window or its presenter. The slots are the
	// service's own words for the exact change it holds in memory, so a
	// restarted runtime withdraws it while pending.
	{Key: RightsChangeKey,
		Text:         "Let {requester} {change}? Program {program}. Action {action}. Resource {resource}. Request {code} expires {until}.",
		About:        "code",
		Options:      []Option{{Name: "allow", Yes: true}, {Name: "not_now"}},
		Closed:       true,
		RuntimeBound: true},
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

// SelfRegistered reads the program and title of a self-registration question
// from its About slot and text. ok is false for any other question or a text
// that does not read as the question's own sentence.
func SelfRegistered(key, about, text string) (program, title string, ok bool) {
	if key != SelfRegisteredKey || about == "" {
		return "", "", false
	}
	// Questions retained from earlier releases keep their original wording.
	if rest, found := strings.CutPrefix(text, "Let "); found {
		title, found = strings.CutSuffix(rest, " work with OpenAbstractions?")
		if found && title != "" {
			return about, title, true
		}
		return "", "", false
	}
	rest, found := strings.CutPrefix(text, about+" was added to OpenAbstractions as ")
	if !found {
		rest, found = strings.CutPrefix(text, about+" added itself to OpenAbstractions as ")
	}
	if !found {
		return "", "", false
	}
	title, found = strings.CutSuffix(rest, ". Keep it?")
	if !found || title == "" {
		return "", "", false
	}
	return about, title, true
}

// SelfRegisteredKey is the existing person-only Keep question for a pending
// program registration, including a legacy self-registered application.
const SelfRegisteredKey = "application.self_registered"

const AgentEnrollmentKey = "application.agent_enrollment"

const AgentUpdateKey = "application.agent_update"

// SetupModelsKey is the runtime's person-only Setup question. It grants no right.
const SetupModelsKey = "setup.models"

// FirstUseKey is the question a runtime admits for a first-use refusal.
const FirstUseKey = "rights.first_use"

// SystemSecurityLogKey is the system-access question for one protected
// Windows Security log page (ASK-S1, logging LOG-N1).
const SystemSecurityLogKey = "system.securitylog.read"

// PairingKey is the runtime's exact frozen directional pairing occurrence.
// Its answer alone grants nothing; the runtime correlates its native answer,
// transport transcript and conditional receiving effect under the Book pin.
const PairingKey = "runtime.machine.pair"

// RightsChangeKey is the runtime's person-only question for one typed rights
// change an operator program requested (rights RIGHTS-O17).
const RightsChangeKey = "rights.change"

// ApplicationCommandOnceKey is admitted only by the native command enforcer.
// Ordinary/responding applications cannot answer it; it deliberately stays
// outside FirstUseKey's persistent permission shape.
const ApplicationCommandOnceKey = "application.command.once"

// RuntimeOnlyKey reports whether only a runtime may admit key: its own
// first-use, typed permission and system-access questions and its question
// about a self-registered app (ASK-R2, ASK-S1, ASK-R11).
func RuntimeOnlyKey(key string) bool {
	return key == ProgramReviewKey || key == SetupModelsKey || key == FirstUseKey || key == TypedPermissionKey || key == SystemSecurityLogKey || key == SelfRegisteredKey || key == AgentEnrollmentKey || key == AgentUpdateKey || key == PairingKey || key == ApplicationCommandOnceKey || key == RightsChangeKey
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
	if q.Key == ProgramReviewKey {
		_, instruction, _, err := ProgramReviewPresentation(a.Slots)
		return instruction, err
	}
	if q.Key == SetupModelsKey {
		view, err := SetupQuestionPresentation(a.Slots)
		return view.Instruction, err
	}
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

// Ask is the catalogue rendering input; native caller evidence stays in the Book.
type Ask struct {
	Asker string
	Key   string
	Slots map[string]string
}

const TypedPermissionKey = "rights.permission@2"
const MaxSlot = maxSlot

func ValidQuestionSlot(value string) bool { return validQuestionSlot(value) }

func ApplicationQuestion(display, key string, slots map[string]string) (Ask, string, error) {
	q, ok := Find(key)
	if !ok {
		return Ask{}, "", ErrUnknownQuestion
	}
	if key == ProgramReviewKey {
		if _, err := ProgramReviewFactsFromSlots(slots); err != nil {
			return Ask{}, "", err
		}
		a := Ask{Asker: display, Key: key, Slots: slots}
		text, err := q.Render(a)
		return a, text, err
	}
	if key == SetupModelsKey {
		facts, err := SetupFactsFromSlots(slots)
		if err != nil || facts.Occurrence == "" {
			return Ask{}, "", ErrBadSlot
		}
		a := Ask{Asker: display, Key: key, Slots: slots}
		text, err := q.Render(a)
		return a, text, err
	}
	required := map[string]bool{}
	if q.About != "" {
		required[q.About] = true
	}
	for _, m := range slotRE.FindAllStringSubmatch(q.Text, -1) {
		if m[1] != "asker" {
			required[m[1]] = true
		}
	}
	if len(slots) != len(required) {
		return Ask{}, "", ErrBadSlot
	}
	for k, v := range slots {
		if !required[k] || !validQuestionSlot(v) {
			return Ask{}, "", ErrBadSlot
		}
	}
	a := Ask{Asker: display, Key: key, Slots: slots}
	text, e := q.Render(a)
	return a, text, e
}
