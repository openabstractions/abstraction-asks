package catalogue

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ProgramReviewKey is reserved to the service's captured PROGRAM review owner.
const ProgramReviewKey = "registry.program.review"

// ProgramReviewUnsignedChanged is the cause of a review the service admits
// for one unsigned program whose image differs from the image the person
// kept. Such a review names exactly that program, by its kept Title.
const ProgramReviewUnsignedChanged = "unsigned:changed"

type ProgramReviewTarget struct {
	Program   string `json:"program"`
	Revision  string `json:"revision"`
	Image     string `json:"image"`
	FileKey   string `json:"file_key"`
	Spec      string `json:"spec"`
	Cause     string `json:"cause"`
	Supported bool   `json:"supported"`
	Title     string `json:"title,omitempty"`
}

type ProgramReviewFacts struct {
	Targets []ProgramReviewTarget
}

func ProgramReviewSlots(f ProgramReviewFacts) (map[string]string, error) {
	if len(f.Targets) == 0 || len(f.Targets) > 16 {
		return nil, ErrBadSlot
	}
	slots := map[string]string{"version": "1"}
	previous := ""
	for i, target := range f.Targets {
		if target.Program <= previous || !setupFactText(target.Program, 4096) || target.Revision == "" || !setupFactText(target.Revision, 128) ||
			(target.Cause != "shipped:hash-mismatch" && target.Cause != "shipped:unrecorded" && target.Cause != "identity:unavailable" && target.Cause != "approval:pending" && target.Cause != ProgramReviewUnsignedChanged) {
			return nil, ErrBadSlot
		}
		unsigned := target.Cause == ProgramReviewUnsignedChanged
		if unsigned != (target.Title != "") || unsigned && (len(f.Targets) != 1 || !target.Supported || !setupFactText(target.Title, 200)) {
			return nil, ErrBadSlot
		}
		if target.Supported && (!programReviewDigest(target.Image) || !programReviewDigest(target.Spec) || target.FileKey == "" || !setupFactText(target.FileKey, 200)) {
			return nil, ErrBadSlot
		}
		previous = target.Program
		raw, err := setupFactJSON(target)
		if err != nil {
			return nil, err
		}
		slots[fmt.Sprintf("target_%02d", i+1)] = raw
	}
	raw, _ := json.Marshal(f.Targets)
	sum := sha256.Sum256(raw)
	slots["occurrence"] = hex.EncodeToString(sum[:])
	return slots, nil
}

func programReviewDigest(v string) bool {
	raw, err := hex.DecodeString(v)
	return err == nil && len(raw) == 32 && strings.ToLower(v) == v
}

func ProgramReviewFactsFromSlots(slots map[string]string) (ProgramReviewFacts, error) {
	f := ProgramReviewFacts{}
	if slots["version"] != "1" {
		return f, ErrBadSlot
	}
	for i := 0; i < 16; i++ {
		raw, ok := slots[fmt.Sprintf("target_%02d", i+1)]
		if !ok {
			break
		}
		var target ProgramReviewTarget
		if decodeSetupFact(raw, &target) != nil {
			return f, ErrBadSlot
		}
		f.Targets = append(f.Targets, target)
	}
	want, err := ProgramReviewSlots(f)
	if err != nil || len(want) != len(slots) {
		return f, ErrBadSlot
	}
	for key, value := range want {
		if slots[key] != value {
			return f, ErrBadSlot
		}
	}
	return f, nil
}

func ProgramReviewOptions(slots map[string]string) []string {
	f, err := ProgramReviewFactsFromSlots(slots)
	if err != nil {
		return nil
	}
	keep, repair := false, false
	for _, target := range f.Targets {
		keep = keep || target.Supported
		repair = repair || target.Cause == "shipped:hash-mismatch"
	}
	options := []string{}
	if keep {
		options = append(options, "keep")
	}
	options = append(options, "not_now")
	if repair {
		options = append(options, "repair")
	}
	return options
}

// ProgramReviewUnsigned reads the kept title of a review about one unsigned
// program whose image changed. ok is false for every other review.
func ProgramReviewUnsigned(slots map[string]string) (title string, ok bool) {
	f, err := ProgramReviewFactsFromSlots(slots)
	if err != nil || len(f.Targets) != 1 || f.Targets[0].Cause != ProgramReviewUnsignedChanged {
		return "", false
	}
	return f.Targets[0].Title, true
}

func ProgramReviewPresentation(slots map[string]string) (title, instruction, details string, err error) {
	f, err := ProgramReviewFactsFromSlots(slots)
	if err != nil {
		return "", "", "", err
	}
	// Architect popup ruling 2026-11-10, rows 11 and 12: one plain question,
	// one body sentence, and the file facts behind Show details.
	if len(f.Targets) == 1 && f.Targets[0].Cause == ProgramReviewUnsignedChanged {
		target := f.Targets[0]
		return "Use the changed " + target.Title + "?",
			"Use it only if you updated it yourself.",
			"Program: " + target.Program + "\nCurrent file SHA-256: " + target.Image, nil
	}
	lines := []string{}
	for _, target := range f.Targets {
		lines = append(lines, programReviewName(target.Program)+" — "+programReviewCauseWords(target.Cause), "Program: "+target.Program)
	}
	return "Use these changed programs?", "Use them only if you updated them yourself.", strings.Join(lines, "\n"), nil
}

// programReviewName is a program path's file name.
func programReviewName(program string) string {
	if i := strings.LastIndexAny(program, `\/`); i >= 0 && i+1 < len(program) {
		return program[i+1:]
	}
	return program
}

// programReviewCauseWords merges the review causes into the two a person can
// act on: a file that differs from the one OpenAbstractions shipped, and one
// OpenAbstractions could not check.
func programReviewCauseWords(cause string) string {
	if cause == "shipped:hash-mismatch" {
		return "differs from the file OpenAbstractions shipped"
	}
	return "couldn't be checked"
}
