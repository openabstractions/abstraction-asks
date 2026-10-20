package catalogue

import (
	"strings"
	"testing"
)

func TestQuestionSummaryCoversTheCatalogue(t *testing.T) {
	keys := map[string]bool{TypedPermissionKey: true}
	for _, q := range Questions {
		keys[q.Key] = true
	}
	if len(questionSummaries) != len(keys) {
		t.Fatalf("summary catalogue has %d entries for %d keys", len(questionSummaries), len(keys))
	}
	for key := range keys {
		spec, ok := questionSummaries[key]
		if !ok || spec.template == "" || spec.fallback == "" {
			t.Fatalf("summary missing for %s", key)
		}
		rest := strings.NewReplacer("{folder.leaf}", "", "{host}", "").Replace(spec.template)
		if strings.ContainsAny(rest, "{}") || strings.ContainsAny(spec.fallback, "{}") {
			t.Fatalf("summary %s has an unapproved display slot", key)
		}
	}
	want := map[string]string{
		ApplicationCommandOnceKey: "Asked a question", AgentEnrollmentKey: "Asked a question",
		PairingKey: "Asked to pair computers", "content.replace": "Asked to replace stored content once",
		"rights.register": "Asked for permissions", FirstUseKey: "Asked a question",
		SelfRegisteredKey: "Added to OpenAbstractions", SystemSecurityLogKey: "Asked to read the Windows Security log",
		TypedPermissionKey: "Asked for permissions", "unknown.question": "Asked a question",
	}
	for key, text := range want {
		if got := (Question{Key: key}).Summary("private request text"); got != text {
			t.Errorf("%s summary %q, want %q", key, got, text)
		}
	}
	q, _ := Find("content.replace")
	q.About, q.Text = "folder", "{rights}"
	if got := q.Summary("private/path"); got != want[q.Key] {
		t.Fatalf("summary inferred from prompt metadata: %q", got)
	}
}

func TestQuestionSummaryDisplaySlots(t *testing.T) {
	folder, _ := Find("opencode.project.edit")
	host, _ := Find("download.reach")
	for _, value := range []string{`C:\Users\person\site`, "/home/person/site", `C:\Users/person\site/`, "site"} {
		if got := folder.Summary(value); got != "Asked to edit files in site" {
			t.Errorf("folder %q summary %q", value, got)
		}
	}
	for _, value := range []string{"example.com", "localhost:8080", "[::1]", "münich.example"} {
		if got := host.Summary(value); got != "Asked to fetch from "+value {
			t.Errorf("host %q summary %q", value, got)
		}
	}
	for _, value := range []string{"", "/", `C:\`, `\\`, ".", "..", "https://person:secret@example.com/private", "folder\nprivate", "folder\u202eprivate", string([]byte{0xff}), strings.Repeat("x", maxSlot+1)} {
		if got := folder.Summary(value); got != "Asked to edit files in a folder" {
			t.Errorf("unsafe folder %q summary %q", value, got)
		}
	}
	for _, value := range []string{"", "https://person:secret@example.com/private?token=secret", "person:secret@example.com", "example.com/private", "example.com?secret", "example.com#secret", "example.com&token=secret", "example.com∕private", "example.com\u00a0private", `example.com\private`, "example.com\nprivate", "example.com\u202eprivate", ":443", "..", string([]byte{0xff}), strings.Repeat("x", maxSlot+1)} {
		if got := host.Summary(value); got != "Asked to fetch from the internet" {
			t.Errorf("unsafe host %q summary %q", value, got)
		}
	}
}
