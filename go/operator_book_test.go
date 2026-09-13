package asks

import (
	"bytes"
	"errors"
	"github.com/openabstractions/abstraction-identity/listen"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOperatorBookReplayPreservesDecisionAndTombstone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	pending, _, err := b.AskApplication("scope", "application", "request", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if err != nil {
		t.Fatal(err)
	}
	first, outcome, err := b.AnswerApplication(pending.ID, "once")
	if err != nil || outcome != "answered" {
		t.Fatal(outcome, err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	replay, outcome, err := reopened.AnswerApplication(pending.ID, "once")
	if err != nil || outcome != "answered" || !reflect.DeepEqual(first, replay) {
		t.Fatal(replay, outcome, err)
	}
	for _, tc := range []struct{ option, outcome string }{{"refuse", "conflict"}, {"bogus", "invalid"}} {
		_, outcome, err = reopened.AnswerApplication(pending.ID, tc.option)
		if err != nil || outcome != tc.outcome {
			t.Fatal(outcome, err)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("replay/refusal changed bytes", err)
	}
	if err = reopened.Forget(pending.ID); err != nil {
		t.Fatal(err)
	}
	_, outcome, err = reopened.AnswerApplication(pending.ID, "once")
	if err != nil || outcome != "unknown" {
		t.Fatal(outcome, err)
	}
	_, outcome, err = reopened.AskApplication("scope", "application", "request", "download.reach", map[string]string{"host": "example.com"}, listen.Seen{})
	if err != nil || outcome != "gone" {
		t.Fatal("operator resurrected admission", outcome, err)
	}
}
func TestOperatorBookStorageFailurePreservesEvidence(t *testing.T) {
	for _, data := range [][]byte{[]byte(`{"bad":`), bytes.Repeat([]byte("x"), MaxApplicationBytes+1)} {
		path := filepath.Join(t.TempDir(), "book.json")
		b, err := LoadApplicationBook(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err = b.ApplicationHistory(); err == nil {
			t.Fatal("bad history appeared empty")
		}
		if _, outcome, err := b.AnswerApplication("missing", "once"); err == nil || outcome != "unavailable" {
			t.Fatal(outcome, err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, after) {
			t.Fatal("bad state changed", err)
		}
	}
}

func TestOperatorUnknownDoesNotCreateBook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.json")
	b, err := LoadApplicationBook(path)
	if err != nil {
		t.Fatal(err)
	}
	_, outcome, err := b.AnswerApplication("missing", "once")
	if err != nil || outcome != "unknown" {
		t.Fatal(outcome, err)
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unknown answer created state", err)
	}
}
