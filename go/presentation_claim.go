package asks

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

const (
	presentationLease       = 10 * time.Second
	maxPresentationSessions = 64
)

// PresentationSession is provided by a trusted operator adapter from native
// account and interactive-session evidence. It must never come from a request.
type PresentationSession struct {
	Account string
	Session string
}

// PresentationClaim allows one presenter to show an exact pending occurrence
// until Expires. It does not confer answer authority.
type PresentationClaim struct {
	ID                 string
	Token              string
	Expires            time.Time
	explicit           bool
	providerCompletion bool
	program            string
	started            bool
}

var ErrPresentationBusy = errors.New("asks: presentation slot is held")

// ClaimPresentation selects one automatic popup per authenticated session.
// Live waits precede other pending questions. Equal timestamps keep Book order.
func (b *Book) ClaimPresentation(session PresentationSession, exact ...string) (PresentationClaim, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.presentationSessionValid(session) {
		return PresentationClaim{}, errors.New("asks: invalid presentation session")
	}
	now := b.clock()
	b.prunePopupRates(now)
	for key, claim := range b.presentationClaims {
		if !now.Before(claim.Expires) {
			delete(b.presentationClaims, key)
		}
	}
	if _, busy := b.presentationClaims[session]; busy {
		return PresentationClaim{}, ErrPresentationBusy
	}
	f, err := b.readApplication()
	if err != nil {
		return PresentationClaim{}, err
	}
	pending := make(map[string]bool, len(f.Asks))
	for _, record := range f.Asks {
		if b.presentationPending(record, now) {
			pending[record.ID] = true
		}
	}
	for key, seen := range b.presentationSeen {
		for id := range seen {
			if !pending[id] {
				delete(seen, id)
			}
		}
		if len(seen) == 0 {
			delete(b.presentationSeen, key)
		}
	}
	sessions := len(b.presentationSeen)
	for key := range b.presentationClaims {
		if b.presentationSeen[key] == nil {
			sessions++
		}
	}
	if b.presentationSeen[session] == nil && sessions >= maxPresentationSessions {
		return PresentationClaim{}, nil
	}
	var id string
	if len(exact) > 0 {
		id = exact[0]
	}
	var selected *Record
	for _, record := range f.Asks {
		if id != "" {
			if record.ID == id && b.explicitPresentation(record, now) {
				selected = record
				break
			}
			continue
		}
		if !b.presentationPending(record, now) || !AutomaticNativeQuestion(record.Key) || b.presentationSeen[session][record.ID] {
			continue
		}
		program := b.popupRequester(record)
		if rate, held := b.popupRates[program]; held {
			if !rate.noticeSent && rate.notice.ID == "" {
				rate.notice.Program, rate.notice.ID = program, record.ID
				b.popupRates[program] = rate
			}
			continue
		}
		if len(b.popupTimes) >= popupMinuteBudget || b.popupClaimed(program) {
			continue
		}
		if selected == nil || b.presentationPrecedes(record, selected) {
			selected = record
		}
	}
	if selected == nil {
		return PresentationClaim{}, nil
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return PresentationClaim{}, err
	}
	claim := PresentationClaim{ID: selected.ID, Token: hex.EncodeToString(random[:]), Expires: now.Add(presentationLease), explicit: id != "", program: b.popupRequester(selected)}
	if b.presentationClaims == nil {
		b.presentationClaims = make(map[PresentationSession]PresentationClaim)
	}
	b.presentationClaims[session] = claim
	return claim, nil
}

// RenewPresentation requires the same session, exact claim token, and a
// still-eligible occurrence. The presenter dismisses when renewal fails.
func (b *Book) RenewPresentation(session PresentationSession, token string) (PresentationClaim, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	current, ok := b.presentationClaims[session]
	now := b.clock()
	if !ok || token != current.Token || !now.Before(current.Expires) {
		return PresentationClaim{}, nil
	}
	f, err := b.readApplication()
	if err != nil {
		return PresentationClaim{}, err
	}
	if i := f.index(current.ID); i < 0 || !b.presentationRenewable(current, f.Asks[i], now) {
		return PresentationClaim{}, nil
	}
	current.Expires = now.Add(presentationLease)
	b.presentationClaims[session] = current
	return current, nil
}

// ReleasePresentation frees the slot after the presenter has dismissed it.
// A shown occurrence stays seen until its pending lifetime ends. Deferral
// or expiry leaves it eligible again.
func (b *Book) ReleasePresentation(session PresentationSession, token string, shown bool) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if current, ok := b.presentationClaims[session]; ok && current.Token == token && b.clock().Before(current.Expires) {
		delete(b.presentationClaims, session)
		if shown {

			if b.presentationSeen == nil {
				b.presentationSeen = make(map[PresentationSession]map[string]bool)
			}
			if b.presentationSeen[session] == nil {
				b.presentationSeen[session] = make(map[string]bool)
			}
			b.presentationSeen[session][current.ID] = true
		}
		return true
	}
	return false
}

func (b *Book) presentationSessionValid(session PresentationSession) bool {
	return b.application && session.Account != "" && session.Account == b.owner &&
		session.Session != "" && len(session.Session) <= 128
}

func (b *Book) presentationPending(record *Record, now time.Time) bool {
	return record.Pending() && now.Before(record.Asked.Add(b.pendingLifetime()))
}

func (b *Book) presentationPrecedes(a, other *Record) bool {
	waiting, otherWaiting := b.liveWaits[a.ID] > 0, b.liveWaits[other.ID] > 0
	if waiting != otherWaiting {
		return waiting
	}
	return a.Asked.Before(other.Asked)
}

func (b *Book) explicitPresentation(record *Record, now time.Time) bool {
	return record.Key == SetupModelsKey && b.presentationPending(record, now) || record.Key == FirstUseKey && record.Option == "allow"
}

func (b *Book) presentationRenewable(claim PresentationClaim, record *Record, now time.Time) bool {
	if claim.explicit {
		return b.explicitPresentation(record, now)
	}
	return b.presentationPending(record, now) || claim.providerCompletion && record.Option == "allow" && record.AnsweredBy == AnsweredByOperator
}

// BeginProviderPresentationCompletion records Allow and enables renewal in the
// same Book edit, only for the live automatic claim and verified provider pair.
func (b *Book) BeginProviderPresentationCompletion(session PresentationSession, token string, authorize func() error) (bool, error) {
	b.mu.Lock()
	claim := b.presentationClaims[session]
	b.mu.Unlock()
	if claim.Token != token || token == "" || claim.explicit {
		return false, nil
	}
	_, outcome, err := b.answerApplication(claim.ID, "allow", true, AnsweredByOperator, "", func(record Record, _ string) error {
		if err := authorize(); err != nil {
			return err
		}
		current := b.presentationClaims[session]
		if current.Token != token || !b.clock().Before(current.Expires) || record.Key != FirstUseKey || record.Permission == nil {
			return errProviderPresentationPhase
		}
		if _, ok := ProviderPermissionName(*record.Permission); !ok || !b.presentationPending(&record, b.clock()) {
			return errProviderPresentationPhase
		}
		return nil
	}, func() {
		current := b.presentationClaims[session]
		current.providerCompletion = true
		b.presentationClaims[session] = current
	})
	if errors.Is(err, errProviderPresentationPhase) {
		return false, nil
	}
	return outcome == "answered", err
}

var errProviderPresentationPhase = errors.New("asks: provider presentation is no longer eligible")
