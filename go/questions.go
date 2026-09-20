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
}

var Never = Option{Name: "never", Kept: true}

var Questions = []Question{
	{Key: "rights.register",
		Text:    "{asker} wants to: {rights}",
		Options: []Option{{Name: "allow", Yes: true}, {Name: "refuse"}}},
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
}

// FirstUseKey is the question a runtime admits for a first-use refusal.
const FirstUseKey = "rights.first_use"

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
	for _, q := range Questions {
		if q.Key == key {
			return q, true
		}
	}
	return Question{}, false
}

func (q Question) Option(name string) (Option, bool) {
	for _, o := range append(q.Options, Never) {
		if o.Name == name {
			return o, true
		}
	}
	return Option{}, false
}

func (q Question) OptionNames() []string {
	var out []string
	for _, o := range append(q.Options, Never) {
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
