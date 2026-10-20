package catalogue

import (
	"strings"
	"testing"
)

// Setup's step notice and applications@3 RequestSetup's cannot-run reason
// both read this sentence; the words and the rounding live here once.
func TestSetupCannotRunSentence(t *testing.T) {
	cases := []struct {
		pool     int64
		smallest string
		need     int64
		want     string
	}{
		{4 << 30, "Qwen3.6 35B", 24<<30 + 1, "This computer has 4 GB free for models; Qwen3.6 35B needs 25 GB."},
		{4_000_000_000, "Qwen3.6 35B", 25_000_000_000, "This computer has 4 GB free for models; Qwen3.6 35B needs 24 GB."},
		{4 << 30, "", 0, "This computer has 4 GB free for models, and no model in this release fits it."},
		{-1, "", 0, "This computer has 0 GB free for models, and no model in this release fits it."},
	}
	for _, c := range cases {
		if got := SetupCannotRunSentence(c.pool, c.smallest, c.need); got != c.want {
			t.Errorf("SetupCannotRunSentence(%d, %q, %d) = %q, want %q", c.pool, c.smallest, c.need, got, c.want)
		}
	}
}

// A gigabyte is 1024³ bytes, as Windows Explorer counts it (architect
// ruling 2026-11-09).
func TestSetupGigabytesRoundsUp(t *testing.T) {
	for n, want := range map[int64]string{0: "0 GB", -5: "0 GB", 1: "1 GB", 1 << 30: "1 GB", 1<<30 + 1: "2 GB", 1_000_000_000: "1 GB", 25_000_000_000: "24 GB"} {
		if got := SetupGigabytes(n); got != want {
			t.Errorf("SetupGigabytes(%d) = %q, want %q", n, got, want)
		}
	}
}

// The Setup question's heading names the apps, its body is the download in
// one sentence, and Show details holds every other fact (architect ruling
// 2026-11-10, row 13).
func TestSetupQuestionHeadingBodyAndDetails(t *testing.T) {
	const gb = int64(1) << 30
	code := func() *SetupCodeFact {
		return &SetupCodeFact{Title: "Qwen3-Coder", License: "Apache-2.0", Source: "https://example.test/qwen", State: "download", DownloadBytes: 5 * gb, PeakBytes: 6 * gb}
	}
	picture := func(elevation bool) *SetupPictureFact {
		return &SetupPictureFact{Title: "FLUX.2 Klein 4B", Elevation: elevation, DownloadBytes: 3 * gb, PeakBytes: 4 * gb, PackageBytes: gb, PackagePeakBytes: gb, Licenses: []string{"Apache-2.0"}, Permissions: []string{"make_pictures"}}
	}
	opencode := SetupTargetFact{Name: "opencode", Title: "OpenCode", Use: "code"}
	comfy := SetupTargetFact{Name: "comfyui", Title: "ComfyUI", Use: "image"}
	render := func(t *testing.T, f SetupQuestionFacts) SetupPresentation {
		t.Helper()
		q, err := SetupPresentationForFacts(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(q.Details, "\n") {
			t.Fatalf("details break lines: %q", q.Details)
		}
		return q
	}

	t.Run("two apps with elevation", func(t *testing.T) {
		q := render(t, SetupQuestionFacts{Targets: []SetupTargetFact{opencode, comfy}, Code: code(), Picture: picture(true)})
		if q.Title != "Set up OpenCode and ComfyUI?" || q.Instruction != "About 9 GB to download; Windows asks for approval once." {
			t.Fatalf("question = %+v", q)
		}
		if !strings.HasPrefix(q.Details, "OpenCode gets Qwen3-Coder on this computer. ComfyUI gets pictures on this computer. 11 GB free needed. Licence: Apache-2.0.") {
			t.Fatalf("details = %q", q.Details)
		}
		if strings.Contains(q.Details, "administrator") {
			t.Fatalf("details repeat the approval: %q", q.Details)
		}
	})
	t.Run("pictures without elevation", func(t *testing.T) {
		q := render(t, SetupQuestionFacts{Targets: []SetupTargetFact{comfy}, Picture: picture(false)})
		if q.Title != "Set up ComfyUI?" || q.Instruction != "About 4 GB to download." {
			t.Fatalf("question = %+v", q)
		}
	})
	t.Run("code only", func(t *testing.T) {
		q := render(t, SetupQuestionFacts{Targets: []SetupTargetFact{opencode}, Code: code()})
		if q.Title != "Set up OpenCode?" || q.Instruction != "About 5 GB to download." {
			t.Fatalf("question = %+v", q)
		}
	})
	t.Run("nothing to download", func(t *testing.T) {
		served := code()
		served.State = "served"
		q := render(t, SetupQuestionFacts{Targets: []SetupTargetFact{opencode}, Code: served})
		if q.Instruction != "Nothing to download." || !strings.HasPrefix(q.Details, "OpenCode gets Qwen3-Coder, which this computer already runs. ") {
			t.Fatalf("question = %+v", q)
		}
		configured := picture(true)
		configured.Configured, configured.PackageBytes, configured.PackagePeakBytes = true, 0, 0
		q = render(t, SetupQuestionFacts{Targets: []SetupTargetFact{opencode, comfy}, Code: served, Picture: configured})
		if q.Instruction != "Nothing to download; Windows asks for approval once." {
			t.Fatalf("question = %+v", q)
		}
	})
	t.Run("app request", func(t *testing.T) {
		q := render(t, SetupQuestionFacts{Requester: "OpenCode", RequestUse: "code", Targets: []SetupTargetFact{opencode}, Code: code()})
		if q.Title != "Set up a coding model for OpenCode?" || q.Instruction != "About 5 GB to download." {
			t.Fatalf("question = %+v", q)
		}
		q = render(t, SetupQuestionFacts{Requester: "ComfyUI", RequestUse: "image", Targets: []SetupTargetFact{comfy}, Picture: picture(true)})
		if q.Title != "Set up pictures for ComfyUI?" || q.Instruction != "About 4 GB to download; Windows asks for approval once." {
			t.Fatalf("question = %+v", q)
		}
	})
}

// A video fact's licence link must be one https URL without spaces or quotes.
func TestSetupVideoLicenseURLIsChecked(t *testing.T) {
	base := SetupVideoFact{Model: "ltx-2.5", Title: "LTX-2.5", License: "LTX-2 Community License Agreement",
		Sources: []SetupVideoSource{{Repository: "Lightricks/LTX-2.5", Revision: "5e6e71018ee1756ed329b697a7b4aedc934dfce9"}}}
	for url, ok := range map[string]bool{
		"": true, "https://huggingface.co/Lightricks/LTX-2.5/blob/main/README.md": true,
		"http://example.com/licence": false, "https://a b": false, `https://a"b`: false, "javascript:alert(1)": false,
	} {
		v := base
		v.LicenseURL = url
		if got := validSetupVideo(v); got != ok {
			t.Errorf("validSetupVideo(license_url %q) = %v, want %v", url, got, ok)
		}
	}
}
