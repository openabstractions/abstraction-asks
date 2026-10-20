package asks

import (
	"errors"
	"slices"
	"time"
)

// DefaultPendingLifetime is how long an application-profile question stays
// pending before it expires (ASK-A7): the 15-minute device-code lifetime
// GitHub and Microsoft Entra default to for RFC 8628 device authorization,
// the consent request shaped most like a question the requester was refused
// on at once and asks again about later.
const DefaultPendingLifetime = 15 * time.Minute

// SetPendingLifetime sets how long this Book's questions stay pending
// (ASK-A7). The lifetime must be positive.
func (b *Book) SetPendingLifetime(d time.Duration) error {
	if d <= 0 {
		return errors.New("asks: a pending lifetime must be positive")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lifetime = d
	return nil
}

func (b *Book) pendingLifetime() time.Duration {
	if b.lifetime > 0 {
		return b.lifetime
	}
	return DefaultPendingLifetime
}

// SetKeyLifetime sets how long this Book's pending questions of one key stay
// pending (ASK-A7). It takes precedence over SetPendingLifetime for that key.
// The lifetime must be positive.
func (b *Book) SetKeyLifetime(key string, d time.Duration) error {
	if d <= 0 || key == "" {
		return errors.New("asks: a key lifetime must be positive and name a key")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.keyLifetimes == nil {
		b.keyLifetimes = map[string]time.Duration{}
	}
	b.keyLifetimes[key] = d
	return nil
}

// lifetimeOf is the pending lifetime of a question of key. Callers hold b.mu.
func (b *Book) lifetimeOf(key string) time.Duration {
	if d, ok := b.keyLifetimes[key]; ok {
		return d
	}
	return b.pendingLifetime()
}

// expire removes every question still pending a full lifetime after it was
// asked, with each admission naming it, and returns their ids. The keys of
// those admissions admit a new question; an answered question never expires.
func (f *book) expire(now time.Time, lifetime func(key string) time.Duration) []string {
	var gone []string
	kept := f.Asks[:0]
	for _, r := range f.Asks {
		if r.Pending() && !now.Before(r.Asked.Add(lifetime(r.Key))) {
			gone = append(gone, r.ID)
			continue
		}
		kept = append(kept, r)
	}
	if len(gone) == 0 {
		return nil
	}
	f.Asks = kept
	admissions := f.Admissions[:0]
	for _, a := range f.Admissions {
		if !slices.Contains(gone, a.ID) {
			admissions = append(admissions, a)
		}
	}
	f.Admissions = admissions
	return gone
}

func (b *Book) clock() time.Time {
	if b.now != nil {
		return b.now()
	}
	return time.Now()
}

// PendingUntil is the time a pending question expires (ASK-A7): its recorded
// asked time plus this Book's pending lifetime for its key.
func (b *Book) PendingUntil(r Record) time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return r.Asked.Add(b.lifetimeOf(r.Key))
}
