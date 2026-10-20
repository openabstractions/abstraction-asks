package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

// NotificationPolicy is trusted host configuration deciding, for every Post
// and Act after same-account Program proof, whether the bound program may
// notify (ASK-N1, ASK-N6): abstraction.asks/notification.post on account.
// Return nil to admit, asks.ErrNotificationSuppressed for a live explicit
// block, ErrAskPolicyUnavailable when no decision was obtained, and any other
// error for an evaluated refusal.
type NotificationPolicy func(ctx context.Context, peer *identity.Peer) error

// NotificationBlocked is the same decision for a program named by its bound
// path, used when an operator acts on another program's notification. It
// returns asks.ErrNotificationSuppressed for a live explicit block, nil when
// no block stands, and any other error when the decision is uncertain.
type NotificationBlocked func(ctx context.Context, program string) error

// OperationOwner is trusted host configuration that confirms a job operation
// was accepted in the posting caller's own scope (ASK-N1). Return nil when it
// was, ErrOperationNotOwned when it was not or does not exist, and any other
// error when the owner could not be reached.
type OperationOwner func(ctx context.Context, peer *identity.Peer, operationID string) error

// ErrOperationNotOwned refuses an operation reference outside the caller's scope.
var ErrOperationNotOwned = errors.New("asks: operation is not the caller's own")

// EnableNotifications serves notification@1 and notification-operator@1.
// Without it every Post is forbidden. owner may be nil; then a request naming
// an operation is invalid. Configure it before Serve.
func (h *Host) EnableNotifications(policy NotificationPolicy, blocked NotificationBlocked, owner OperationOwner) error {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	if h.serving || h.ctx.Err() != nil {
		return errors.New("asks: configure notifications before Serve")
	}
	if policy == nil || blocked == nil {
		return errors.New("asks: explicit notification policy required")
	}
	h.notificationPolicy, h.notificationBlocked, h.operationOwner = policy, blocked, owner
	return nil
}

// presentationToken binds one record to this host epoch (ASK-N4). A native
// item shown before a restart carries another epoch's token and is stale.
func (h *Host) presentationToken(id string) string {
	mac := hmac.New(sha256.New, h.presentationKey[:])
	mac.Write([]byte("asks-notification-presentation@1\x00" + id)) //unchecked: a hash never returns a write error
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

type notificationReceiver struct{ receiver }

func (r *notificationReceiver) policy() error {
	if err := r.call.WaitContext().Err(); err != nil {
		return err
	}
	if _, _, err := r.scope(); err != nil {
		return ErrOperatorForbidden
	}
	if r.host.notificationPolicy == nil {
		return ErrOperatorForbidden
	}
	peer, err := r.call.Peer()
	if err != nil {
		return err
	}
	if err = r.host.notificationPolicy(r.call.WaitContext(), peer); err != nil {
		if errors.Is(err, asks.ErrNotificationSuppressed) || errors.Is(err, ErrAskPolicyUnavailable) {
			return err
		}
		return ErrOperatorForbidden
	}
	return r.call.Recheck()
}

func notificationFailure(err error) wire.NotificationOutcome {
	switch {
	case errors.Is(err, asks.ErrNotificationSuppressed):
		return wire.NotificationOutcomeSuppressed
	case errors.Is(err, ErrOperatorForbidden):
		return wire.NotificationOutcomeForbidden
	case errors.Is(err, ErrOperationNotOwned):
		return wire.NotificationOutcomeInvalid
	}
	return wire.NotificationOutcomeUnavailable
}

func (r *notificationReceiver) Post(q wire.NotificationRequest) (wire.NotificationObservation, error) {
	scope, _, err := r.scope()
	if err != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeForbidden}, nil
	}
	// The recorded program is the caller's Rights subject, so the person's
	// block on it names the same program the policy decides (ASK-N6).
	peer, err := r.call.Peer()
	if err != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeForbidden}, nil
	}
	program, err := identity.SubjectProgram(peer, listen.Program.Path)
	if err != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeForbidden}, nil
	}
	if !asks.ValidNotificationRequest(q) {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeInvalid}, nil
	}
	if err = r.policy(); err != nil {
		return wire.NotificationObservation{Outcome: notificationFailure(err)}, nil
	}
	if q.OperationID != "" {
		if r.host.operationOwner == nil {
			return wire.NotificationObservation{Outcome: wire.NotificationOutcomeInvalid}, nil
		}
		if err = r.host.operationOwner(r.call.WaitContext(), peer, q.OperationID); err != nil {
			if errors.Is(err, ErrOperationNotOwned) {
				return wire.NotificationObservation{Outcome: wire.NotificationOutcomeInvalid}, nil
			}
			return wire.NotificationObservation{Outcome: wire.NotificationOutcomeUnavailable}, nil
		}
	}
	publisher := asks.Publisher{}
	if code, proof := peer.Code.Get(); proof > identity.ProofNone {
		publisher = asks.CodePublisher(code)
	}
	result, _, err := r.host.book.PostNotificationFrom(scope, program, q, r.call.Caller, publisher, r.policy)
	if err != nil {
		return wire.NotificationObservation{Outcome: notificationFailure(err)}, nil
	}
	return result, nil
}

func (r *notificationReceiver) Observe(ref wire.RequestRef, waitMS int64) (wire.NotificationObservation, error) {
	scope, _, err := r.scope()
	if err != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeForbidden}, nil
	}
	if waitMS < 0 || waitMS > 30000 || !asks.ValidChoiceReference(ref) {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeInvalid}, nil
	}
	result, err := r.host.book.ObserveNotification(scope, ref)
	if err != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeUnavailable}, nil
	}
	if (result.Outcome != wire.NotificationOutcomePosted && result.Outcome != wire.NotificationOutcomeScheduled) || waitMS == 0 {
		return result, nil
	}
	wait := time.Duration(waitMS) * time.Millisecond
	if record, found, e := r.host.book.NotificationRecord(result.ID); e != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeUnavailable}, nil
	} else if found {
		if until := time.Until(time.UnixMilli(record.Request.ExpiresMs)); until < wait {
			wait = max(until, 0)
		}
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-r.call.WaitContext().Done():
		return wire.NotificationObservation{}, r.call.WaitContext().Err()
	case <-timer.C:
	case <-r.host.book.Decided(result.ID):
	}
	if _, _, err := r.scope(); err != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeForbidden}, nil
	}
	result, err = r.host.book.ObserveNotification(scope, ref)
	if err != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeUnavailable}, nil
	}
	return result, nil
}

func (r *notificationReceiver) ownRecheck(scope, actor string) func(string) error {
	return func(string) error {
		if err := r.call.WaitContext().Err(); err != nil {
			return err
		}
		current, _, err := r.scope()
		if err != nil || current != scope {
			return ErrOperatorForbidden
		}
		if a, err := r.choiceActor(); err != nil || a != actor {
			return ErrOperatorForbidden
		}
		return r.call.Recheck()
	}
}

func (r *notificationReceiver) Withdraw(ref wire.RequestRef) (wire.NotificationObservation, error) {
	scope, _, err := r.scope()
	if err != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeForbidden}, nil
	}
	if !asks.ValidChoiceReference(ref) {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeInvalid}, nil
	}
	actor, err := r.choiceActor()
	if err != nil {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeForbidden}, nil
	}
	decision, err := r.host.book.WithdrawOwnNotification(scope, ref, actor, r.ownRecheck(scope, actor))
	if err != nil {
		return wire.NotificationObservation{Outcome: notificationFailure(err)}, nil
	}
	switch decision.Outcome {
	case wire.NotificationDecisionOutcomeUnknown:
		// The full reference names no admission of this caller.
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeUnknown}, nil
	case wire.NotificationDecisionOutcomeConflict:
		return r.host.book.ObserveNotification(scope, ref)
	}
	outcome, ok := wire.ParseNotificationOutcome(decision.Outcome.String())
	if !ok {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeUnavailable}, nil
	}
	result := wire.NotificationObservation{Outcome: outcome}
	if decision.Record != nil {
		result.ID, result.Result = decision.Record.ID, decision.Record.Result
	}
	return result, nil
}

func (r *notificationReceiver) Acknowledge(ref wire.RequestRef, result wire.NotificationResult) (wire.ChoiceAcknowledgement, error) {
	scope, _, err := r.scope()
	if err != nil {
		return wire.ChoiceAcknowledgement{Outcome: wire.ChoiceAckOutcomeForbidden}, nil
	}
	ack, err := r.host.book.AcknowledgeNotification(scope, ref, result, func() error {
		if err := r.call.WaitContext().Err(); err != nil {
			return err
		}
		if current, _, err := r.scope(); err != nil || current != scope {
			return ErrOperatorForbidden
		}
		return r.call.Recheck()
	})
	if err != nil {
		outcome := wire.ChoiceAckOutcomeUnavailable
		if errors.Is(err, ErrOperatorForbidden) {
			outcome = wire.ChoiceAckOutcomeForbidden
		}
		return wire.ChoiceAcknowledgement{Outcome: outcome}, nil
	}
	return ack, nil
}

type notificationOperatorReceiver struct{ operatorReceiver }

func (r *notificationOperatorReceiver) present(record wire.NotificationRecord) wire.NotificationRecord {
	record.Presentation = r.host.presentationToken(record.ID)
	return record
}

func notificationPageRefusal(outcome wire.OperatorPageOutcome) wire.NotificationPage {
	return wire.NotificationPage{Outcome: outcome, Records: []wire.NotificationRecord{}}
}

// operatorCursor binds a cursor to the caller, host epoch and revision.
func (r *notificationOperatorReceiver) cursorPrefix(kind, scope, revision string) string {
	sum := sha256.Sum256([]byte(scope))
	return kind + ":" + r.host.operatorEpoch + ":" + hex.EncodeToString(sum[:]) + ":" + revision + ":"
}

func cursorOffset(cursor, prefix string, total int) (int, bool) {
	if cursor == "" {
		return 0, true
	}
	word, ok := strings.CutPrefix(cursor, prefix)
	n, e := strconv.Atoi(word)
	if !ok || e != nil || n < 0 || n > total || strconv.Itoa(n) != word {
		return 0, false
	}
	return n, true
}

func (r *notificationOperatorReceiver) ListNotifications(cursor string, limit int64) (wire.NotificationPage, error) {
	scope, err := r.authorize()
	if err != nil {
		return notificationPageRefusal(operatorPageError(err)), nil
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return notificationPageRefusal(wire.OperatorPageOutcomeInvalid), nil
	}
	records, revision, err := r.host.book.NotificationHistory()
	if err != nil {
		return notificationPageRefusal(wire.OperatorPageOutcomeUnavailable), nil
	}
	prefix := r.cursorPrefix("notifications", scope, revision)
	offset, ok := cursorOffset(cursor, prefix, len(records))
	if !ok {
		return notificationPageRefusal(wire.OperatorPageOutcomeGap), nil
	}
	page := wire.NotificationPage{Outcome: wire.OperatorPageOutcomePage, Records: []wire.NotificationRecord{}}
	used := 2048
	for offset < len(records) && int64(len(page.Records)) < limit {
		record := r.present(records[offset])
		raw, e := json.Marshal(record)
		cost := len(raw) + 2048
		if e != nil || cost > (256<<10)-2048 {
			return notificationPageRefusal(wire.OperatorPageOutcomeUnavailable), nil
		}
		if used+cost > 256<<10 {
			break
		}
		used += cost
		page.Records = append(page.Records, record)
		offset++
	}
	page.Complete = offset == len(records)
	if !page.Complete {
		page.Next = prefix + strconv.Itoa(offset)
	}
	if _, err = r.authorize(); err != nil {
		return notificationPageRefusal(operatorPageError(err)), nil
	}
	return page, nil
}

func (r *notificationOperatorReceiver) ReadNotification(id string) (wire.NotificationRead, error) {
	if _, err := r.authorize(); err != nil {
		return wire.NotificationRead{Outcome: operatorQuestionError(err)}, nil
	}
	if !asks.ValidRequestKey(id) {
		return wire.NotificationRead{Outcome: wire.OperatorQuestionOutcomeInvalid}, nil
	}
	record, found, err := r.host.book.NotificationRecord(id)
	if err != nil {
		return wire.NotificationRead{Outcome: wire.OperatorQuestionOutcomeUnavailable}, nil
	}
	if !found {
		return wire.NotificationRead{Outcome: wire.OperatorQuestionOutcomeUnknown}, nil
	}
	if _, err = r.authorize(); err != nil {
		return wire.NotificationRead{Outcome: operatorQuestionError(err)}, nil
	}
	record = r.present(record)
	return wire.NotificationRead{Outcome: wire.OperatorQuestionOutcomeFound, Record: &record}, nil
}

func notificationDecisionError(err error) wire.NotificationDecision {
	if errors.Is(err, ErrOperatorForbidden) || errors.Is(err, asks.ErrNotificationSuppressed) {
		return wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeForbidden}
	}
	return wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeUnavailable}
}

func (r *notificationOperatorReceiver) decide(id string, kind asks.NotificationKind, action, presentation string) (wire.NotificationDecision, error) {
	if _, err := r.authorize(); err != nil {
		return notificationDecisionError(err), nil
	}
	if !asks.ValidRequestKey(id) {
		return wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeInvalid}, nil
	}
	if kind == asks.NotificationAct && !hmac.Equal([]byte(presentation), []byte(r.host.presentationToken(id))) {
		return wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeStale}, nil
	}
	actor, err := r.choiceActor()
	if err != nil {
		return notificationDecisionError(err), nil
	}
	result, err := r.host.book.DecideNotification(id, kind, action, actor, func(program string) error {
		if _, err := r.authorize(); err != nil {
			return err
		}
		if kind == asks.NotificationAct {
			if err := r.host.notificationBlocked(r.ctx, program); err != nil {
				return err
			}
		}
		return r.call.Recheck()
	})
	if err != nil {
		return notificationDecisionError(err), nil
	}
	if result.Record != nil {
		record := r.present(*result.Record)
		result.Record = &record
	}
	return result, nil
}

func (r *notificationOperatorReceiver) Act(id, action, presentation string) (wire.NotificationDecision, error) {
	return r.decide(id, asks.NotificationAct, action, presentation)
}

func (r *notificationOperatorReceiver) Dismiss(id string) (wire.NotificationDecision, error) {
	return r.decide(id, asks.NotificationDismiss, "", "")
}

func (r *notificationOperatorReceiver) WithdrawProgram(program string) (wire.NotificationDecision, error) {
	if _, err := r.authorize(); err != nil {
		return notificationDecisionError(err), nil
	}
	actor, err := r.choiceActor()
	if err != nil {
		return notificationDecisionError(err), nil
	}
	count, _, err := r.host.book.WithdrawProgramNotifications(program, actor, func() error {
		if _, err := r.authorize(); err != nil {
			return err
		}
		return r.call.Recheck()
	})
	if errors.Is(err, asks.ErrNoRecord) {
		return wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeInvalid}, nil
	}
	if err != nil {
		return notificationDecisionError(err), nil
	}
	return wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeWithdrawn, Count: count}, nil
}

func (r *notificationOperatorReceiver) ListNotifiers(cursor string, limit int64) (wire.NotifierPage, error) {
	refused := func(outcome wire.OperatorPageOutcome) wire.NotifierPage {
		return wire.NotifierPage{Outcome: outcome, Notifiers: []wire.NotifierCount{}}
	}
	scope, err := r.authorize()
	if err != nil {
		return refused(operatorPageError(err)), nil
	}
	if len(cursor) > 256 || !utf8.ValidString(cursor) || limit < 1 || limit > 64 {
		return refused(wire.OperatorPageOutcomeInvalid), nil
	}
	rows, revision, err := r.host.book.NotifierCounts()
	if err != nil {
		return refused(wire.OperatorPageOutcomeUnavailable), nil
	}
	prefix := r.cursorPrefix("notifiers", scope, revision)
	offset, ok := cursorOffset(cursor, prefix, len(rows))
	if !ok {
		return refused(wire.OperatorPageOutcomeGap), nil
	}
	page := wire.NotifierPage{Outcome: wire.OperatorPageOutcomePage, Notifiers: []wire.NotifierCount{}}
	for offset < len(rows) && int64(len(page.Notifiers)) < limit {
		page.Notifiers = append(page.Notifiers, rows[offset])
		offset++
	}
	page.Complete = offset == len(rows)
	if !page.Complete {
		page.Next = prefix + strconv.Itoa(offset)
	}
	if _, err = r.authorize(); err != nil {
		return refused(operatorPageError(err)), nil
	}
	return page, nil
}
