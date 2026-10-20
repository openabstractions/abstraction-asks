package client

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

type NotificationAction = wire.NotificationAction
type NotificationActivation = wire.NotificationActivation
type NotificationDelivery = wire.NotificationDelivery
type NotificationRequest = wire.NotificationRequest
type NotificationState = wire.NotificationState
type NotificationResult = wire.NotificationResult
type NotificationOutcome = wire.NotificationOutcome
type NotificationObservation = wire.NotificationObservation
type NotificationRecord = wire.NotificationRecord
type NotificationPage = wire.NotificationPage
type NotificationRead = wire.NotificationRead
type NotificationDecision = wire.NotificationDecision
type NotificationDecisionOutcome = wire.NotificationDecisionOutcome
type NotifierCount = wire.NotifierCount
type NotifierDay = wire.NotifierDay
type NotifierPage = wire.NotifierPage

const (
	NotificationActivationApp          = wire.NotificationActivationApp
	NotificationActivationOpenActivity = wire.NotificationActivationOpenActivity
	NotificationDeliveryNow            = wire.NotificationDeliveryNow
	NotificationDeliveryOperationEnd   = wire.NotificationDeliveryOperationEnd
	NotificationStateScheduled         = wire.NotificationStateScheduled
	NotificationStatePosted            = wire.NotificationStatePosted
	NotificationStateActed             = wire.NotificationStateActed
	NotificationStateDismissed         = wire.NotificationStateDismissed
	NotificationStateWithdrawn         = wire.NotificationStateWithdrawn
	NotificationStateExpired           = wire.NotificationStateExpired
	NotificationOutcomeScheduled       = wire.NotificationOutcomeScheduled
	NotificationOutcomePosted          = wire.NotificationOutcomePosted
	NotificationOutcomeActed           = wire.NotificationOutcomeActed
	NotificationOutcomeDismissed       = wire.NotificationOutcomeDismissed
	NotificationOutcomeWithdrawn       = wire.NotificationOutcomeWithdrawn
	NotificationOutcomeExpired         = wire.NotificationOutcomeExpired
	NotificationOutcomeGone            = wire.NotificationOutcomeGone
	NotificationOutcomeUnknown         = wire.NotificationOutcomeUnknown
	NotificationOutcomeConflict        = wire.NotificationOutcomeConflict
	NotificationOutcomeInvalid         = wire.NotificationOutcomeInvalid
	NotificationOutcomeSuppressed      = wire.NotificationOutcomeSuppressed
	NotificationOutcomeExhausted       = wire.NotificationOutcomeExhausted
	NotificationOutcomeForbidden       = wire.NotificationOutcomeForbidden
	NotificationOutcomeUnavailable     = wire.NotificationOutcomeUnavailable
	NotificationDecisionActed          = wire.NotificationDecisionOutcomeActed
	NotificationDecisionDismissed      = wire.NotificationDecisionOutcomeDismissed
	NotificationDecisionWithdrawn      = wire.NotificationDecisionOutcomeWithdrawn
	NotificationDecisionExpired        = wire.NotificationDecisionOutcomeExpired
	NotificationDecisionGone           = wire.NotificationDecisionOutcomeGone
	NotificationDecisionUnknown        = wire.NotificationDecisionOutcomeUnknown
	NotificationDecisionConflict       = wire.NotificationDecisionOutcomeConflict
	NotificationDecisionStale          = wire.NotificationDecisionOutcomeStale
	NotificationDecisionInvalid        = wire.NotificationDecisionOutcomeInvalid
	NotificationDecisionForbidden      = wire.NotificationDecisionOutcomeForbidden
	NotificationDecisionUnavailable    = wire.NotificationDecisionOutcomeUnavailable
)

func NotificationActivationValues() []NotificationActivation {
	return wire.NotificationActivationValues()
}
func NotificationDeliveryValues() []NotificationDelivery { return wire.NotificationDeliveryValues() }
func NotificationStateValues() []NotificationState       { return wire.NotificationStateValues() }
func NotificationOutcomeValues() []NotificationOutcome   { return wire.NotificationOutcomeValues() }
func NotificationDecisionOutcomeValues() []NotificationDecisionOutcome {
	return wire.NotificationDecisionOutcomeValues()
}

// NotificationClient posts and observes this caller's notifications
// (abstraction.asks/notification@1). Retain each full RequestRef unchanged
// across retries and restarts; no method refreshes a reference or retries.
type NotificationClient struct{ transport listen.FrameClient }

func NewNotification(endpoint string) *NotificationClient {
	return NewNotificationWithTransport(listen.FrameClient{Endpoint: endpoint})
}
func NewNotificationWithTransport(transport listen.FrameClient) *NotificationClient {
	return &NotificationClient{transport: transport.WithDefaults(5*time.Second, 1<<20)}
}

// WithTimeout returns a copy with a total call budget of at most 35 seconds,
// for an Observe that waits.
func (c *NotificationClient) WithTimeout(timeout time.Duration) (*NotificationClient, error) {
	if timeout <= 0 || timeout > 35*time.Second {
		return nil, errors.New("asks: waiting budget must be positive and at most 35s")
	}
	copy := *c
	copy.transport.Timeout = timeout
	return &copy, nil
}

func (c *NotificationClient) PostContext(ctx context.Context, q NotificationRequest) (NotificationObservation, error) {
	if err := ctx.Err(); err != nil {
		return NotificationObservation{}, err
	}
	if !asks.ValidNotificationRequest(q) {
		return NotificationObservation{}, errors.New("asks: invalid notification")
	}
	result, err := checkedNotification(wire.NewNotificationApplicationClient(c.transport.WithContext(ctx)).Post(q))
	if err == nil && result.Result != nil && result.Result.State == NotificationStateActed && !offered(q, result.Result.Action) {
		return NotificationObservation{}, errors.New("asks: unoffered notification action")
	}
	return result, err
}
func offered(q NotificationRequest, id string) bool {
	for _, a := range q.Actions {
		if a.ID == id {
			return true
		}
	}
	return false
}
func (c *NotificationClient) ObserveContext(ctx context.Context, ref RequestRef, waitMS int64) (NotificationObservation, error) {
	if err := ctx.Err(); err != nil {
		return NotificationObservation{}, err
	}
	if !asks.ValidChoiceReference(ref) || waitMS < 0 || waitMS > 30000 {
		return NotificationObservation{}, errors.New("asks: invalid notification observation")
	}
	return checkedNotification(wire.NewNotificationApplicationClient(c.transport.WithContext(ctx)).Observe(ref, waitMS))
}
func (c *NotificationClient) WithdrawContext(ctx context.Context, ref RequestRef) (NotificationObservation, error) {
	if err := ctx.Err(); err != nil {
		return NotificationObservation{}, err
	}
	if !asks.ValidChoiceReference(ref) {
		return NotificationObservation{}, errors.New("asks: invalid notification withdrawal")
	}
	return checkedNotification(wire.NewNotificationApplicationClient(c.transport.WithContext(ctx)).Withdraw(ref))
}
func (c *NotificationClient) AcknowledgeContext(ctx context.Context, ref RequestRef, result NotificationResult) (ChoiceAcknowledgement, error) {
	if err := ctx.Err(); err != nil {
		return ChoiceAcknowledgement{}, err
	}
	if !asks.ValidChoiceReference(ref) || !asks.ValidNotificationResult(result) {
		return ChoiceAcknowledgement{}, errors.New("asks: invalid notification acknowledgement")
	}
	return wire.NewNotificationApplicationClient(c.transport.WithContext(ctx)).Acknowledge(ref, result)
}

func checkedNotification(result NotificationObservation, err error) (NotificationObservation, error) {
	if err != nil {
		return NotificationObservation{}, err
	}
	switch result.Outcome {
	case NotificationOutcomeScheduled, NotificationOutcomePosted:
		if result.ID == "" || result.Result != nil || !asks.ValidRequestKey(result.ID) {
			return NotificationObservation{}, errors.New("asks: inconsistent notification observation")
		}
	case NotificationOutcomeActed, NotificationOutcomeDismissed, NotificationOutcomeWithdrawn:
		r := result.Result
		if result.ID == "" || r == nil || !asks.ValidNotificationResult(*r) || r.ID != result.ID || r.State.String() != result.Outcome.String() {
			return NotificationObservation{}, errors.New("asks: inconsistent notification result")
		}
	default:
		if result.ID != "" || result.Result != nil {
			return NotificationObservation{}, errors.New("asks: inconsistent notification observation")
		}
	}
	return result, nil
}

// NotificationOperator lists, acts on and audits notifications for an
// explicitly authorized operator (abstraction.asks/notification-operator@1).
type NotificationOperator struct{ transport listen.FrameClient }

func NewNotificationOperator(endpoint string) *NotificationOperator {
	return NewNotificationOperatorWithTransport(listen.FrameClient{Endpoint: endpoint})
}
func NewNotificationOperatorWithTransport(transport listen.FrameClient) *NotificationOperator {
	return &NotificationOperator{transport: transport.WithDefaults(5*time.Second, 1<<20)}
}

func validRange(cursor string, limit int64) bool {
	return len(cursor) <= 256 && utf8.ValidString(cursor) && limit >= 1 && limit <= 64
}

func (c *NotificationOperator) ListNotificationsContext(ctx context.Context, cursor string, limit int64) (NotificationPage, error) {
	if err := ctx.Err(); err != nil {
		return NotificationPage{}, err
	}
	if !validRange(cursor, limit) {
		return NotificationPage{}, errors.New("asks: invalid notification range")
	}
	page, err := wire.NewNotificationOperatorClient(c.transport.WithContext(ctx)).ListNotifications(cursor, limit)
	if err != nil {
		return NotificationPage{}, err
	}
	if page.Outcome != wire.OperatorPageOutcomePage {
		if len(page.Records) != 0 || page.Next != "" || page.Complete {
			return NotificationPage{}, errors.New("asks: malformed notification refusal")
		}
		return page, nil
	}
	if int64(len(page.Records)) > limit || len(page.Next) > 256 || page.Complete != (page.Next == "") || !page.Complete && len(page.Records) == 0 {
		return NotificationPage{}, errors.New("asks: malformed notification page")
	}
	ids := map[string]bool{}
	for _, r := range page.Records {
		if ids[r.ID] || r.Presentation == "" || !validServedNotification(r) {
			return NotificationPage{}, errors.New("asks: malformed notification record")
		}
		ids[r.ID] = true
	}
	return page, nil
}

func validServedNotification(r NotificationRecord) bool {
	stored := r
	stored.Presentation = ""
	return asks.ValidNotificationRecord(stored) && len(r.Presentation) <= 64
}

// ListAllNotificationsContext reads every page, restarting once on a gap.
func (c *NotificationOperator) ListAllNotificationsContext(ctx context.Context) ([]NotificationRecord, error) {
	for attempt := 0; attempt < 2; attempt++ {
		var out []NotificationRecord
		cursor := ""
		for page := 0; page < 64; page++ {
			result, err := c.ListNotificationsContext(ctx, cursor, 64)
			if err != nil {
				return nil, err
			}
			if result.Outcome == wire.OperatorPageOutcomeGap {
				break
			}
			if result.Outcome != wire.OperatorPageOutcomePage {
				return nil, errors.New("asks: notifications " + result.Outcome.String())
			}
			out = append(out, result.Records...)
			if result.Complete {
				return out, nil
			}
			cursor = result.Next
		}
	}
	return nil, errors.New("asks: notification list kept changing")
}

func (c *NotificationOperator) ReadNotificationContext(ctx context.Context, id string) (NotificationRead, error) {
	if err := ctx.Err(); err != nil {
		return NotificationRead{}, err
	}
	read, err := wire.NewNotificationOperatorClient(c.transport.WithContext(ctx)).ReadNotification(id)
	if err != nil {
		return NotificationRead{}, err
	}
	if (read.Outcome == wire.OperatorQuestionOutcomeFound) != (read.Record != nil) || read.Record != nil && (read.Record.ID != id || !validServedNotification(*read.Record)) {
		return NotificationRead{}, errors.New("asks: malformed notification read")
	}
	return read, nil
}

func checkedNotificationDecision(result NotificationDecision, err error, id string) (NotificationDecision, error) {
	if err != nil {
		return NotificationDecision{}, err
	}
	carries := result.Outcome == NotificationDecisionActed || result.Outcome == NotificationDecisionDismissed || result.Outcome == NotificationDecisionWithdrawn
	if id != "" && carries != (result.Record != nil) || result.Record != nil && (id != "" && result.Record.ID != id || !validServedNotification(*result.Record)) {
		return NotificationDecision{}, errors.New("asks: malformed notification decision")
	}
	return result, nil
}

// ActContext records the person's chosen action with the presentation token
// the shown record carried. Stale means the item belongs to another showing.
func (c *NotificationOperator) ActContext(ctx context.Context, id, action, presentation string) (NotificationDecision, error) {
	if err := ctx.Err(); err != nil {
		return NotificationDecision{}, err
	}
	result, err := wire.NewNotificationOperatorClient(c.transport.WithContext(ctx)).Act(id, action, presentation)
	return checkedNotificationDecision(result, err, id)
}
func (c *NotificationOperator) DismissContext(ctx context.Context, id string) (NotificationDecision, error) {
	if err := ctx.Err(); err != nil {
		return NotificationDecision{}, err
	}
	result, err := wire.NewNotificationOperatorClient(c.transport.WithContext(ctx)).Dismiss(id)
	return checkedNotificationDecision(result, err, id)
}
func (c *NotificationOperator) WithdrawProgramContext(ctx context.Context, program string) (NotificationDecision, error) {
	if err := ctx.Err(); err != nil {
		return NotificationDecision{}, err
	}
	result, err := wire.NewNotificationOperatorClient(c.transport.WithContext(ctx)).WithdrawProgram(program)
	if err != nil {
		return NotificationDecision{}, err
	}
	if result.Record != nil || result.Count < 0 {
		return NotificationDecision{}, errors.New("asks: malformed notification withdrawal")
	}
	return result, nil
}
func (c *NotificationOperator) ListNotifiersContext(ctx context.Context, cursor string, limit int64) (NotifierPage, error) {
	if err := ctx.Err(); err != nil {
		return NotifierPage{}, err
	}
	if !validRange(cursor, limit) {
		return NotifierPage{}, errors.New("asks: invalid notifier range")
	}
	page, err := wire.NewNotificationOperatorClient(c.transport.WithContext(ctx)).ListNotifiers(cursor, limit)
	if err != nil {
		return NotifierPage{}, err
	}
	if page.Outcome != wire.OperatorPageOutcomePage && (len(page.Notifiers) != 0 || page.Next != "" || page.Complete) || int64(len(page.Notifiers)) > limit {
		return NotifierPage{}, errors.New("asks: malformed notifier page")
	}
	return page, nil
}
