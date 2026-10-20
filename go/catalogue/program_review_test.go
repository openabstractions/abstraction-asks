package catalogue

import (
	"maps"
	"strings"
	"testing"
)

func TestProgramReviewFactsAreBoundedCanonicalAndRuntimeOwned(t *testing.T) {
	f := ProgramReviewFacts{Targets: []ProgramReviewTarget{{Program: `C:\fixture\provider.exe`, Revision: "revision", FileKey: "fixture-native-file", Image: strings.Repeat("a", 64), Spec: strings.Repeat("b", 64), Cause: "shipped:hash-mismatch", Supported: true}}}
	slots, err := ProgramReviewSlots(f)
	if err != nil {
		t.Fatal(err)
	}
	if !RuntimeOnlyKey(ProgramReviewKey) {
		t.Fatal("application could admit native review")
	}
	q, ok := Find(ProgramReviewKey)
	if !ok || !q.RuntimeBound || !q.Closed {
		t.Fatal(q)
	}
	for _, mutate := range []func(map[string]string){func(v map[string]string) { v["occurrence"] = strings.Repeat("f", 64) }, func(v map[string]string) { v["target_01"] += " " }, func(v map[string]string) { v["target_03"] = v["target_01"] }, func(v map[string]string) { v["unknown"] = "data" }} {
		bad := maps.Clone(slots)
		mutate(bad)
		if _, err := ProgramReviewFactsFromSlots(bad); err == nil {
			t.Fatal("changed facts accepted", bad)
		}
	}
	if _, err := ProgramReviewSlots(ProgramReviewFacts{Targets: append(f.Targets, f.Targets...)}); err == nil {
		t.Fatal("duplicate target accepted")
	}
	f.Targets[0].Program = strings.Repeat("x", 900)
	if _, err := ProgramReviewSlots(f); err == nil {
		t.Fatal("oversized carrier accepted")
	}
	for _, digest := range []string{strings.Repeat("A", 64), strings.Repeat("g", 64), strings.Repeat("a", 63), strings.Repeat("a", 65), "../image"} {
		for _, field := range []string{"image", "spec"} {
			bad := ProgramReviewTarget{Program: `C:\fixture\provider.exe`, Revision: "revision", FileKey: "fixture-native-file", Image: strings.Repeat("a", 64), Spec: strings.Repeat("b", 64), Cause: "shipped:hash-mismatch", Supported: true}
			if field == "image" {
				bad.Image = digest
			} else {
				bad.Spec = digest
			}
			if _, err := ProgramReviewSlots(ProgramReviewFacts{Targets: []ProgramReviewTarget{bad}}); err == nil {
				t.Fatal("malformed canonical digest accepted", field, digest)
			}
		}
	}
}

func TestProgramReviewPresentationUsesPlainCauseWords(t *testing.T) {
	for cause, words := range map[string]string{
		"shipped:hash-mismatch": "provider.exe — differs from the file OpenAbstractions shipped",
		"shipped:unrecorded":    "provider.exe — couldn't be checked",
		"identity:unavailable":  "provider.exe — couldn't be checked",
		"approval:pending":      "provider.exe — couldn't be checked",
	} {
		t.Run(cause, func(t *testing.T) {
			slots, err := ProgramReviewSlots(ProgramReviewFacts{Targets: []ProgramReviewTarget{{Program: `C:\fixture\provider.exe`, Revision: "current", Cause: cause}}})
			if err != nil {
				t.Fatal(err)
			}
			title, instruction, details, err := ProgramReviewPresentation(slots)
			if err != nil || title != "Use these changed programs?" || instruction != "Use them only if you updated them yourself." ||
				!strings.Contains(details, words) || strings.Contains(details, cause) || !strings.Contains(details, `C:\fixture\provider.exe`) {
				t.Fatal(title, instruction, details, err)
			}
		})
	}
	for _, cause := range []string{"", "unknown:internal-code", "<script>untrusted</script>", "untrusted\ntext"} {
		if got := programReviewCauseWords(cause); got != "couldn't be checked" {
			t.Fatal("untrusted cause reached presentation", got)
		}
	}
}
