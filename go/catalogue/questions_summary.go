package catalogue

import (
	"net/url"
	"strings"
	"unicode"
)

type questionSummary struct{ template, fallback string }

// These are catalogue display facts, separate from full prompts and authority.
// Every key has an explicit entry; slots are deliberately limited below.
var questionSummaries = map[string]questionSummary{
	ProgramReviewKey:          {"Asked to review updated programs", "Asked a question"},
	SetupModelsKey:            {"Asked to set up models", "Asked a question"},
	ApplicationCommandOnceKey: {"Asked a question", "Asked a question"},
	AgentEnrollmentKey:        {"Asked a question", "Asked a question"},
	AgentUpdateKey:            {"Asked a question", "Asked a question"},
	PairingKey:                {"Asked to pair computers", "Asked a question"},
	"content.replace":         {"Asked to replace stored content once", "Asked a question"},
	"rights.register":         {"Asked for permissions", "Asked a question"},
	projectEditQuestionKey:    {"Asked to edit files in {folder.leaf}", "Asked to edit files in a folder"},
	"download.reach":          {"Asked to fetch from {host}", "Asked to fetch from the internet"},
	FirstUseKey:               {"Asked a question", "Asked a question"},
	SelfRegisteredKey:         {"Added to OpenAbstractions", "Asked a question"},
	SystemSecurityLogKey:      {"Asked to read the Windows Security log", "Asked a question"},
	RightsChangeKey:           {"Asked to change permissions", "Asked a question"},
	TypedPermissionKey:        {"Asked for permissions", "Asked a question"},
}

// Summary returns the catalogue's short plain-text display sentence. Only the
// folder's leaf or a bare host may enter it; unknown or unsafe values use the
// catalogue's generic fallback. It grants nothing and is separate from Render's
// full prompt and the trusted richer permission/registration projections.
func (q Question) Summary(about string) string {
	spec, ok := questionSummaries[q.Key]
	if !ok {
		return "Asked a question"
	}
	if !strings.ContainsAny(spec.template, "{}") {
		return spec.template
	}
	if !validQuestionSlot(about) || strings.IndexFunc(about, func(r rune) bool {
		return unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
	}) >= 0 {
		return spec.fallback
	}
	switch {
	case strings.Contains(spec.template, "{folder.leaf}"):
		if strings.Contains(about, "://") {
			return spec.fallback
		}
		// Display a recorded path from either convention without resolving it.
		leaf := strings.TrimRight(about, `/\`)
		if i := strings.LastIndexAny(leaf, `/\`); i >= 0 {
			leaf = leaf[i+1:]
		}
		if leaf == "" || leaf == "." || leaf == ".." || strings.ContainsAny(leaf, `:?#@{}`) || strings.TrimSpace(leaf) == "" {
			return spec.fallback
		}
		return strings.ReplaceAll(spec.template, "{folder.leaf}", leaf)
	case strings.Contains(spec.template, "{host}"):
		if about == "" || strings.ContainsAny(about, `/\@?#{}`) || strings.Trim(about, ".") == "" {
			return spec.fallback
		}
		if strings.IndexFunc(about, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && !strings.ContainsRune(".-:[]", r)
		}) >= 0 {
			return spec.fallback
		}
		u, err := url.Parse("//" + about)
		if err != nil || u.Host != about || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return spec.fallback
		}
		return strings.ReplaceAll(spec.template, "{host}", about)
	default:
		return spec.fallback
	}
}
