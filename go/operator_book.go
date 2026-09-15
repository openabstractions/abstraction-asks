package asks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"time"
)

// ApplicationHistory reads the bounded application-profile snapshot for an
// explicitly authorized operator. It propagates storage errors; native List
// retains its legacy signature. The revision describes this exact snapshot.
func (b *Book) ApplicationHistory() ([]Record, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return nil, "", errors.New("asks: application Book required")
	}
	f, err := b.readApplication()
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(&f)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	records := make([]Record, 0, len(f.Asks))
	for _, r := range f.Asks {
		records = append(records, *r)
	}
	return records, hex.EncodeToString(sum[:]), nil
}

// AnswerApplication records an explicit operator selection. Same-option replay
// is idempotent across uncertain replies/restarts; a different answer conflicts.
// The caller must authorize the operator before invoking this provider method.
func (b *Book) AnswerApplication(id, option string) (Record, string, error) {
	return b.AnswerApplicationAuthorized(id, option, nil)
}

// AnswerApplicationAuthorized rechecks trusted service admission inside the
// atomic edit, after lock waiting and before any decision effect.
func (b *Book) AnswerApplicationAuthorized(id, option string, authorize func() error) (Record, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return Record{}, "unavailable", errors.New("asks: application Book required")
	}
	if !ValidRequestKey(id) || !ValidRequestKey(option) {
		return Record{}, "invalid", nil
	}
	var out Record
	outcome := "unknown"
	unchanged := errors.New("asks: operator result without edit")
	err := b.changeApplication(func(f *book) error {
		if authorize != nil {
			if err := authorize(); err != nil {
				return err
			}
		}
		i := f.index(id)
		if i < 0 {
			return unchanged
		}
		r := f.Asks[i]
		q, ok := Find(r.Key)
		if !ok {
			return ErrUnknownQuestion
		}
		o, ok := q.Option(option)
		if !ok {
			outcome = "invalid"
			return unchanged
		}
		if !r.Pending() {
			if r.Option != option {
				outcome = "conflict"
				return unchanged
			}
			out = *r
			outcome = "answered"
			return unchanged
		}
		r.Option, r.Yes, r.Kept, r.Answered = o.Name, o.Yes, o.Kept, time.Now().UTC()
		out = *r
		outcome = "answered"
		return nil
	})
	if errors.Is(err, unchanged) {
		err = nil
	}
	if err != nil {
		return Record{}, "unavailable", err
	}
	if outcome == "answered" {
		b.settle(id)
	}
	return out, outcome, nil
}

// RetireApplicationAuthorized removes a retained pending or answered question.
// Trusted operator admission is rechecked inside the atomic edit, after lock
// waiting and before removal. Admission tombstones remain, so application
// replay and observation report gone. An ID named only by an admission was
// already retired or forgotten and replays retired without a record.
func (b *Book) RetireApplicationAuthorized(id string, authorize func() error) (Record, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.application {
		return Record{}, "unavailable", errors.New("asks: application Book required")
	}
	if !ValidRequestKey(id) {
		return Record{}, "invalid", nil
	}
	var out Record
	outcome := "unknown"
	unchanged := errors.New("asks: retirement without edit")
	err := b.changeApplication(func(f *book) error {
		if authorize != nil {
			if err := authorize(); err != nil {
				return err
			}
		}
		i := f.index(id)
		if i < 0 {
			for _, a := range f.Admissions {
				if a.ID == id {
					outcome = "retired"
					break
				}
			}
			return unchanged
		}
		out = *f.Asks[i]
		f.Asks = slices.Delete(f.Asks, i, i+1)
		outcome = "retired"
		return nil
	})
	if errors.Is(err, unchanged) {
		return Record{}, outcome, nil
	}
	if err != nil {
		return Record{}, "unavailable", err
	}
	b.settle(id)
	return out, outcome, nil
}
