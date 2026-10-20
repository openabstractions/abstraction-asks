package asks

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	cas "github.com/openabstractions/abstraction-cas/go"
)

// InlineStorageRemoval is an already-answered Panel occurrence. These rows
// are private to the runtime's Asks owner. They never enter the pending Book.
type InlineStorageRemoval struct {
	Account, Program, Store, Digest, Observation, Occurrence string
	Size                                                     int64 `json:",omitempty"`
}

var ErrInlineStorageRemovalUsed = errors.New("asks: inline storage removal already answered")

type inlineStorageAnswer struct {
	InlineStorageRemoval
	Answered time.Time
	Removed  bool `json:",omitempty"`
}

const inlineStorageLimit = 2 << 20

// RecordInlineStorageRemoval atomically settles an authenticated click. Every
// removal requires a newly recorded occurrence, regardless of hold history.
func (b *Book) RecordInlineStorageRemoval(ctx context.Context, answer InlineStorageRemoval) error {
	if !b.application || b.owner == "" || b.owner != answer.Account || !answer.valid() {
		return errors.New("asks: invalid inline storage removal")
	}
	return cas.ChangeLimitContext(ctx, b.path+".storage-removal-v1.json", inlineStorageLimit, func(raw []byte) ([]byte, error) {
		rows, err := decodeInlineStorageAnswers(raw, b.owner)
		if err != nil {
			return nil, err
		}
		if _, used := rows[answer.Occurrence]; used {
			return nil, ErrInlineStorageRemovalUsed
		}
		rows[answer.Occurrence] = inlineStorageAnswer{InlineStorageRemoval: answer, Answered: b.clock().UTC()}
		return json.Marshal(rows)
	})
}

func (b *Book) CheckInlineStorageRemoval(ctx context.Context, answer InlineStorageRemoval) error {
	if !b.application || b.owner == "" || b.owner != answer.Account || !answer.valid() {
		return errors.New("asks: invalid inline storage removal")
	}
	raw, err := cas.ReadLimitContext(ctx, b.path+".storage-removal-v1.json", inlineStorageLimit)
	if err != nil {
		return err
	}
	rows, err := decodeInlineStorageAnswers(raw, b.owner)
	if err != nil {
		return err
	}
	if row, ok := rows[answer.Occurrence]; !ok || row.InlineStorageRemoval != answer {
		return errors.New("asks: exact inline storage answer unavailable")
	}
	return nil
}

func decodeInlineStorageAnswers(raw []byte, owner string) (map[string]inlineStorageAnswer, error) {
	rows := make(map[string]inlineStorageAnswer)
	if raw == nil {
		return rows, nil
	}
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil || !canonicalRetainedJSON(raw, &rows) {
		return nil, errors.New("asks: invalid inline storage answers")
	}
	for id, row := range rows {
		if id != row.Occurrence || row.Account != owner || row.Answered.IsZero() || !row.InlineStorageRemoval.valid() {
			return nil, errors.New("asks: invalid retained storage answer")
		}
	}
	return rows, nil
}

func (a InlineStorageRemoval) valid() bool {
	validHex := func(s string, n int) bool {
		decoded, err := hex.DecodeString(s)
		return len(s) == n && err == nil && hex.EncodeToString(decoded) == s
	}
	return a.Size >= 0 && validAskerProgram(a.Program) && a.Store == "runtime-content" &&
		strings.HasPrefix(a.Digest, "sha256:") && validHex(strings.TrimPrefix(a.Digest, "sha256:"), 64) &&
		validHex(a.Observation, 64) && validHex(a.Occurrence, 32)
}

// CompleteInlineStorageRemoval records that native deletion finished. A
// click alone supplies no removal event or restore authority.
func (b *Book) CompleteInlineStorageRemoval(ctx context.Context, occurrence string) error {
	return cas.ChangeLimitContext(ctx, b.path+".storage-removal-v1.json", inlineStorageLimit, func(raw []byte) ([]byte, error) {
		rows, err := decodeInlineStorageAnswers(raw, b.owner)
		if err != nil {
			return nil, err
		}
		row, ok := rows[occurrence]
		if !ok {
			return nil, errors.New("asks: completed removal has no answer")
		}
		if row.Removed {
			return raw, nil
		}
		row.Removed = true
		rows[occurrence] = row
		return json.Marshal(rows)
	})
}

type InlineStorageRemoved struct {
	InlineStorageRemoval
	At time.Time
}

// InlineStorageRemoved lists only completed removals in the runtime's
// private Activity source. It cannot answer or authorize any question.
func (b *Book) InlineStorageRemoved(ctx context.Context) ([]InlineStorageRemoved, error) {
	return b.inlineStorageHistory(ctx, true)
}

// InlineStorageRecovery returns retained answers to the runtime storage
// owner. An answer is not proof that the original file was removed.
func (b *Book) InlineStorageRecovery(ctx context.Context) ([]InlineStorageRemoved, error) {
	return b.inlineStorageHistory(ctx, false)
}

func (b *Book) inlineStorageHistory(ctx context.Context, completedOnly bool) ([]InlineStorageRemoved, error) {
	raw, err := cas.ReadLimitContext(ctx, b.path+".storage-removal-v1.json", inlineStorageLimit)
	if err != nil {
		return nil, err
	}
	rows, err := decodeInlineStorageAnswers(raw, b.owner)
	if err != nil {
		return nil, err
	}
	result := make([]InlineStorageRemoved, 0, len(rows))
	for _, row := range rows {
		if row.Removed || !completedOnly {
			result = append(result, InlineStorageRemoved{row.InlineStorageRemoval, row.Answered})
		}
	}
	slices.SortFunc(result, func(a, b InlineStorageRemoved) int {
		if order := b.At.Compare(a.At); order != 0 {
			return order
		}
		return strings.Compare(a.Occurrence, b.Occurrence)
	})
	return result, nil
}
