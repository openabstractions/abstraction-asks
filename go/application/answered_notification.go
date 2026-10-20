package application

import (
	"errors"
	"os"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/listen"
)

// The service owns the announcement independently of the assistant host. The
// immutable answer occurrence is also its Q2 replay key, including after a lost
// reply. Replaying an answer never creates a second notification.
func (r *responderReceiver) notifyAnswered(id string, answeredMs int64, choice bool) {
	if r.host.notificationPolicy == nil || answeredMs <= 0 {
		return
	}
	program, err := os.Executable()
	if err != nil {
		r.reportNotificationError(err)
		return
	}
	program = identity.CanonicalProgramPath(program)
	principal, err := identity.Self()
	if err != nil {
		r.reportNotificationError(err)
		return
	}
	body := "The assistant answered a question for you. Review it in the OpenAbstractions Panel, Questions."
	if choice {
		body = "The assistant chose an option for an app. Review it in the OpenAbstractions Panel, Activity."
	}
	until := answeredMs + asks.MaxChoiceReplayMilliseconds
	request := wire.NotificationRequest{
		RequestRef: wire.RequestRef{Key: "answered-" + id, ReplayUntilMs: until},
		Title:      "Answered for you", Body: body, Delivery: wire.NotificationDeliveryNow,
		ExpiresMs: until, Actions: []wire.NotificationAction{},
	}
	// Enter the existing native notification receiver as this service program.
	// Its posting policy is checked normally; the block-only operator callback
	// cannot supply posting authority. Server trust comes from this process.
	transport := listen.FrameClient{Endpoint: r.host.endpoint, MaxFrame: MaxFrameBytes,
		Server: &listen.ServerExpectation{Principal: principal, Program: program}}
	result, err := wire.NewNotificationApplicationClient(transport.WithContext(r.host.ctx)).Post(request)
	if err == nil && result.Outcome != wire.NotificationOutcomePosted && result.Outcome != wire.NotificationOutcomeActed && result.Outcome != wire.NotificationOutcomeDismissed && result.Outcome != wire.NotificationOutcomeWithdrawn && result.Outcome != wire.NotificationOutcomeExpired {
		err = errors.New("asks: answered notification was not posted")
	}
	r.reportNotificationError(err)
}

func (r *responderReceiver) reportNotificationError(err error) {
	if err != nil && r.host.OnError != nil {
		r.host.OnError(err)
	}
}
