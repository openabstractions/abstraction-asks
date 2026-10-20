package catalogue

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Setup facts are native catalogue data, never caller-authored question prose
// or permission evidence. The existing retained-content wire carrier stays at
// 32 slots of 800 bytes; sixteen target rows use thirteen other fact slots.
type SetupQuestionFacts struct {
	Occurrence            string
	Requester, RequestUse string
	Targets               []SetupTargetFact
	Code                  *SetupCodeFact
	Picture               *SetupPictureFact
	// Video is a reviewed video model set an app's request for use video
	// asks about. It names no target: the answer grants no app anything.
	Video *SetupVideoFact
}
type SetupTargetFact struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Program  string `json:"program"`
	Revision string `json:"revision"`
	Rules    string `json:"rules"`
	Use      string `json:"use"`
	Chat     bool   `json:"chat"`
}
type SetupModelFact struct {
	Family         string `json:"family"`
	CatalogVersion string `json:"catalog_version,omitempty"`
	Digest         string `json:"digest,omitempty"`
	Plan           string `json:"plan,omitempty"`
	Packages       string `json:"packages,omitempty"`
	Context        int64  `json:"context,omitempty"`
}
type SetupCodeFact struct {
	Title         string         `json:"title"`
	License       string         `json:"license"`
	Source        string         `json:"-"`
	State         string         `json:"state"`
	Untrusted     bool           `json:"untrusted"`
	DownloadBytes int64          `json:"download_bytes"`
	PeakBytes     int64          `json:"peak_bytes"`
	Ref           SetupModelFact `json:"-"`
}
type SetupPictureFact struct {
	Title            string         `json:"title"`
	Configured       bool           `json:"configured"`
	Elevation        bool           `json:"elevation"`
	Fallback         string         `json:"fallback"`
	DownloadBytes    int64          `json:"-"`
	PeakBytes        int64          `json:"-"`
	PackageBytes     int64          `json:"-"`
	PackagePeakBytes int64          `json:"-"`
	Licenses         []string       `json:"-"`
	PackageLicenses  []string       `json:"-"`
	Permissions      []string       `json:"-"`
	Ref              SetupModelFact `json:"-"`
}

// SetupVideoFact is a reviewed video model set as the question states it:
// its short title, the use policy that travels with its weights and the
// https link to its text, the Hugging Face repositories and revisions its files come from, what it
// downloads, the disk space it needs, the memory it needs while it makes a
// video, and its presets with their measured times.
type SetupVideoFact struct {
	Model         string             `json:"model"`
	Title         string             `json:"title"`
	License       string             `json:"license"`
	LicenseURL    string             `json:"license_url,omitempty"`
	Sources       []SetupVideoSource `json:"sources"`
	DownloadBytes int64              `json:"download_bytes"`
	PeakBytes     int64              `json:"peak_bytes"`
	MemoryBytes   int64              `json:"memory_bytes"`
	Presets       []SetupVideoPreset `json:"presets"`
}

// SetupVideoSource is one Hugging Face repository at one revision.
type SetupVideoSource struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
}

// SetupVideoPreset is one preset: quick or quality, its size, its stages
// (two for a generate-upscale-refine preset) and its measured seconds.
type SetupVideoPreset struct {
	Name    string `json:"name"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Stages  int    `json:"stages"`
	Seconds int64  `json:"seconds"`
}

// setupVideoSlot carries SetupVideoFact. It is present only in a question
// about video, so every other question keeps its 29 slots unchanged.
const setupVideoSlot = "video"

var setupFixedSlots = []string{"version", "occurrence", "requester", "code", "code_source", "code_ref", "code_plan", "picture", "picture_bytes", "picture_ref", "picture_licenses", "picture_package_licenses", "picture_permissions"}

func setupTargetSlot(i int) string { return fmt.Sprintf("target_%02d", i+1) }
func validSetupFactSlot(v string) bool {
	return len(v) <= 800 && utf8.ValidString(v) && strings.IndexFunc(v, unicode.IsControl) < 0
}
func validSetupSlot(key, value string) bool {
	if strings.HasPrefix(key, "target_") || key == setupVideoSlot {
		return validSetupFactSlot(value)
	}
	return len(value) <= 800 && validQuestionSlot(value)
}
func setupFactText(v string, max int) bool {
	return utf8.ValidString(v) && utf8.RuneCountInString(v) <= max && strings.IndexFunc(v, unicode.IsControl) < 0
}
func setupFactJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil || !validSetupFactSlot(string(raw)) {
		return "", ErrBadSlot
	}
	return string(raw), nil
}
func decodeSetupFact(raw string, out any) error {
	if !validSetupFactSlot(raw) {
		return ErrBadSlot
	}
	if err := decodeRetainedJSON([]byte(raw), out); err != nil {
		return ErrBadSlot
	}
	// Typed canonical rows reject duplicates, unknown members and alternate
	// spellings. No prose chunks, base64, or executable interpolation exists.
	if !canonicalRetainedJSON([]byte(raw), out) {
		return ErrBadSlot
	}
	return nil
}

func validateSetupFacts(f SetupQuestionFacts) error {
	if f.Video != nil {
		// A video question is an app's request for video alone.
		if len(f.Targets) != 0 || f.Code != nil || f.Picture != nil || f.Requester == "" || f.RequestUse != "video" || !validSetupVideo(*f.Video) {
			return ErrBadSlot
		}
	} else if len(f.Targets) < 1 || len(f.Targets) > 16 || f.Code == nil && f.Picture == nil || f.RequestUse == "video" {
		return ErrBadSlot
	}
	if f.Occurrence != "" {
		if len(f.Occurrence) != 48 || strings.ToLower(f.Occurrence) != f.Occurrence {
			return ErrBadSlot
		}
		if _, err := hex.DecodeString(f.Occurrence); err != nil {
			return ErrBadSlot
		}
	}
	if !setupFactText(f.Requester, 128) || (f.Requester == "") != (f.RequestUse == "") || f.RequestUse != "" && f.RequestUse != "code" && f.RequestUse != "image" && f.RequestUse != "video" {
		return ErrBadSlot
	}
	seen := map[string]bool{}
	code, picture := 0, 0
	for _, t := range f.Targets {
		if t.Name == "" || t.Title == "" || !setupFactText(t.Name, 64) || !setupFactText(t.Title, 128) || !setupFactText(t.Program, 4096) || !setupFactText(t.Revision, 256) || !setupFactText(t.Rules, 64) || seen[t.Name] {
			return ErrBadSlot
		}
		seen[t.Name] = true
		switch t.Use {
		case "code":
			code++
		case "image":
			picture++
			if t.Chat {
				return ErrBadSlot
			}
		default:
			return ErrBadSlot
		}
	}
	if (code > 0) != (f.Code != nil) || (picture == 1) != (f.Picture != nil) || picture > 1 {
		return ErrBadSlot
	}
	ref := func(r SetupModelFact) bool {
		return setupFactText(r.Family, 128) && setupFactText(r.CatalogVersion, 64) && setupFactText(r.Digest, 71) && setupFactText(r.Plan, 128) && setupFactText(r.Packages, 128) && r.Context >= 0
	}
	if c := f.Code; c != nil {
		if c.Title == "" || !setupFactText(c.Title, 64) || !setupFactText(c.License, 200) || !setupFactText(c.Source, 200) || !ref(c.Ref) || c.DownloadBytes < 0 || c.PeakBytes < 0 {
			return ErrBadSlot
		}
		switch c.State {
		case "download", "local", "served", "hosted":
		default:
			return ErrBadSlot
		}
	}
	if p := f.Picture; p != nil {
		if p.Title == "" || !setupFactText(p.Title, 64) || !ref(p.Ref) || p.DownloadBytes < 0 || p.PeakBytes < 0 || p.PackageBytes < 0 || p.PackagePeakBytes < 0 {
			return ErrBadSlot
		}
		if p.Fallback != "" && p.Fallback != "no_package" && p.Fallback != "no_feature" {
			return ErrBadSlot
		}
		if len(p.Licenses) > 16 || len(p.PackageLicenses) > 16 || len(p.Permissions) > 3 {
			return ErrBadSlot
		}
		for _, license := range append(append([]string{}, p.Licenses...), p.PackageLicenses...) {
			if license == "" || !setupFactText(license, 64) {
				return ErrBadSlot
			}
		}
		permissions := map[string]bool{}
		for _, kind := range p.Permissions {
			if SetupPermissionWords(kind) == "" || permissions[kind] {
				return ErrBadSlot
			}
			permissions[kind] = true
		}
	}
	var download, peak int64
	add := func(a, b int64) bool {
		if a > math.MaxInt64-download || b > math.MaxInt64-peak {
			return false
		}
		download += a
		peak += b
		return true
	}
	if c := f.Code; c != nil && c.State == "download" {
		if !add(c.DownloadBytes, c.PeakBytes) {
			return ErrBadSlot
		}
	}
	if p := f.Picture; p != nil {
		if !p.Configured && !add(p.DownloadBytes, p.PeakBytes) {
			return ErrBadSlot
		}
		if !add(p.PackageBytes, p.PackagePeakBytes) {
			return ErrBadSlot
		}
	}
	if download > math.MaxInt64-999999999 || peak > math.MaxInt64-999999999 {
		return ErrBadSlot
	}
	return nil
}

// setupVideoMaxBytes bounds every size a video fact states (1 PiB).
const setupVideoMaxBytes = 1 << 50

func validSetupVideo(v SetupVideoFact) bool {
	if v.Model == "" || v.Title == "" || v.License == "" || !setupFactText(v.Model, 64) || !setupFactText(v.Title, 64) || !setupFactText(v.License, 128) ||
		len(v.Sources) < 1 || len(v.Sources) > 4 || len(v.Presets) > 2 ||
		v.DownloadBytes < 0 || v.PeakBytes < v.DownloadBytes || v.MemoryBytes < 0 ||
		v.PeakBytes > setupVideoMaxBytes || v.MemoryBytes > setupVideoMaxBytes ||
		v.LicenseURL != "" && (!strings.HasPrefix(v.LicenseURL, "https://") || !setupFactText(v.LicenseURL, 200) || strings.ContainsAny(v.LicenseURL, " \"<>")) {
		return false
	}
	for _, source := range v.Sources {
		if source.Repository == "" || !setupFactText(source.Repository, 96) || len(source.Revision) != 40 || strings.ToLower(source.Revision) != source.Revision {
			return false
		}
		if _, err := hex.DecodeString(source.Revision); err != nil {
			return false
		}
	}
	names := map[string]bool{}
	for _, p := range v.Presets {
		if p.Name != "quick" && p.Name != "quality" || names[p.Name] || p.Width < 1 || p.Width > 8192 || p.Height < 1 || p.Height > 8192 ||
			p.Stages < 1 || p.Stages > 2 || p.Seconds < 0 || p.Seconds > 86400 {
			return false
		}
		names[p.Name] = true
	}
	return true
}

// SetupFactSlots projects the frozen native plan into the existing immutable
// Asks carrier. Occurrence is initially empty; the admitting runtime fills it.
func SetupFactSlots(f SetupQuestionFacts) (map[string]string, error) {
	if err := validateSetupFacts(f); err != nil {
		return nil, err
	}
	slots := map[string]string{}
	for _, key := range setupFixedSlots {
		slots[key] = ""
	}
	for i := 0; i < 16; i++ {
		slots[setupTargetSlot(i)] = ""
	}
	slots["version"], slots["occurrence"] = "1", f.Occurrence
	put := func(key string, value any) error {
		raw, err := setupFactJSON(value)
		if err == nil && !validSetupSlot(key, raw) {
			err = ErrBadSlot
		}
		slots[key] = raw
		return err
	}
	if err := put("requester", struct {
		App string `json:"app"`
		Use string `json:"use"`
	}{f.Requester, f.RequestUse}); err != nil {
		return nil, err
	}
	for i, target := range f.Targets {
		if err := put(setupTargetSlot(i), target); err != nil {
			return nil, err
		}
	}
	if f.Code != nil {
		if err := put("code", f.Code); err != nil {
			return nil, err
		}
		ref := f.Code.Ref
		ref.Plan = ""
		if err := put("code_ref", ref); err != nil {
			return nil, err
		}
		slots["code_source"], slots["code_plan"] = f.Code.Source, f.Code.Ref.Plan
	}
	if f.Video != nil {
		if err := put(setupVideoSlot, f.Video); err != nil {
			return nil, err
		}
	}
	if p := f.Picture; p != nil {
		for key, value := range map[string]any{"picture": p, "picture_bytes": [4]int64{p.DownloadBytes, p.PeakBytes, p.PackageBytes, p.PackagePeakBytes}, "picture_ref": p.Ref, "picture_licenses": p.Licenses, "picture_package_licenses": p.PackageLicenses, "picture_permissions": p.Permissions} {
			if err := put(key, value); err != nil {
				return nil, err
			}
		}
	}
	for key, value := range slots {
		if !validSetupSlot(key, value) {
			return nil, ErrBadSlot
		}
	}
	return slots, nil
}

func SetupFactsFromSlots(slots map[string]string) (SetupQuestionFacts, error) {
	var f SetupQuestionFacts
	video, hasVideo := slots[setupVideoSlot]
	if hasVideo && (video == "" || len(slots) != 30) || !hasVideo && len(slots) != 29 || slots["version"] != "1" {
		return f, ErrBadSlot
	}
	if hasVideo {
		f.Video = &SetupVideoFact{}
		if decodeSetupFact(video, f.Video) != nil {
			return f, ErrBadSlot
		}
	}
	for _, key := range setupFixedSlots {
		if value, ok := slots[key]; !ok || !validSetupSlot(key, value) {
			return f, ErrBadSlot
		}
	}
	for i := 0; i < 16; i++ {
		key := setupTargetSlot(i)
		if value, ok := slots[key]; !ok || !validSetupSlot(key, value) {
			return f, ErrBadSlot
		}
	}
	f.Occurrence = slots["occurrence"]
	var requester struct {
		App string `json:"app"`
		Use string `json:"use"`
	}
	if err := decodeSetupFact(slots["requester"], &requester); err != nil {
		return f, err
	}
	f.Requester, f.RequestUse = requester.App, requester.Use
	empty := false
	for i := 0; i < 16; i++ {
		value := slots[setupTargetSlot(i)]
		if value == "" {
			empty = true
			continue
		}
		if empty {
			return f, ErrBadSlot
		}
		var target SetupTargetFact
		if err := decodeSetupFact(value, &target); err != nil {
			return f, err
		}
		f.Targets = append(f.Targets, target)
	}
	if slots["code"] != "" {
		f.Code = &SetupCodeFact{}
		if decodeSetupFact(slots["code"], f.Code) != nil || decodeSetupFact(slots["code_ref"], &f.Code.Ref) != nil || f.Code.Ref.Plan != "" {
			return f, ErrBadSlot
		}
		f.Code.Source, f.Code.Ref.Plan = slots["code_source"], slots["code_plan"]
	} else {
		for _, key := range []string{"code_source", "code_ref", "code_plan"} {
			if slots[key] != "" {
				return f, ErrBadSlot
			}
		}
	}
	if slots["picture"] != "" {
		f.Picture = &SetupPictureFact{}
		var sizes [4]int64
		for key, out := range map[string]any{"picture": f.Picture, "picture_bytes": &sizes, "picture_ref": &f.Picture.Ref, "picture_licenses": &f.Picture.Licenses, "picture_package_licenses": &f.Picture.PackageLicenses, "picture_permissions": &f.Picture.Permissions} {
			if decodeSetupFact(slots[key], out) != nil {
				return f, ErrBadSlot
			}
		}
		f.Picture.DownloadBytes, f.Picture.PeakBytes, f.Picture.PackageBytes, f.Picture.PackagePeakBytes = sizes[0], sizes[1], sizes[2], sizes[3]
	} else {
		for _, key := range []string{"picture_bytes", "picture_ref", "picture_licenses", "picture_package_licenses", "picture_permissions"} {
			if slots[key] != "" {
				return f, ErrBadSlot
			}
		}
	}
	if err := validateSetupFacts(f); err != nil {
		return f, err
	}
	return f, nil
}

func SetupPermissionWords(kind string) string {
	switch kind {
	case "make_pictures":
		return "Make pictures on this computer."
	case "write_pictures":
		return "Save the pictures it makes."
	case "read_pictures":
		return "Read back the pictures it made."
	}
	return ""
}

const SetupFallbackNoPackage = "The faster picture engine for this graphics card has no reviewed package in this release yet, so pictures use the standard engine."
const SetupFallbackNoFeature = "This installation cannot add the faster picture engine for this graphics card, so pictures use the standard engine."

func SetupFallbackWords(kind string) string {
	switch kind {
	case "no_package":
		return SetupFallbackNoPackage
	case "no_feature":
		return SetupFallbackNoFeature
	}
	return ""
}
func SetupFallbackFact(words string) (string, error) {
	switch words {
	case "":
		return "", nil
	case SetupFallbackNoPackage:
		return "no_package", nil
	case SetupFallbackNoFeature:
		return "no_feature", nil
	}
	return "", errors.New("asks: unknown Setup fallback")
}

// Slot declarations are per catalogue key. Other questions keep their existing
// 200-rune limit; native Setup rows use the carrier's existing 800-byte bound.
func setupDeclaredSlots() []string {
	keys := append([]string{}, setupFixedSlots...)
	for i := 0; i < 16; i++ {
		keys = append(keys, setupTargetSlot(i))
	}
	return keys
}
