package asks

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/openabstractions/abstraction-identity/listen"
)

func TestFirstUseAnswerResourceIsExactAndBounded(t *testing.T) {
	program := filepath.Join(t.TempDir(), "opencode.exe")
	encoded, _ := json.Marshal(program)
	a, err := FirstUseAnswerResource(program, "abstraction.model/lookup", "hf", "allow")
	if err != nil || !strings.HasPrefix(a, "first-use@1:["+string(encoded)+",") {
		t.Fatalf("canonical answer resource %q %v", a, err)
	}
	b, err := FirstUseAnswerResource(program, "abstraction.model/lookup", "hf", "refuse")
	if err != nil || b == a {
		t.Fatalf("option scope %q %v", b, err)
	}
	if _, err := FirstUseAnswerResource("app", "abstraction.model/lookup", "hf", "never"); err == nil {
		t.Fatal("unknown option accepted")
	}
	if _, err := FirstUseAnswerResource(strings.Repeat("界", 200), "abstraction.model/lookup", strings.Repeat("界", 200), "allow"); err == nil {
		t.Fatal("oversize rights resource accepted")
	}
}

func TestAnswerResourcePreservesLegacyBytesAndSelectsOneEncoding(t *testing.T) {
	program := "/apps/app.exe"
	golden := `first-use@1:["/apps/app.exe","abstraction.model/lookup","hf\u003c\u0026\u003e\"\\","allow"]`
	if runtime.GOOS == "windows" {
		program = `C:\apps\app.exe`
		golden = `first-use@1:["C:\\apps\\app.exe","abstraction.model/lookup","hf\u003c\u0026\u003e\"\\","allow"]`
	}
	got, err := FirstUseAnswerResource(program, "abstraction.model/lookup", `hf<&>"\`, "allow")
	if err != nil || got != golden {
		t.Fatalf("legacy bytes %q want %q: %v", got, golden, err)
	}
	// Exactly 200 runes and 800 UTF-8 bytes remains representable.
	resource := strings.Repeat("😀", 200)
	got, err = FirstUseAnswerResource(program, "abstraction.model/lookup", resource, "allow")
	encoded, _ := json.Marshal([4]string{program, "abstraction.model/lookup", resource, "allow"})
	if err != nil || got != "first-use@1:"+string(encoded) {
		t.Fatalf("200 rune/800 byte legacy boundary: %q %v", got, err)
	}
	base := filepath.VolumeName(t.TempDir()) + string(filepath.Separator) + "apps" + string(filepath.Separator)
	suffix := string(filepath.Separator) + "app.exe"
	boundaryProgram := base + strings.Repeat("p", 200-utf8.RuneCountInString(base+suffix)) + suffix
	got, err = FirstUseAnswerResource(boundaryProgram, "abstraction.model/lookup", "hf", "allow")
	encoded, _ = json.Marshal([4]string{boundaryProgram, "abstraction.model/lookup", "hf", "allow"})
	if err != nil || got != "first-use@1:"+string(encoded) {
		t.Fatalf("200 rune program boundary %q %v", got, err)
	}
	if beyond, err := FirstUseAnswerResource(boundaryProgram+"x", "abstraction.model/lookup", "hf", "allow"); err != nil || !strings.HasPrefix(beyond, "first-use@2:") {
		t.Fatalf("201 rune program boundary %q %v", beyond, err)
	}
	// JSON's HTML escaping can cross the final bound before either field bound.
	encoded, _ = json.Marshal([4]string{program, "abstraction.model/lookup", "", "allow"})
	resource = strings.Repeat("<", (1024-len("first-use@1:")-len(encoded))/6)
	encoded, _ = json.Marshal([4]string{program, "abstraction.model/lookup", resource, "allow"})
	resource += strings.Repeat("x", 1024-len("first-use@1:")-len(encoded))
	if len([]rune(resource)) > 200 {
		t.Fatal("bad boundary fixture")
	}
	got, err = FirstUseAnswerResource(program, "abstraction.model/lookup", resource, "allow")
	if err != nil || len(got) != 1024 || !strings.HasPrefix(got, "first-use@1:") {
		t.Fatalf("1024 boundary: %d %v", len(got), err)
	}
	digest, err := FirstUseAnswerResource(program, "abstraction.model/lookup", resource+"x", "allow")
	if err != nil || len(digest) != 76 || !strings.HasPrefix(digest, "first-use@2:") {
		t.Fatalf("1025 encoding: %q %v", digest, err)
	}
	long := filepath.Join(t.TempDir(), strings.Repeat("p", 220), "app.exe")
	digest, err = FirstUseAnswerResource(long, "abstraction.model/lookup", strings.Repeat("r", 1024), "allow")
	other, otherErr := FirstUseAnswerResource(long, "abstraction.model/lookup", strings.Repeat("r", 1024), "refuse")
	if err != nil || otherErr != nil || digest == other || !strings.HasPrefix(digest, "first-use@2:") {
		t.Fatalf("long scope %q %q %v %v", digest, other, err, otherErr)
	}
	for _, args := range [][4]string{{"relative.exe", "abstraction.model/lookup", "hf", "allow"}, {long, "abstraction.model/lookup", strings.Repeat("r", 1025), "allow"}, {long, "bad action", "hf", "allow"}, {long, "abstraction.model/lookup", "hf", "never"}} {
		if _, err := FirstUseAnswerResource(args[0], args[1], args[2], args[3]); err == nil {
			t.Fatalf("malformed tuple fell back to digest: %+v", args)
		}
	}
}

func TestScopedBookAnswerUsesOriginalAdmission(t *testing.T) {
	b, err := LoadApplicationBook(t.TempDir() + "/asks.json")
	if err != nil {
		t.Fatal(err)
	}
	record, outcome, err := b.AskApplication("runtime-scope", "runtime", "first-use", FirstUseKey,
		map[string]string{"program": filepath.Join(t.TempDir(), "app.exe"), "action": "abstraction.model/lookup", "resource": "hf"}, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatalf("admit %+v %s %v", record, outcome, err)
	}
	_, _, err = b.AnswerApplicationScopedAuthorized(record.ID, "allow", func(_ Record, scope string) error {
		if scope != "runtime-scope" {
			t.Errorf("scope %q", scope)
		}
		return ErrUnknownQuestion
	})
	if err == nil {
		t.Fatal("refused callback answered")
	}
	answered, outcome, err := b.AnswerApplicationScopedAuthorized(record.ID, "allow", func(current Record, scope string) error {
		if scope != "runtime-scope" || current.ID != record.ID || !current.Pending() {
			t.Fatalf("callback current %+v %q", current, scope)
		}
		return nil
	})
	if err != nil || outcome != "answered" || answered.Option != "allow" {
		t.Fatalf("answer %+v %s %v", answered, outcome, err)
	}
	replayed, outcome, err := b.AnswerApplicationScopedAuthorized(record.ID, "allow", func(current Record, scope string) error {
		if scope != "runtime-scope" || current.Pending() {
			t.Fatalf("replay current %+v %q", current, scope)
		}
		return nil
	})
	if err != nil || outcome != "answered" || !replayed.Answered.Equal(answered.Answered) {
		t.Fatalf("replay %+v %s %v", replayed, outcome, err)
	}
	_, outcome, err = b.AnswerApplicationScopedAuthorized(record.ID, "refuse", func(Record, string) error { return nil })
	if err != nil || outcome != "conflict" {
		t.Fatalf("different option %s %v", outcome, err)
	}
}
