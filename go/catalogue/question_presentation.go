package catalogue

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// QuestionView is a catalogue question's native popup words, rendered from
// the occurrence's retained slots (architect popup ruling 2026-11-10): one
// plain question naming the app and the action, at most one short body
// sentence, and the exact values behind Show details. The stored record text
// is unchanged; every client that reads the record still reads it.
type QuestionView struct {
	Heading string
	Body    string
	Details string
}

// PopupChoicePolicy describes display labels for selected recorded options.
// DefaultOption names a recorded refusal. CloseLabel instead describes a
// neutral close control that sends no answer. The presenter owns the current
// occurrence's options and display order.
type PopupChoicePolicy struct {
	Labels        map[string]string
	DefaultOption string
	CloseLabel    string
}

// QuestionPopupChoices reads the native button policy for the catalogue
// questions covered by the 2026-11-10 popup ruling, rows 24 to 26.
func QuestionPopupChoices(key string) (PopupChoicePolicy, bool) {
	switch key {
	case "content.replace":
		return PopupChoicePolicy{Labels: map[string]string{"once": "Allow"}, CloseLabel: "Not now"}, true
	case "download.reach":
		return PopupChoicePolicy{Labels: map[string]string{"allow": "Allow", "refuse": "Not now"}, DefaultOption: "refuse"}, true
	case projectEditQuestionKey:
		return PopupChoicePolicy{Labels: map[string]string{"once": "Allow", "refuse": "Not now"}, DefaultOption: "refuse"}, true
	}
	return PopupChoicePolicy{}, false
}

// QuestionPresentation words the popup of a question whose catalogue entry
// has popup words; ok is false for every other key and for slots that do not
// carry every value the words need. asker is the requester's display name.
func QuestionPresentation(key, asker string, slots map[string]string) (QuestionView, bool) {
	if asker == "" || slots == nil {
		return QuestionView{}, false
	}
	has := func(names ...string) bool {
		for _, name := range names {
			if strings.TrimSpace(slots[name]) == "" {
				return false
			}
		}
		return true
	}
	request := func() []string {
		return []string{"Request: " + slots["code"], "Expires: " + slots["until"]}
	}
	switch key {
	case SystemSecurityLogKey:
		if !has("code", "until") {
			return QuestionView{}, false
		}
		return QuestionView{
			Heading: "Allow " + asker + " to read one page of the Windows Security log?",
			Body:    "Windows asks for administrator approval next.",
			Details: strings.Join(request(), "\n"),
		}, true
	case "content.replace":
		if !has("target", "replacement", "code", "until") {
			return QuestionView{}, false
		}
		return QuestionView{
			Heading: "Allow " + asker + " to replace " + slots["target"] + " once?",
			Details: strings.Join(append([]string{"Replacement: " + slots["replacement"]}, request()...), "\n"),
		}, true
	case "download.reach":
		if !has("host") {
			return QuestionView{}, false
		}
		return QuestionView{Heading: "Allow " + asker + " to download from " + slots["host"] + "?"}, true
	case projectEditQuestionKey:
		if !has("folder", "code", "until") {
			return QuestionView{}, false
		}
		folder := slots["folder"]
		return QuestionView{
			Heading: "Allow " + asker + " to change files in " + folderName(folder) + "?",
			Body:    "Until this session ends.",
			Details: strings.Join(append([]string{"Folder: " + folder,
				"It can read, run, create, edit and delete files in this folder, including its subfolders."}, request()...), "\n"),
		}, true
	case ApplicationCommandOnceKey:
		if !has("operation", "command", "code", "until") {
			return QuestionView{}, false
		}
		operation := lowerInitial(slots["operation"] + slots["operationmore"])
		details := []string{}
		if program := slots["program"]; program != "" {
			details = append(details, "Program: "+program)
		}
		details = append(details, "Command: "+slots["command"])
		return QuestionView{
			Heading: "Allow " + asker + " to " + operation + " this once?",
			Details: strings.Join(append(details, request()...), "\n"),
		}, true
	}
	return QuestionView{}, false
}

// folderName is a folder path's last element, the name a person knows the
// folder by; a drive root keeps its whole path.
func folderName(path string) string {
	trimmed := strings.TrimRight(path, `\/`)
	if i := strings.LastIndexAny(trimmed, `\/`); i >= 0 && i+1 < len(trimmed) {
		return trimmed[i+1:]
	}
	if trimmed == "" || strings.HasSuffix(trimmed, ":") {
		return path
	}
	return trimmed
}

// lowerInitial lowers the first letter of an operation and preserves a leading
// acronym ("OA build").
func lowerInitial(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || !unicode.IsUpper(r) {
		return s
	}
	if next, _ := utf8.DecodeRuneInString(s[size:]); unicode.IsUpper(next) {
		return s
	}
	return string(unicode.ToLower(r)) + s[size:]
}
