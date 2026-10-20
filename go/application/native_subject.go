package application

import (
	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

func (r *receiver) nativeBook() (*asks.NativeApplicationBook, error) {
	peer, err := r.call.Peer()
	if err != nil {
		return nil, err
	}
	return r.nativeBookForPeer(peer)
}

func (r *receiver) nativeBookForPeer(peer *identity.Peer) (*asks.NativeApplicationBook, error) {
	_, path, err := callerScope(peer, r.host.owner)
	if err != nil {
		return nil, err
	}
	subject, verdict := identity.AdmitOwner(peer, identity.Need{User: listen.Program.User, Path: listen.Program.Path}, r.host.owner)
	if verdict == identity.OwnerOtherAccount {
		return nil, identity.ErrNoAccount
	}
	if verdict != identity.OwnerAdmitted {
		return nil, identity.ErrNotProven
	}
	return r.host.book.BindNativeSubject(subject, path)
}

func (r *receiver) nativeBookAskApplicationAs(_ string, program, requestKey, key string, slots map[string]string, via listen.Seen, publisher asks.Publisher) (asks.Record, string, error) {
	b, err := r.nativeBook()
	if err != nil {
		return asks.Record{}, "", err
	}
	return b.AskApplicationAs(program, requestKey, key, slots, via, publisher)
}

func (r *receiver) nativeBookObserveApplication(_ string, key string) (asks.Record, string, error) {
	b, err := r.nativeBook()
	if err != nil {
		return asks.Record{}, "", err
	}
	return b.ObserveApplication(key)
}

func (r *receiver) nativeBookAskChoiceAs(_ string, program string, q wire.ChoiceRequest, shared []string, via listen.Seen, publisher asks.Publisher, authorize func() error) (wire.ChoiceObservation, error) {
	b, err := r.nativeBook()
	if err != nil {
		return wire.ChoiceObservation{}, err
	}
	return b.AskChoiceAs(program, q, shared, via, publisher, authorize)
}

func (r *receiver) nativeBookObserveChoice(_ string, ref wire.RequestRef) (wire.ChoiceObservation, error) {
	b, err := r.nativeBook()
	if err != nil {
		return wire.ChoiceObservation{}, err
	}
	return b.ObserveChoice(ref)
}

func (r *receiver) nativeBookAcknowledgeChoice(_ string, ref wire.RequestRef, answer wire.ChoiceAnswer, authorize func() error) (wire.ChoiceAcknowledgement, error) {
	b, err := r.nativeBook()
	if err != nil {
		return wire.ChoiceAcknowledgement{}, err
	}
	return b.AcknowledgeChoice(ref, answer, authorize)
}

func (r *receiver) nativeBookDecideOwnChoiceAuthorized(_ string, ref wire.RequestRef, option, actor string, dismiss, ifPending bool, authorize func() error) (wire.ChoiceDecision, error) {
	b, err := r.nativeBook()
	if err != nil {
		return wire.ChoiceDecision{}, err
	}
	return b.DecideOwnChoiceAuthorized(ref, option, actor, dismiss, ifPending, authorize)
}

func (r *receiver) nativeBookPostNotificationFrom(_ string, program string, q wire.NotificationRequest, via listen.Seen, publisher asks.Publisher, authorize func() error) (wire.NotificationObservation, []string, error) {
	b, err := r.nativeBook()
	if err != nil {
		return wire.NotificationObservation{}, nil, err
	}
	return b.PostNotificationFrom(program, q, via, publisher, authorize)
}

func (r *receiver) nativeBookObserveNotification(_ string, ref wire.RequestRef) (wire.NotificationObservation, error) {
	b, err := r.nativeBook()
	if err != nil {
		return wire.NotificationObservation{}, err
	}
	return b.ObserveNotification(ref)
}

func (r *receiver) nativeBookWithdrawOwnNotification(_ string, ref wire.RequestRef, actor string, authorize func(program string) error) (wire.NotificationDecision, error) {
	b, err := r.nativeBook()
	if err != nil {
		return wire.NotificationDecision{}, err
	}
	return b.WithdrawOwnNotification(ref, actor, authorize)
}

func (r *receiver) nativeBookAcknowledgeNotification(_ string, ref wire.RequestRef, result wire.NotificationResult, authorize func() error) (wire.ChoiceAcknowledgement, error) {
	b, err := r.nativeBook()
	if err != nil {
		return wire.ChoiceAcknowledgement{}, err
	}
	return b.AcknowledgeNotification(ref, result, authorize)
}
