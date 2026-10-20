package catalogue

import (
	"fmt"
	"slices"
	"strings"
)

type SetupPresentation struct{ Title, Instruction, Details string }
type setupRenderApp struct{ Title string }
type setupRenderPick struct {
	Title, License            string
	Served, Hosted, Untrusted bool
	Local                     *bool
}
type setupRenderPlan struct {
	Source                   string
	Licenses                 []string
	DownloadBytes, PeakBytes int64
}
type setupRenderChat struct{ Permission string }
type setupRenderOffer struct {
	Title                          string
	Permissions, PackageLicenses   []string
	Configured, Elevation          bool
	PackageBytes, PackagePeakBytes int64
	Fallback                       string
}
type setupRenderPictures struct {
	app   setupRenderApp
	plan  setupRenderPlan
	offer setupRenderOffer
}

// SetupRequestSentence names the requesting app in the question about setting
// up a coding model or, for image use, pictures (architect ruling 2026-11-10,
// row 13). For video use it says the app asks to set up video with sound on
// this computer (architect ruling 2026-11-11, Setup use video).
func SetupRequestSentence(app, use string) string {
	if use == "video" {
		return app + " asks to set up video with sound on this computer."
	}
	what := "a coding model"
	if use == "image" {
		what = "pictures"
	}
	return fmt.Sprintf("Set up %s for %s?", what, app)
}

// SetupCannotRunSentence words a Fit answer in which nothing fits this
// computer: the pool free for models and the smallest model's title and
// need, or the pool alone when no model in the release has a size.
func SetupCannotRunSentence(pool int64, smallest string, need int64) string {
	if smallest == "" {
		return fmt.Sprintf("This computer has %s free for models, and no model in this release fits it.", SetupGigabytes(pool))
	}
	return fmt.Sprintf("This computer has %s free for models; %s needs %s.", SetupGigabytes(pool), smallest, SetupGigabytes(need))
}

// SetupQuestionPresentation is the setup.models catalogue's complete native
// presentation. Both admission and every presenter render the retained facts
// here; the caller supplies no title, instruction or disclosure prose.
func SetupQuestionPresentation(slots map[string]string) (SetupPresentation, error) {
	facts, err := SetupFactsFromSlots(slots)
	if err != nil {
		return SetupPresentation{}, err
	}
	return SetupPresentationForFacts(facts)
}

func SetupPresentationForFacts(facts SetupQuestionFacts) (SetupPresentation, error) {
	if err := validateSetupFacts(facts); err != nil {
		return SetupPresentation{}, err
	}
	if facts.Video != nil {
		return setupVideoPresentation(facts.Requester, *facts.Video), nil
	}
	var code []setupRenderApp
	var chats []setupRenderChat
	var pictureApp setupRenderApp
	for _, target := range facts.Targets {
		app := setupRenderApp{Title: target.Title}
		if target.Use == "image" {
			pictureApp = app
			continue
		}
		code = append(code, app)
		if target.Chat {
			chats = append(chats, setupRenderChat{Permission: fmt.Sprintf("%s can use %s on this computer.", target.Title, facts.Code.Title)})
		}
	}
	var pick *setupRenderPick
	var plan setupRenderPlan
	var pictures *setupRenderPictures
	if c := facts.Code; c != nil {
		pick = &setupRenderPick{Title: c.Title, License: c.License, Served: c.State == "served", Hosted: c.State == "hosted", Untrusted: c.Untrusted}
		if c.State == "local" {
			local := true
			pick.Local = &local
		}
		plan = setupRenderPlan{Source: c.Source, DownloadBytes: c.DownloadBytes, PeakBytes: c.PeakBytes}
	}
	if p := facts.Picture; p != nil {
		var permissions []string
		for _, kind := range p.Permissions {
			permissions = append(permissions, SetupPermissionWords(kind))
		}
		pictures = &setupRenderPictures{app: pictureApp, plan: setupRenderPlan{Licenses: p.Licenses, DownloadBytes: p.DownloadBytes, PeakBytes: p.PeakBytes}, offer: setupRenderOffer{Title: p.Title, Permissions: permissions, PackageLicenses: p.PackageLicenses, Configured: p.Configured, Elevation: p.Elevation, PackageBytes: p.PackageBytes, PackagePeakBytes: p.PackagePeakBytes, Fallback: SetupFallbackWords(p.Fallback)}}
	}
	q := renderSetupQuestion(code, pick, plan, pictures, chats)
	if facts.Requester != "" {
		q.Title = SetupRequestSentence(facts.Requester, facts.RequestUse)
	}
	return q, nil
}

// renderSetupQuestion words the question (architect ruling 2026-11-10, row
// 13): Title is one plain question naming the apps, Instruction is the
// download size in one sentence, and Details holds every other fact.
func renderSetupQuestion(code []setupRenderApp, pick *setupRenderPick, plan setupRenderPlan, pictures *setupRenderPictures, chat []setupRenderChat) SetupPresentation {
	titles := make([]string, 0, len(code)+1)
	for _, app := range code {
		titles = append(titles, app.Title)
	}
	slices.Sort(titles)
	who := setupJoinWords(titles, "and")
	gets := "gets"
	if len(code) > 1 {
		gets = "get"
	}
	if len(chat) > 0 && len(chat) == len(code) {
		gets = "can use"
	}
	var download, peak int64
	var lines, details []string
	if pick != nil {
		switch {
		case pick.Served || pick.Hosted:
			lines = append(lines, fmt.Sprintf("%s %s %s, which this computer already runs.", who, gets, pick.Title))
			details = append(details, setupServedDetails(who, len(code) > 1, *pick, chat))
		case pick.Local != nil:
			lines = append(lines, fmt.Sprintf("%s %s %s, which is already on this computer.", who, gets, pick.Title))
			details = append(details, setupChatDetails(who, len(code) > 1, *pick, plan, chat))
		default:
			lines = append(lines, fmt.Sprintf("%s %s %s on this computer.", who, gets, pick.Title))
			details = append(details, setupChatDetails(who, len(code) > 1, *pick, plan, chat))
			download, peak = plan.DownloadBytes, plan.PeakBytes
		}
	}
	elevation := false
	if pictures != nil {
		titles = append(titles, pictures.app.Title)
		lines = append(lines, pictures.app.Title+" gets pictures on this computer.")
		if !pictures.offer.Configured {
			download, peak = download+pictures.plan.DownloadBytes, peak+pictures.plan.PeakBytes
		}
		download, peak = download+pictures.offer.PackageBytes, peak+pictures.offer.PackagePeakBytes
		details = append(details, setupPictureDetails(pictures))
		elevation = pictures.offer.Elevation
	}
	if peak > 0 {
		lines = append(lines, SetupGigabytes(peak)+" free needed.")
	}
	q := SetupPresentation{Title: "Set up " + setupJoinWords(titles, "and") + "?"}
	q.Instruction = "Nothing to download"
	if download > 0 {
		q.Instruction = fmt.Sprintf("About %s to download", SetupGigabytes(download))
	}
	if elevation {
		q.Instruction += "; Windows asks for approval once"
	}
	q.Instruction += "."
	// One paragraph: the native details box shows a bare line feed as no
	// break.
	q.Details = strings.Join(append(lines, details...), " ")
	return q
}

func setupChatDetails(who string, many bool, pick setupRenderPick, plan setupRenderPlan, chat []setupRenderChat) string {
	if pick.Local != nil {
		return fmt.Sprintf("Licence: %s. Source: %s. OpenAbstractions uses the copy already on this computer, which has the reviewed file's size and SHA-256, where it lies. It runs the model with its local engine and makes it the default chat model for apps on this account. %s", pick.License, plan.Source, setupChatUse(who, many, chat))
	}
	text := fmt.Sprintf("Licence: %s. Source: %s. OpenAbstractions downloads the model, runs it with its local engine and makes it the default chat model for apps on this account. %s", pick.License, plan.Source, setupChatUse(who, many, chat))
	if pick.Untrusted {
		text += " Other accounts can change the copy here. OpenAbstractions downloads its own."
	}
	return text
}

// setupServedDetails is Show details for a model this computer already runs.
func setupServedDetails(who string, many bool, pick setupRenderPick, chat []setupRenderChat) string {
	text := fmt.Sprintf("Choosing Set up makes %s the default chat model for apps on this account.", pick.Title)
	if len(chat) > 0 {
		text += " " + setupChatUse(who, many, chat)
	}
	return text
}

// setupChatUse states the chat permission the answer grants: each offer's own
// sentence and that nothing beyond this model is included, or, without one,
// that the apps ask at their first chat.
func setupChatUse(who string, many bool, chat []setupRenderChat) string {
	if len(chat) == 0 {
		if many {
			return who + " ask before their first chat."
		}
		return who + " asks before its first chat."
	}
	sentences := make([]string, 0, len(chat))
	for _, offer := range chat {
		sentences = append(sentences, offer.Permission)
	}
	return strings.Join(sentences, " ") + " No other model is included."
}

// setupPictureDetails names the picture model, its licences and the engine
// package's, and each permission the picture app gets.
func setupPictureDetails(work *setupRenderPictures) string {
	licences := slices.Clone(work.plan.Licenses)
	for _, l := range work.offer.PackageLicenses {
		if !slices.Contains(licences, l) {
			licences = append(licences, l)
		}
	}
	text := fmt.Sprintf("Pictures: %s, made on this computer. Licences: %s.", work.offer.Title, strings.Join(licences, ", "))
	if len(work.offer.Permissions) > 0 {
		text += fmt.Sprintf(" %s gets these permissions: %s", work.app.Title, strings.Join(work.offer.Permissions, " "))
	}
	if work.offer.Fallback != "" {
		text += " " + work.offer.Fallback
	}
	return text
}

// setupJoinWords joins words as a sentence lists them, the last after conj
// ("and" or "or").
func setupJoinWords(words []string, conj string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return fmt.Sprintf("%s %s %s", strings.Join(words[:len(words)-1], ", "), conj, words[len(words)-1])
}

// SetupGigabytes is a byte count as Setup's questions and notices show it,
// in whole gigabytes rounded up. OpenAbstractions counts every size as
// Windows Explorer and Task Manager do, 1024³ bytes to a GB; the Panel's
// oaBytes is the same rule (architect ruling 2026-11-09).
func SetupGigabytes(n int64) string {
	if n <= 0 {
		return "0 GB"
	}
	const gb = 1 << 30
	return fmt.Sprintf("%d GB", (n+gb-1)/gb)
}

// setupVideoPresentation words an app's request for video (architect
// ruling 2026-11-11, Setup use video): the heading names the app, the body
// gives the download and the free space it needs, and Show details holds the
// licence, the sources, the memory need and the presets' times.
func setupVideoPresentation(app string, v SetupVideoFact) SetupPresentation {
	q := SetupPresentation{Title: SetupRequestSentence(app, "video")}
	q.Instruction = fmt.Sprintf("%s: about %s to download; %s free needed.", v.Title, SetupGigabytes(v.DownloadBytes), SetupGigabytes(v.PeakBytes))
	sources := make([]string, 0, len(v.Sources))
	for _, source := range v.Sources {
		sources = append(sources, fmt.Sprintf("%s at revision %s", source.Repository, source.Revision[:7]))
	}
	licence := fmt.Sprintf("Licence: %s.", v.License)
	if v.LicenseURL != "" {
		licence = fmt.Sprintf("Licence: %s (%s).", v.License, v.LicenseURL)
	}
	details := []string{
		licence,
		fmt.Sprintf("Sources on Hugging Face: %s.", setupJoinWords(sources, "and")),
		fmt.Sprintf("Making a video needs about %s of memory.", SetupGigabytes(v.MemoryBytes)),
	}
	for _, p := range v.Presets {
		name := "Quick"
		if p.Name == "quality" {
			name = "Quality"
		}
		stages := ""
		if p.Stages == 2 {
			stages = " in two stages"
		}
		details = append(details, fmt.Sprintf("%s: %dx%d%s, about %s.", name, p.Width, p.Height, stages, setupMinutes(p.Seconds)))
	}
	q.Details = strings.Join(details, " ")
	return q
}

// setupMinutes is a measured time in whole minutes, rounded to the nearest
// and at least one.
func setupMinutes(seconds int64) string {
	return fmt.Sprintf("%d min", max(1, (seconds+30)/60))
}
