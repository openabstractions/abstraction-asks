package asks

import (
	"time"

	identity "github.com/openabstractions/abstraction-identity"
)

const (
	popupAppInterval  = 10 * time.Second
	popupMinuteBudget = 12
)

// PresentationHold is an operator-only summary of a suppressed popup.
type PresentationHold struct {
	Program string
	ID      string
	Token   string
	Expires time.Time
}

type popupRate struct {
	until      time.Time
	notice     PresentationHold
	noticeSent bool
}

// popupRequester uses immutable permission subjects or the runtime-attributed
// asker executable. Presentation text and admission scope are not identities.
func (b *Book) popupRequester(r *Record) string {
	program := recordProgram(r)
	if r.Permission == nil && r.TypedPermission == nil && validAskerProgram(program) {
		return identity.NormalizeSubjectProgram(program)
	}
	return program
}

func (b *Book) prunePopupRates(now time.Time) {
	for program, rate := range b.popupRates {
		if !now.Before(rate.until) {
			delete(b.popupRates, program)
		}
	}
	kept := b.popupTimes[:0]
	for _, stamp := range b.popupTimes {
		if now.Before(stamp.Add(time.Minute)) {
			kept = append(kept, stamp)
		}
	}
	b.popupTimes = kept
}

// PresentationHoldNotice reads an unacknowledged hold window.
func (b *Book) PresentationHoldNotice(session PresentationSession) PresentationHold {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.presentationSessionValid(session) {
		return PresentationHold{}
	}
	b.prunePopupRates(b.clock())
	for _, rate := range b.popupRates {
		if !rate.noticeSent && rate.notice.ID != "" {
			return rate.notice
		}
	}
	return PresentationHold{}
}

func (b *Book) AcknowledgePresentationHold(session PresentationSession, token string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.presentationSessionValid(session) || token == "" {
		return false
	}
	b.prunePopupRates(b.clock())
	for program, rate := range b.popupRates {
		if rate.notice.Token != token {
			continue
		}
		rate.noticeSent = true
		b.popupRates[program] = rate
		return true
	}
	return false
}

func (b *Book) StartPresentation(session PresentationSession, token string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	claim, ok := b.presentationClaims[session]
	now := b.clock()
	if !ok || claim.Token != token || !now.Before(claim.Expires) {
		return false, nil
	}
	f, err := b.readApplication()
	if err != nil {
		return false, err
	}
	if i := f.index(claim.ID); i < 0 || !b.presentationRenewable(claim, f.Asks[i], now) {
		return false, nil
	}
	if !claim.started {
		if !claim.explicit {
			b.recordPopup(claim.program, token, now)
		}
		claim.started = true
		b.presentationClaims[session] = claim
	}
	return true, nil
}

func (b *Book) recordPopup(program, token string, now time.Time) {
	b.prunePopupRates(now)
	b.popupTimes = append(b.popupTimes, now)
	if program != "" {
		if b.popupRates == nil {
			b.popupRates = make(map[string]popupRate)
		}
		b.popupRates[program] = popupRate{until: now.Add(popupAppInterval), notice: PresentationHold{Token: token, Expires: now.Add(24 * time.Hour)}}
	}
}

func (b *Book) popupClaimed(program string) bool {
	active := 0
	for _, claim := range b.presentationClaims {
		if claim.explicit || claim.started {
			continue
		}
		if program != "" && claim.program == program {
			return true
		}
		active++
	}
	return len(b.popupTimes)+active >= popupMinuteBudget
}
