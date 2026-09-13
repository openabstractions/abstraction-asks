package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
	"path/filepath"
	"strconv"
	"time"
)

type receiver struct {
	host *Host
	call *listen.FramedCall
}

func callerScope(p *identity.Peer, owner string) (string, string, error) {
	if p == nil {
		return "", "", errors.New("asks: caller required")
	}
	u, e := p.User.AtLeast(listen.Program.User)
	if e != nil {
		return "", "", e
	}
	principal := ""
	if u.Kind == "windows" {
		principal = u.SID
	} else if u.Kind == "posix" && u.UID >= 0 {
		principal = strconv.Itoa(u.UID)
	}
	if principal == "" || principal != owner {
		return "", "", errors.New("asks: caller account refused")
	}
	path, e := p.Path.AtLeast(listen.Program.Path)
	if e != nil || !filepath.IsAbs(path) {
		return "", "", errors.New("asks: bound program required")
	}
	path = filepath.Clean(path)
	data, _ := json.Marshal([]string{"asks-owner-program@1", u.Kind, principal, path})
	sum := sha256.Sum256(data)
	return "asks-owner-program@1:" + hex.EncodeToString(sum[:]), filepath.Base(path), nil
}
func (r *receiver) scope() (string, string, error) {
	p, e := r.call.Peer()
	if e != nil {
		return "", "", e
	}
	return callerScope(p, r.host.owner)
}
func observation(record asks.Record, outcome string, e error) wire.QuestionObservation {
	if e != nil {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeUnavailable}
	}
	result := wire.QuestionObservation{Outcome: outcome}
	if outcome == wire.ObservationOutcomePending || outcome == wire.ObservationOutcomeAnswered {
		result.Answer = &wire.Answer{Id: record.ID, Pending: record.Pending(), Option: record.Option, Yes: record.Yes, Kept: record.Kept}
	}
	return result
}
func (r *receiver) Ask(q wire.ApplicationQuestion) (wire.QuestionObservation, error) {
	scope, display, e := r.scope()
	if e != nil {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeForbidden}, nil
	}
	record, outcome, e := r.host.book.AskApplication(scope, display, q.RequestKey, q.Key, q.Slots, r.call.Caller)
	return observation(record, outcome, e), nil
}
func (r *receiver) Observe(key string, waitMS int64) (wire.QuestionObservation, error) {
	scope, _, e := r.scope()
	if e != nil {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeForbidden}, nil
	}
	if waitMS < 0 || waitMS > 30000 || !asks.ValidRequestKey(key) {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeInvalid}, nil
	}
	record, outcome, e := r.host.book.ObserveApplication(scope, key)
	if e != nil || outcome != wire.ObservationOutcomePending || waitMS == 0 {
		return observation(record, outcome, e), nil
	}
	waitCtx := r.call.WaitContext()
	timer := time.NewTimer(time.Duration(waitMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-waitCtx.Done():
		return wire.QuestionObservation{}, waitCtx.Err()
	case <-timer.C:
	case <-r.host.book.Decided(record.ID):
	}
	// Recheck binding and storage after every wake; forgetting is an explicit gap.
	if _, _, e = r.scope(); e != nil {
		return wire.QuestionObservation{Outcome: wire.ObservationOutcomeForbidden}, nil
	}
	record, outcome, e = r.host.book.ObserveApplication(scope, key)
	return observation(record, outcome, e), nil
}
