package asks

import (
	"errors"
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
			return "", errors.New(ErrBadSlot.Error() + " " + k + ": one line, at most 200 characters")
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
		return "", errors.New(ErrBadSlot.Error() + " " + strings.Join(missing, ", ") + " not given")
	}
	return text, nil
}
