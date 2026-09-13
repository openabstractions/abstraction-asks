package asks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	cas "github.com/openabstractions/abstraction-cas/go"
	"github.com/openabstractions/abstraction-identity/listen"
	"io"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const ApplicationProfile = "asks-application@1"
const MaxApplicationBytes = 4 << 20
const MaxApplicationAdmissions = 4096

var ErrApplicationCapacity = errors.New("asks: application capacity reached")

type applicationAdmission struct {
	Scope       string `json:"scope"`
	Key         string `json:"key"`
	Fingerprint string `json:"fingerprint"`
	ID          string `json:"id"`
}

// LoadApplicationBook opens the separate application-profile file. The host owns
// this file; legacy binaries must never be configured with its path. Native
// Answer/Forget on this returned Book are explicit trusted operator integration.
func LoadApplicationBook(path string) (*Book, error) {
	b := &Book{path: path, application: true, decided: map[string]chan struct{}{}}
	_, e := b.readApplication()
	return b, e
}
func (b *Book) ApplicationProfile() bool { return b != nil && b.application }
func decodeApplication(data []byte) (book, error) {
	if data == nil {
		return book{Profile: ApplicationProfile}, nil
	}
	var f book
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(&f); e != nil {
		return f, e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return f, errors.New("asks: trailing application state")
	}
	if f.Profile != ApplicationProfile || len(f.Admissions) > MaxApplicationAdmissions || len(f.Asks) > MaxApplicationAdmissions {
		return f, errors.New("asks: unsupported application state")
	}
	ids := map[string]bool{}
	keys := map[string]bool{}
	for _, r := range f.Asks {
		if r == nil || r.ID == "" || ids[r.ID] {
			return f, errors.New("asks: invalid application record")
		}
		q, ok := Find(r.Key)
		if !ok || r.Asked.IsZero() || r.Asker == "" {
			return f, errors.New("asks: invalid application question")
		}
		if r.Pending() {
			if r.Yes || r.Kept || !r.Answered.IsZero() {
				return f, errors.New("asks: invalid pending decision")
			}
		} else {
			option, ok := q.Option(r.Option)
			if !ok || option.Yes != r.Yes || option.Kept != r.Kept || r.Answered.IsZero() {
				return f, errors.New("asks: invalid recorded decision")
			}
		}
		ids[r.ID] = true
	}
	for _, a := range f.Admissions {
		key := a.Scope + "\x00" + a.Key
		digest, decodeErr := hex.DecodeString(a.Fingerprint)
		if !ValidRequestKey(a.Scope) || !ValidRequestKey(a.Key) || decodeErr != nil || len(digest) != 32 || a.ID == "" || keys[key] {
			return f, errors.New("asks: invalid application admission")
		}
		keys[key] = true
	}
	// This version owns a canonical JSON file. Refuse duplicate fields, invalid
	// Unicode and alternate spellings before they can normalize authority data.
	canonical, e := json.Marshal(&f)
	if e != nil || !bytes.Equal(data, canonical) {
		return f, errors.New("asks: noncanonical application state")
	}
	return f, nil
}
func (b *Book) readApplication() (book, error) {
	if e := b.applicationFile(); e != nil {
		return book{}, e
	}
	data, e := cas.ReadLimit(b.path, MaxApplicationBytes)
	if e != nil {
		return book{}, e
	}
	return decodeApplication(data)
}
func (b *Book) changeApplication(edit func(*book) error) error {
	if e := b.applicationFile(); e != nil {
		return e
	}
	return cas.ChangeLimit(b.path, MaxApplicationBytes, func(data []byte) ([]byte, error) {
		f, e := decodeApplication(data)
		if e != nil {
			return nil, e
		}
		if e = edit(&f); e != nil {
			return nil, e
		}
		return json.Marshal(&f)
	})
}
func (b *Book) applicationFile() error {
	info, e := os.Lstat(b.path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return errors.New("asks: application state must be a regular file")
	}
	return nil
}
func ValidRequestKey(key string) bool {
	return len(key) > 0 && len(key) <= 128 && utf8.ValidString(key) && strings.IndexFunc(key, unicode.IsControl) < 0
}
func applicationQuestion(display, key string, slots map[string]string) (Ask, string, error) {
	q, ok := Find(key)
	if !ok {
		return Ask{}, "", ErrUnknownQuestion
	}
	required := map[string]bool{}
	for _, m := range slotRE.FindAllStringSubmatch(q.Text, -1) {
		if m[1] != "asker" {
			required[m[1]] = true
		}
	}
	if len(slots) != len(required) {
		return Ask{}, "", ErrBadSlot
	}
	for k, v := range slots {
		if !required[k] || !utf8.ValidString(v) || utf8.RuneCountInString(v) > maxSlot || strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return Ask{}, "", ErrBadSlot
		}
	}
	a := Ask{Asker: display, Key: key, Slots: slots}
	text, e := q.Render(a)
	return a, text, e
}

// AskApplication receives only receiver-derived scope/display/evidence. It is a
// provider method; the application service supplies these after native binding.
func (b *Book) AskApplication(scope, display, requestKey, key string, slots map[string]string, via listen.Seen) (Record, string, error) {
	if !b.application {
		return Record{}, "", errors.New("asks: application Book required")
	}
	if scope == "" || !ValidRequestKey(requestKey) {
		return Record{}, "invalid", nil
	}
	a, text, e := applicationQuestion(display, key, slots)
	if e != nil {
		return Record{}, "invalid", nil
	}
	canonical, _ := json.Marshal(struct {
		Key   string
		Slots map[string]string
	}{key, slots})
	sum := sha256.Sum256(canonical)
	fingerprint := hex.EncodeToString(sum[:])
	b.mu.Lock()
	defer b.mu.Unlock()
	var out Record
	outcome := ""
	e = b.changeApplication(func(f *book) error {
		for _, ad := range f.Admissions {
			if ad.Scope == scope && ad.Key == requestKey {
				if ad.Fingerprint != fingerprint {
					outcome = "conflict"
					return nil
				}
				i := f.index(ad.ID)
				if i < 0 {
					outcome = "gone"
					return nil
				}
				out = *f.Asks[i]
				outcome = recordOutcome(out)
				return nil
			}
		}
		if len(f.Admissions) >= MaxApplicationAdmissions {
			return ErrApplicationCapacity
		}
		for _, ad := range f.Admissions {
			if ad.Scope != scope || ad.Fingerprint != fingerprint {
				continue
			}
			i := f.index(ad.ID)
			if i >= 0 && (f.Asks[i].Pending() || f.Asks[i].Kept) {
				out = *f.Asks[i]
				break
			}
		}
		if out.ID == "" {
			if len(f.Asks) >= MaxApplicationAdmissions {
				return ErrApplicationCapacity
			}
			q, _ := Find(key)
			out = Record{ID: random(16), Asker: display, Key: key, About: a.Slots[q.About], Text: text, Options: q.OptionNames(), Asked: time.Now().UTC(), Via: via}
			copy := out
			f.Asks = append(f.Asks, &copy)
		}
		f.Admissions = append(f.Admissions, applicationAdmission{scope, requestKey, fingerprint, out.ID})
		outcome = recordOutcome(out)
		return nil
	})
	return out, outcome, e
}
func recordOutcome(r Record) string {
	if r.Pending() {
		return "pending"
	}
	return "answered"
}
func (b *Book) ObserveApplication(scope, key string) (Record, string, error) {
	if !b.application {
		return Record{}, "", fmt.Errorf("asks: application Book required")
	}
	if scope == "" || !ValidRequestKey(key) {
		return Record{}, "invalid", nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	f, e := b.readApplication()
	if e != nil {
		return Record{}, "", e
	}
	for _, a := range f.Admissions {
		if a.Scope == scope && a.Key == key {
			i := f.index(a.ID)
			if i < 0 {
				return Record{}, "gone", nil
			}
			r := *f.Asks[i]
			return r, recordOutcome(r), nil
		}
	}
	return Record{}, "unknown", nil
}
