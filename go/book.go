package asks

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	cas "github.com/openabstractions/abstraction-cas/go"
	"github.com/openabstractions/abstraction-identity/listen"
)

var (
	ErrNothingPending = errors.New("asks: nothing is pending under that id")
	ErrNoRecord       = errors.New("asks: no record under that id")
)

type book struct {
	Asks []*Record `json:"asks"`
}

type Book struct {
	mu      sync.Mutex
	path    string
	decided map[string]chan struct{}
}

func LoadBook(path string) (*Book, error) {
	b := &Book{path: path, decided: map[string]chan struct{}{}}
	_, err := b.read()
	return b, err
}

func (b *Book) read() (book, error) {
	var f book
	raw, err := cas.Read(b.path)
	if err != nil || raw == nil {
		return f, err
	}
	return f, json.Unmarshal(raw, &f)
}

func (b *Book) change(edit func(*book) error) error {
	return cas.Change(b.path, func(cur []byte) ([]byte, error) {
		var f book
		if cur != nil {
			if err := json.Unmarshal(cur, &f); err != nil {
				return nil, err
			}
		}
		if err := edit(&f); err != nil {
			return nil, err
		}
		sort.Slice(f.Asks, func(i, j int) bool { return f.Asks[i].Asked.Before(f.Asks[j].Asked) })
		raw, err := json.MarshalIndent(f, "", "  ")
		return append(raw, '\n'), err
	})
}

func (f *book) index(id string) int {
	return slices.IndexFunc(f.Asks, func(r *Record) bool { return r.ID == id })
}

func random(n int) string {
	buf := make([]byte, n)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (b *Book) Ask(a Ask, via listen.Seen) (Record, bool, error) {
	q, ok := Find(a.Key)
	if !ok {
		return Record{}, false, errors.New(ErrUnknownQuestion.Error() + ": " + a.Key)
	}
	if strings.TrimSpace(a.Asker) == "" {
		return Record{}, false, errors.New("asks: an asker needs a name")
	}
	text, err := q.Render(a)
	if err != nil {
		return Record{}, false, err
	}
	about := a.Slots[q.About]
	b.mu.Lock()
	defer b.mu.Unlock()
	var out Record
	created := false
	err = b.change(func(f *book) error {
		var pending *Record
		for _, r := range f.Asks {
			if r.Asker != a.Asker || r.Key != a.Key || r.About != about {
				continue
			}
			if r.Pending() {
				pending = r
			} else if r.Kept {
				out = *r
				return nil
			}
		}
		if pending != nil {
			out = *pending
			return nil
		}
		r := &Record{ID: random(3), Asker: a.Asker, Key: a.Key, About: about, Text: text,
			Options: q.OptionNames(), Asked: time.Now().UTC(), Via: via, For: a.For}
		f.Asks = append(f.Asks, r)
		out, created = *r, true
		return nil
	})
	if err != nil {
		return Record{}, false, err
	}
	return out, created, nil
}

func (b *Book) Decided(id string) <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.decidedLocked(id)
}

func (b *Book) decidedLocked(id string) chan struct{} {
	ch, ok := b.decided[id]
	if !ok {
		ch = make(chan struct{})
		b.decided[id] = ch
		if r, ok := b.get(id); !ok || !r.Pending() {
			close(ch)
		}
	}
	return ch
}

func (b *Book) settle(id string) {
	if ch, ok := b.decided[id]; ok {
		close(ch)
		delete(b.decided, id)
	}
}

func (b *Book) Answer(id, option string) (Record, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out Record
	err := b.change(func(f *book) error {
		i := f.index(id)
		if i < 0 || !f.Asks[i].Pending() {
			return ErrNothingPending
		}
		r := f.Asks[i]
		q, _ := Find(r.Key)
		o, ok := q.Option(option)
		if !ok {
			return errors.New(ErrUnknownOption.Error() + ": " + option + "; one of " + strings.Join(r.Options, ", "))
		}
		r.Option, r.Yes, r.Kept, r.Answered = o.Name, o.Yes, o.Kept, time.Now().UTC()
		out = *r
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	b.settle(id)
	return out, nil
}

func (b *Book) get(id string) (Record, bool) {
	f, _ := b.read()
	if i := f.index(id); i >= 0 {
		return *f.Asks[i], true
	}
	return Record{}, false
}

func (b *Book) Get(id string) (Record, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.get(id)
}

func (b *Book) Forget(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	err := b.change(func(f *book) error {
		i := f.index(id)
		if i < 0 {
			return ErrNoRecord
		}
		f.Asks = slices.Delete(f.Asks, i, i+1)
		return nil
	})
	if err == nil {
		b.settle(id)
	}
	return err
}

func (b *Book) List(pending bool) []Record {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, _ := b.read()
	var out []Record
	for _, r := range f.Asks {
		if r.Pending() == pending {
			out = append(out, *r)
		}
	}
	return out
}
