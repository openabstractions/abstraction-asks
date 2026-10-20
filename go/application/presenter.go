package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/presenter"
	identity "github.com/openabstractions/abstraction-identity"
)

// PresentationSession derives the interactive session from a retained native peer.
type PresentationSession func(*identity.Peer) (string, bool)

func (h *Host) EnablePresenter(session PresentationSession) error {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	if h.serving || h.ctx.Err() != nil || h.operator == nil || session == nil {
		return errors.New("asks: configure presenter with operator authorization before Serve")
	}
	h.presenter = session
	return nil
}

type presenterReceiver struct {
	operatorReceiver
}

type presenterBinding struct {
	session asks.PresentationSession
	process identity.Process
	scope   string
	pin     *identity.Binding
}

func (r *presenterReceiver) bind() (presenterBinding, wire.LeaseOutcome) {
	scope, err := r.authorize()
	if errors.Is(err, ErrOperatorForbidden) {
		return presenterBinding{}, wire.LeaseOutcomeForbidden
	}
	if err != nil {
		return presenterBinding{}, wire.LeaseOutcomeUnavailable
	}
	pin, err := r.call.RetainProcess()
	if err != nil {
		return presenterBinding{}, wire.LeaseOutcomeUnavailable
	}
	keep := false
	defer func() {
		if !keep {
			pin.Close()
		}
	}()
	peer, err := pin.Peer()
	if err != nil {
		return presenterBinding{}, wire.LeaseOutcomeUnavailable
	}
	process, err := peer.Process.AtLeast(identity.ProofKernel)
	if err != nil || process.PID <= 0 || process.Recycled || process.StartTime.IsZero() && process.Generation == 0 {
		return presenterBinding{}, wire.LeaseOutcomeUnavailable
	}
	session, ok := r.host.presenter(peer)
	if !ok || session == "" {
		return presenterBinding{}, wire.LeaseOutcomeForbidden
	}
	alive, err := pin.Alive()
	if err != nil || !alive || r.ctx.Err() != nil {
		return presenterBinding{}, wire.LeaseOutcomeUnavailable
	}
	if until := pin.EvidenceValidUntil(); !until.IsZero() && !time.Now().Before(until) {
		return presenterBinding{}, wire.LeaseOutcomeUnavailable
	}
	keep = true
	return presenterBinding{asks.PresentationSession{Account: r.host.owner, Session: session}, process, scope, pin}, wire.LeaseOutcomeHeld
}

func (r *presenterReceiver) signedToken(binding presenterBinding, token string) string {
	mac := hmac.New(sha256.New, r.host.presentationKey[:])
	fmt.Fprintf(mac, "asks-presenter-v1:%q:%q:%q:%d:%d:%d:%s", binding.session.Account, binding.session.Session, binding.scope,
		binding.process.PID, binding.process.StartTime.UnixNano(), binding.process.Generation, token)
	return token + hex.EncodeToString(mac.Sum(nil))
}

func (r *presenterReceiver) validToken(binding presenterBinding, token string) bool {
	return len(token) == 96 && hmac.Equal([]byte(token), []byte(r.signedToken(binding, token[:32])))
}

func (r *presenterReceiver) Claim(id string) (wire.Lease, error) {
	binding, outcome := r.bind()
	if outcome != wire.LeaseOutcomeHeld {
		return wire.Lease{Outcome: outcome}, nil
	}
	defer binding.pin.Close()
	claim, err := r.host.book.ClaimPresentation(binding.session, id)
	if errors.Is(err, asks.ErrPresentationBusy) {
		return wire.Lease{Outcome: wire.LeaseOutcomeBusy}, nil
	}
	return r.result(binding, claim, err, wire.LeaseOutcomeIdle), nil
}

func (r *presenterReceiver) Renew(token string) (wire.Lease, error) {
	binding, outcome := r.bind()
	if outcome != wire.LeaseOutcomeHeld {
		return wire.Lease{Outcome: outcome}, nil
	}
	defer binding.pin.Close()
	if !r.validToken(binding, token) {
		return wire.Lease{Outcome: wire.LeaseOutcomeStale}, nil
	}
	claim, err := r.host.book.RenewPresentation(binding.session, token[:32])
	return r.result(binding, claim, err, wire.LeaseOutcomeStale), nil
}

func (r *presenterReceiver) Release(token string, shown bool) (wire.Lease, error) {
	binding, outcome := r.bind()
	if outcome != wire.LeaseOutcomeHeld {
		return wire.Lease{Outcome: outcome}, nil
	}
	defer binding.pin.Close()
	if !r.validToken(binding, token) || !r.host.book.ReleasePresentation(binding.session, token[:32], shown) {
		return wire.Lease{Outcome: wire.LeaseOutcomeStale}, nil
	}
	return wire.Lease{Outcome: wire.LeaseOutcomeReleased}, nil
}

func (r *presenterReceiver) result(binding presenterBinding, claim asks.PresentationClaim, err error, absent wire.LeaseOutcome) wire.Lease {
	if err != nil {
		return wire.Lease{Outcome: wire.LeaseOutcomeUnavailable}
	}
	if claim.ID == "" {
		return wire.Lease{Outcome: absent}
	}
	return wire.Lease{Outcome: wire.LeaseOutcomeHeld, ID: claim.ID, Token: r.signedToken(binding, claim.Token), ExpiresMs: claim.Expires.UnixMilli()}
}

func (r *presenterReceiver) BeginProviderCompletion(token string) (wire.Lease, error) {
	binding, outcome := r.bind()
	if outcome != wire.LeaseOutcomeHeld {
		return wire.Lease{Outcome: outcome}, nil
	}
	defer binding.pin.Close()
	if !r.validToken(binding, token) {
		return wire.Lease{Outcome: wire.LeaseOutcomeStale}, nil
	}
	ok, err := r.host.book.BeginProviderPresentationCompletion(binding.session, token[:32], func() error {
		fresh, outcome := r.bind()
		if outcome == wire.LeaseOutcomeForbidden {
			return ErrOperatorForbidden
		}
		if outcome != wire.LeaseOutcomeHeld {
			return errors.New("asks: presenter evidence unavailable")
		}
		defer fresh.pin.Close()
		if !r.validToken(fresh, token) {
			return ErrOperatorForbidden
		}
		return nil
	})
	switch {
	case errors.Is(err, ErrOperatorForbidden):
		return wire.Lease{Outcome: wire.LeaseOutcomeForbidden}, nil
	case err != nil:
		return wire.Lease{Outcome: wire.LeaseOutcomeUnavailable}, nil
	case !ok:
		return wire.Lease{Outcome: wire.LeaseOutcomeStale}, nil
	default:
		return wire.Lease{Outcome: wire.LeaseOutcomeCompletion}, nil
	}
}

func (r *presenterReceiver) ReadHoldNotice() (wire.HoldNotice, error) {
	binding, outcome := r.bind()
	if outcome != wire.LeaseOutcomeHeld {
		return wire.HoldNotice{Outcome: outcome}, nil
	}
	defer binding.pin.Close()
	notice := r.host.book.PresentationHoldNotice(binding.session)
	if notice.ID == "" {
		return wire.HoldNotice{Outcome: wire.LeaseOutcomeIdle}, nil
	}
	return wire.HoldNotice{Outcome: wire.LeaseOutcomeHeld, Program: notice.Program, ID: notice.ID, Token: notice.Token, ExpiresMs: notice.Expires.UnixMilli()}, nil
}

func (r *presenterReceiver) AckHoldNotice(token string) (wire.Lease, error) {
	binding, outcome := r.bind()
	if outcome != wire.LeaseOutcomeHeld {
		return wire.Lease{Outcome: outcome}, nil
	}
	defer binding.pin.Close()
	if !r.host.book.AcknowledgePresentationHold(binding.session, token) {
		return wire.Lease{Outcome: wire.LeaseOutcomeStale}, nil
	}
	return wire.Lease{Outcome: wire.LeaseOutcomeHeld}, nil
}

func (r *presenterReceiver) Start(token string) (wire.Lease, error) {
	binding, outcome := r.bind()
	if outcome != wire.LeaseOutcomeHeld {
		return wire.Lease{Outcome: outcome}, nil
	}
	defer binding.pin.Close()
	if !r.validToken(binding, token) {
		return wire.Lease{Outcome: wire.LeaseOutcomeStale}, nil
	}
	started, err := r.host.book.StartPresentation(binding.session, token[:32])
	if err != nil {
		return wire.Lease{Outcome: wire.LeaseOutcomeUnavailable}, nil
	}
	if !started {
		return wire.Lease{Outcome: wire.LeaseOutcomeStale}, nil
	}
	return wire.Lease{Outcome: wire.LeaseOutcomeHeld}, nil
}
