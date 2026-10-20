package client

import (
	"context"
	"errors"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/responderwait"
	"github.com/openabstractions/abstraction-identity/listen"
)

const ResponderWaitContract = "abstraction.asks/responder-wait@1"

type Outcome = wire.Outcome
type WaitingPage = wire.WaitingPage

const (
	OutcomePage        = wire.OutcomePage
	OutcomeGap         = wire.OutcomeGap
	OutcomeInvalid     = wire.OutcomeInvalid
	OutcomeForbidden   = wire.OutcomeForbidden
	OutcomeUnavailable = wire.OutcomeUnavailable
)

func OutcomeValues() []Outcome                 { return wire.OutcomeValues() }
func ParseOutcome(word string) (Outcome, bool) { return wire.ParseOutcome(word) }

type ResponderWait struct{ transport listen.FrameClient }

func NewResponderWaitWithTransport(transport listen.FrameClient) *ResponderWait {
	return &ResponderWait{transport: transport.WithDefaults(5*time.Second, 1<<20)}
}

func (c *ResponderWait) ListWaitingContext(ctx context.Context, cursor string, limit int64) (WaitingPage, error) {
	if err := ctx.Err(); err != nil {
		return WaitingPage{}, err
	}
	page, err := wire.NewResponderWaitClient(c.transport.WithContext(ctx)).ListWaiting(cursor, limit)
	if err != nil {
		return WaitingPage{}, err
	}
	if page.Outcome != OutcomePage {
		if len(page.IDs) != 0 || page.Next != "" || page.Complete {
			return WaitingPage{}, errors.New("asks: malformed waiting refusal")
		}
		return page, nil
	}
	if int64(len(page.IDs)) > limit || page.Complete != (page.Next == "") {
		return WaitingPage{}, errors.New("asks: malformed waiting page")
	}
	return page, nil
}
