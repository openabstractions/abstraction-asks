package asks

import (
	"errors"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

func (b *Book) ReleaseAnsweredApplicationID(scope, key, expectedID, option string) (bool, error) {
	return b.nativeReleaseAnsweredApplicationID(nil, scope, key, expectedID, option)
}

func (v *NativeApplicationBook) ReleaseAnsweredApplicationID(key, expectedID, option string) (bool, error) {
	return v.book.nativeReleaseAnsweredApplicationID(v.owner, v.owner.scope(), key, expectedID, option)
}

func (b *Book) ObserveApplication(scope, key string) (Record, string, error) {
	return b.nativeObserveApplication(nil, scope, key)
}

func (v *NativeApplicationBook) ObserveApplication(key string) (Record, string, error) {
	r, o, e := v.book.nativeObserveApplication(v.owner, v.owner.scope(), key)
	if errors.Is(e, errNativeApplicationCollision) {
		return Record{}, "conflict", nil
	}
	return r, o, e
}

func (b *Book) ObserveChoice(scope string, ref wire.RequestRef) (wire.ChoiceObservation, error) {
	return b.nativeObserveChoice(nil, scope, ref)
}

func (v *NativeApplicationBook) ObserveChoice(ref wire.RequestRef) (wire.ChoiceObservation, error) {
	result, err := v.book.nativeObserveChoice(v.owner, v.owner.scope(), ref)
	if errors.Is(err, errNativeApplicationCollision) {
		return wire.ChoiceObservation{Outcome: wire.ChoiceOutcomeConflict}, nil
	}
	return result, err
}

func (b *Book) AcknowledgeChoice(scope string, ref wire.RequestRef, answer wire.ChoiceAnswer, authorize func() error) (wire.ChoiceAcknowledgement, error) {
	return b.nativeAcknowledgeChoice(nil, scope, ref, answer, authorize)
}

func (v *NativeApplicationBook) AcknowledgeChoice(ref wire.RequestRef, answer wire.ChoiceAnswer, authorize func() error) (wire.ChoiceAcknowledgement, error) {
	result, err := v.book.nativeAcknowledgeChoice(v.owner, v.owner.scope(), ref, answer, authorize)
	if errors.Is(err, errNativeApplicationCollision) {
		return wire.ChoiceAcknowledgement{Outcome: wire.ChoiceAckOutcomeConflict}, nil
	}
	return result, err
}

func (b *Book) DecideOwnChoiceAuthorized(scope string, ref wire.RequestRef, option, actor string, dismiss, ifPending bool, authorize func() error) (wire.ChoiceDecision, error) {
	return b.nativeDecideOwnChoiceAuthorized(nil, scope, ref, option, actor, dismiss, ifPending, authorize)
}

func (v *NativeApplicationBook) DecideOwnChoiceAuthorized(ref wire.RequestRef, option, actor string, dismiss, ifPending bool, authorize func() error) (wire.ChoiceDecision, error) {
	result, err := v.book.nativeDecideOwnChoiceAuthorized(v.owner, v.owner.scope(), ref, option, actor, dismiss, ifPending, authorize)
	if errors.Is(err, errNativeApplicationCollision) {
		return wire.ChoiceDecision{Outcome: wire.ChoiceDecisionOutcomeConflict}, nil
	}
	return result, err
}

func (b *Book) PostNotificationFrom(scope, program string, q wire.NotificationRequest, via listen.Seen, publisher Publisher, authorize func() error) (wire.NotificationObservation, []string, error) {
	return b.nativePostNotificationFrom(nil, scope, program, q, via, publisher, authorize)
}

func (v *NativeApplicationBook) PostNotificationFrom(program string, q wire.NotificationRequest, via listen.Seen, publisher Publisher, authorize func() error) (wire.NotificationObservation, []string, error) {
	result, replaced, err := v.book.nativePostNotificationFrom(v.owner, v.owner.scope(), program, q, via, publisher, authorize)
	if errors.Is(err, errNativeApplicationCollision) {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeConflict}, nil, nil
	}
	return result, replaced, err
}

func (b *Book) ObserveNotification(scope string, ref wire.RequestRef) (wire.NotificationObservation, error) {
	return b.nativeObserveNotification(nil, scope, ref)
}

func (v *NativeApplicationBook) ObserveNotification(ref wire.RequestRef) (wire.NotificationObservation, error) {
	result, err := v.book.nativeObserveNotification(v.owner, v.owner.scope(), ref)
	if errors.Is(err, errNativeApplicationCollision) {
		return wire.NotificationObservation{Outcome: wire.NotificationOutcomeConflict}, nil
	}
	return result, err
}

func (b *Book) WithdrawOwnNotification(scope string, ref wire.RequestRef, actor string, authorize func(program string) error) (wire.NotificationDecision, error) {
	return b.nativeWithdrawOwnNotification(nil, scope, ref, actor, authorize)
}

func (v *NativeApplicationBook) WithdrawOwnNotification(ref wire.RequestRef, actor string, authorize func(program string) error) (wire.NotificationDecision, error) {
	result, err := v.book.nativeWithdrawOwnNotification(v.owner, v.owner.scope(), ref, actor, authorize)
	if errors.Is(err, errNativeApplicationCollision) {
		return wire.NotificationDecision{Outcome: wire.NotificationDecisionOutcomeConflict}, nil
	}
	return result, err
}

func (b *Book) AcknowledgeNotification(scope string, ref wire.RequestRef, result wire.NotificationResult, authorize func() error) (wire.ChoiceAcknowledgement, error) {
	return b.nativeAcknowledgeNotification(nil, scope, ref, result, authorize)
}

func (v *NativeApplicationBook) AcknowledgeNotification(ref wire.RequestRef, result wire.NotificationResult, authorize func() error) (wire.ChoiceAcknowledgement, error) {
	ack, err := v.book.nativeAcknowledgeNotification(v.owner, v.owner.scope(), ref, result, authorize)
	if errors.Is(err, errNativeApplicationCollision) {
		return wire.ChoiceAcknowledgement{Outcome: wire.ChoiceAckOutcomeConflict}, nil
	}
	return ack, err
}
