package catalogue

import (
	"strings"
	"testing"
)

func unsignedReviewTarget() ProgramReviewTarget {
	return ProgramReviewTarget{Program: `C:\Users\person\OAComfy\launch-helper.exe`, Revision: "revision", FileKey: "native-file",
		Image: strings.Repeat("a", 64), Spec: strings.Repeat("b", 64), Cause: ProgramReviewUnsignedChanged, Supported: true, Title: "ComfyUI"}
}

// The review of a changed unsigned program names that one program by the
// title the person kept, offers Keep and Not now, and shows its path only in
// the details.
func TestUnsignedProgramReviewWordsAndOptions(t *testing.T) {
	slots, err := ProgramReviewSlots(ProgramReviewFacts{Targets: []ProgramReviewTarget{unsignedReviewTarget()}})
	if err != nil {
		t.Fatal(err)
	}
	title, instruction, details, err := ProgramReviewPresentation(slots)
	if err != nil || title != "Use the changed ComfyUI?" || instruction != "Use it only if you updated it yourself." ||
		details != `Program: C:\Users\person\OAComfy\launch-helper.exe`+"\nCurrent file SHA-256: "+strings.Repeat("a", 64) {
		t.Fatalf("%q %q %q %v", title, instruction, details, err)
	}
	if options := ProgramReviewOptions(slots); len(options) != 2 || options[0] != "keep" || options[1] != "not_now" {
		t.Fatal(options)
	}
	if name, ok := ProgramReviewUnsigned(slots); !ok || name != "ComfyUI" {
		t.Fatal(name, ok)
	}
	if strings.Contains(title+instruction, strings.Repeat("a", 64)) {
		t.Fatal("the review's visible words show the image hash")
	}
}

// An unsigned review is one supported program with a title; a title belongs
// to no other cause, and existing shipped reviews keep their encoding.
func TestUnsignedProgramReviewFactsAreBounded(t *testing.T) {
	shipped := ProgramReviewTarget{Program: `C:\fixture\provider.exe`, Revision: "revision", FileKey: "fixture-native-file", Image: strings.Repeat("a", 64), Spec: strings.Repeat("b", 64), Cause: "shipped:hash-mismatch", Supported: true}
	for name, targets := range map[string][]ProgramReviewTarget{
		"no title":         {func() ProgramReviewTarget { v := unsignedReviewTarget(); v.Title = ""; return v }()},
		"unsupported":      {func() ProgramReviewTarget { v := unsignedReviewTarget(); v.Supported = false; return v }()},
		"title on shipped": {func() ProgramReviewTarget { v := shipped; v.Title = "Provider"; return v }()},
		"grouped":          {shipped, func() ProgramReviewTarget { v := unsignedReviewTarget(); v.Program = `C:\z.exe`; return v }()},
	} {
		if _, err := ProgramReviewSlots(ProgramReviewFacts{Targets: targets}); err == nil {
			t.Fatal(name, "accepted")
		}
	}
	slots, err := ProgramReviewSlots(ProgramReviewFacts{Targets: []ProgramReviewTarget{shipped}})
	if err != nil || strings.Contains(slots["target_01"], "title") {
		t.Fatal("a shipped review changed its encoding", slots, err)
	}
	if _, ok := ProgramReviewUnsigned(slots); ok {
		t.Fatal("a shipped review read as unsigned")
	}
	if title, _, _, _ := ProgramReviewPresentation(slots); title != "Use these changed programs?" {
		t.Fatal(title)
	}
}
