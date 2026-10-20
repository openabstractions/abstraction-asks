package catalogue

import (
	"maps"
	"strings"
	"testing"
)

func TestQuestionPopupChoices(t *testing.T) {
	for _, test := range []struct {
		key                  string
		labels               map[string]string
		defaultOption, close string
	}{
		{"content.replace", map[string]string{"once": "Allow"}, "", "Not now"},
		{"download.reach", map[string]string{"allow": "Allow", "refuse": "Not now"}, "refuse", ""},
		{projectEditQuestionKey, map[string]string{"once": "Allow", "refuse": "Not now"}, "refuse", ""},
	} {
		t.Run(test.key, func(t *testing.T) {
			got, ok := QuestionPopupChoices(test.key)
			if !ok || !maps.Equal(got.Labels, test.labels) || got.DefaultOption != test.defaultOption || got.CloseLabel != test.close {
				t.Fatalf("popup choice policy = %+v, found %v", got, ok)
			}
			question, found := Find(test.key)
			if !found {
				t.Fatal("popup policy has no catalogue question")
			}
			for option := range got.Labels {
				if _, found := question.Option(option); !found {
					t.Fatalf("popup policy names an unrecorded option %q", option)
				}
			}
		})
	}
	if _, ok := QuestionPopupChoices("rights.register"); ok {
		t.Fatal("ordinary catalogue questions must retain the presenter's fallback")
	}
}

// Each popup reads as one question naming the app and the action; codes,
// expiry, paths and replacements stay behind Show details (architect popup
// ruling 2026-11-10, rows 7 and 24 to 26).
func TestQuestionPresentationHeadingsKeepValuesInDetails(t *testing.T) {
	request := map[string]string{"code": "c0ffee/1234", "until": "2026-11-10T12:00:00Z"}
	with := func(extra map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range request {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	for _, c := range []struct {
		key, heading, body string
		slots              map[string]string
		details            []string
	}{
		{SystemSecurityLogKey, "Allow OpenCode to read one page of the Windows Security log?", "Windows asks for administrator approval next.",
			with(nil), []string{"Request: c0ffee/1234", "Expires: 2026-11-10T12:00:00Z"}},
		{"content.replace", "Allow OpenCode to replace the saved note once?", "",
			with(map[string]string{"target": "the saved note", "replacement": "the new note"}), []string{"Replacement: the new note", "Request: c0ffee/1234"}},
		{"download.reach", "Allow OpenCode to download from example.com?", "", map[string]string{"host": "example.com"}, nil},
		{projectEditQuestionKey, "Allow OpenCode to change files in proj?", "Until this session ends.",
			with(map[string]string{"folder": `C:\work\proj`}), []string{`Folder: C:\work\proj`, "including its subfolders", "Expires: 2026-11-10T12:00:00Z"}},
		{ApplicationCommandOnceKey, "Allow OpenCode to open ComfyUI this once?", "",
			with(map[string]string{"operation": "Open ComfyUI", "operationmore": "", "program": `C:\oc\opencode.exe`, "command": "open"}), []string{`Program: C:\oc\opencode.exe`, "Command: open"}},
	} {
		view, ok := QuestionPresentation(c.key, "OpenCode", c.slots)
		if !ok || view.Heading != c.heading || view.Body != c.body {
			t.Fatalf("%s: %+v %v", c.key, view, ok)
		}
		for _, want := range c.details {
			if !strings.Contains(view.Details, want) {
				t.Fatalf("%s details lack %q: %q", c.key, want, view.Details)
			}
		}
		for _, slot := range []string{"code", "until", "folder", "program", "replacement"} {
			if value := c.slots[slot]; value != "" && strings.Contains(view.Heading+view.Body, value) {
				t.Fatalf("%s shows its %s outside its details", c.key, slot)
			}
		}
	}
	if _, ok := QuestionPresentation("rights.register", "OpenCode", map[string]string{"rights": "x"}); ok {
		t.Fatal("a question without popup words was worded")
	}
	if _, ok := QuestionPresentation(SystemSecurityLogKey, "OpenCode", map[string]string{"code": "x"}); ok {
		t.Fatal("a question missing a slot was worded")
	}
	if got := folderName(`C:\`); got != `C:\` {
		t.Fatal(got)
	}
}
