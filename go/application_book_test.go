package asks

import (
	"bytes"
	"errors"
	"github.com/openabstractions/abstraction-identity/listen"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationReplayScopeAndTombstone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.json")
	b, e := LoadApplicationBook(path)
	if e != nil {
		t.Fatal(e)
	}
	ask := func(scope, key, host string) (Record, string) {
		t.Helper()
		r, o, e := b.AskApplication(scope, "same-name", key, "download.reach", map[string]string{"host": host}, listen.Seen{Bound: true, Path: scope})
		if e != nil {
			t.Fatal(e)
		}
		return r, o
	}
	first, o := ask("program-a", "key", "example.com")
	if o != "pending" {
		t.Fatal(o)
	}
	if _, e = b.Answer(first.ID, "once"); e != nil {
		t.Fatal(e)
	}
	b, e = LoadApplicationBook(path)
	if e != nil {
		t.Fatal(e)
	}
	replay, o := ask("program-a", "key", "example.com")
	if o != "answered" || replay.ID != first.ID || replay.Option != "once" {
		t.Fatalf("replay %+v %s", replay, o)
	}
	if _, o = ask("program-a", "key", "other.com"); o != "conflict" {
		t.Fatal(o)
	}
	other, o := ask("program-b", "key", "example.com")
	if o != "pending" || other.ID == first.ID {
		t.Fatalf("cross-scope %+v %s", other, o)
	}
	if e = b.Forget(first.ID); e != nil {
		t.Fatal(e)
	}
	b, e = LoadApplicationBook(path)
	if e != nil {
		t.Fatal(e)
	}
	if _, o = ask("program-a", "key", "example.com"); o != "gone" {
		t.Fatal(o)
	}
	if _, e = LoadBook(path); e == nil {
		t.Fatal("legacy opened application file")
	}
	if _, _, e = b.Ask(reach, listen.Seen{}); e == nil {
		t.Fatal("legacy admission into application profile")
	}
}
func TestApplicationQuestionContentAndCapacity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.json")
	b, e := LoadApplicationBook(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, slots := range []map[string]string{{"rights": "first"}, {"rights": "second"}} {
		r, o, e := b.AskApplication("scope", "app", slots["rights"], "rights.register", slots, listen.Seen{})
		if e != nil || o != "pending" {
			t.Fatal(o, e)
		}
		if _, e = b.Answer(r.ID, "never"); e != nil {
			t.Fatal(e)
		}
	}
	if len(b.List(false)) != 2 {
		t.Fatal("different full content reused kept answer")
	}
	for _, slots := range []map[string]string{{"host": "ok", "asker": "victim"}, {"host": "bad\n"}, {"host": strings.Repeat("a", 201)}, {}} {
		_, o, e := b.AskApplication("scope", "app", "bad", "download.reach", slots, listen.Seen{})
		if e != nil || o != "invalid" {
			t.Fatal(o, e)
		}
	}
	r, o, e := b.AskApplication("scope", "app", "base", "download.reach", map[string]string{"host": "ok"}, listen.Seen{})
	if e != nil || o != "pending" {
		t.Fatal(o, e)
	}
	if e = b.changeApplication(func(f *book) error {
		base := f.Admissions[len(f.Admissions)-1]
		for len(f.Admissions) < MaxApplicationAdmissions {
			a := base
			a.Key = string(rune(1000 + len(f.Admissions)))
			f.Admissions = append(f.Admissions, a)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(path)
	if _, _, e = b.AskApplication("scope", "app", "overflow", "download.reach", map[string]string{"host": "ok"}, listen.Seen{}); !errors.Is(e, ErrApplicationCapacity) {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("capacity refusal wrote")
	}
	got, o, e := b.ObserveApplication("scope", "base")
	if e != nil || o != "pending" || got.ID != r.ID {
		t.Fatal(got, o, e)
	}
}

func TestContentReplaceAttemptIsIdempotentButOnceDoesNotCarryToNextAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	slots := map[string]string{
		"target": "model.safetensors", "replacement": "model-klein.safetensors", "code": "R7KD", "until": "2026-09-29 14:30 UTC",
	}
	missing := map[string]string{"target": slots["target"], "replacement": slots["replacement"], "until": slots["until"]}
	if _, outcome, err := b.AskApplication("attempt-scope", "ComfyUI", "missing-slot", "content.replace", missing, listen.Seen{}); err != nil || outcome != "invalid" {
		t.Fatalf("missing declared slot must be refused: outcome=%q err=%v", outcome, err)
	}
	extra := map[string]string{"target": slots["target"], "replacement": slots["replacement"], "code": slots["code"], "until": slots["until"], "custom": "text"}
	if _, outcome, err := b.AskApplication("attempt-scope", "ComfyUI", "extra-slot", "content.replace", extra, listen.Seen{}); err != nil || outcome != "invalid" {
		t.Fatalf("arbitrary extra slot must be refused: outcome=%q err=%v", outcome, err)
	}
	first, outcome, err := b.AskApplication("attempt-scope", "ComfyUI", "replace-1", "content.replace", slots, listen.Seen{})
	if err != nil || outcome != "pending" {
		t.Fatalf("first ask: outcome=%q err=%v", outcome, err)
	}
	if first.About != slots["target"] || first.Text != "ComfyUI requests a one-time replacement of model.safetensors with model-klein.safetensors. Request R7KD expires 2026-09-29 14:30 UTC." {
		t.Fatalf("record does not use the service-owned target and wording: %+v", first)
	}
	replay, outcome, err := b.AskApplication("attempt-scope", "ComfyUI", "replace-1", "content.replace", slots, listen.Seen{})
	if err != nil || outcome != "pending" || replay.ID != first.ID {
		t.Fatalf("same attempt must replay its one pending question: record=%+v outcome=%q err=%v", replay, outcome, err)
	}
	answered, err := b.Answer(first.ID, "once")
	if err != nil || !answered.Yes || answered.Kept || answered.Option != "once" {
		t.Fatalf("once answer must permit this attempt without being kept: %+v err=%v", answered, err)
	}
	replay, outcome, err = b.AskApplication("attempt-scope", "ComfyUI", "replace-1", "content.replace", slots, listen.Seen{})
	if err != nil || outcome != "answered" || replay.ID != first.ID || replay.Option != "once" {
		t.Fatalf("same attempt must recover its once answer: record=%+v outcome=%q err=%v", replay, outcome, err)
	}
	second, outcome, err := b.AskApplication("attempt-scope", "ComfyUI", "replace-2", "content.replace", slots, listen.Seen{})
	if err != nil || outcome != "pending" || second.ID == first.ID {
		t.Fatalf("a new replacement attempt must ask again after once: record=%+v outcome=%q err=%v", second, outcome, err)
	}
	if _, err := b.Answer(second.ID, "never"); err != nil {
		t.Fatal(err)
	}
	third, outcome, err := b.AskApplication("attempt-scope", "ComfyUI", "replace-3", "content.replace", slots, listen.Seen{})
	if err != nil || outcome != "answered" || third.ID != second.ID || third.Yes || !third.Kept || third.Option != "never" {
		t.Fatalf("the existing never answer must remain kept for this content/scope: record=%+v outcome=%q err=%v", third, outcome, err)
	}
}

func TestApplicationCorruptStorageRefusesWithoutWrite(t *testing.T) {
	for _, data := range [][]byte{[]byte(`{"profile":"future","asks":null}`), []byte(`{"profile":"asks-application@1","asks":null,"profile":"asks-application@1"}`), bytes.Repeat([]byte{' '}, MaxApplicationBytes+1)} {
		path := filepath.Join(t.TempDir(), "application.json")
		if e := os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := LoadApplicationBook(path); e == nil {
			t.Fatal("accepted corrupt/oversized state")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(data, after) {
			t.Fatal("read refusal changed file")
		}
	}
}
