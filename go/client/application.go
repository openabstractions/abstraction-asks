// Package client binds the caller-scoped application question service.
package client

import (
	"context"
	"errors"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
	"time"
)

type ApplicationQuestion = wire.ApplicationQuestion
type QuestionObservation = wire.QuestionObservation
type Client struct{ transport listen.FrameClient }

func New(endpoint string) *Client {
	return NewWithTransport(listen.FrameClient{Endpoint: endpoint})
}

// NewWithTransport retains the caller's endpoint, server trust and waiting limits.
func NewWithTransport(transport listen.FrameClient) *Client {
	return &Client{transport: transport.WithDefaults(5*time.Second, 1<<20)}
}

// WithTimeout explicitly chooses a fresh per-call budget; the original remains reusable.
func (c *Client) WithTimeout(timeout time.Duration) (*Client, error) {
	if timeout <= 0 || timeout > 35*time.Second {
		return nil, errors.New("asks: waiting budget must be positive and at most 35s")
	}
	copy := *c
	copy.transport.Timeout = timeout
	return &copy, nil
}
func (c *Client) Ask(q ApplicationQuestion) (QuestionObservation, error) {
	return c.AskContext(context.Background(), q)
}

// AskContext never retries. After a transport error, retry the same request key
// and content; the receiver may already have admitted the question.
func (c *Client) AskContext(ctx context.Context, q ApplicationQuestion) (QuestionObservation, error) {
	if e := ctx.Err(); e != nil {
		return QuestionObservation{}, e
	}
	result, e := wire.NewQuestionApplicationClient(c.transport.WithContext(ctx)).Ask(q)
	return checked(result, e)
}
func (c *Client) ObserveContext(ctx context.Context, key string, waitMS int64) (QuestionObservation, error) {
	if e := ctx.Err(); e != nil {
		return QuestionObservation{}, e
	}
	if waitMS < 0 || waitMS > 30000 {
		return QuestionObservation{}, errors.New("asks: wait_ms must be 0..30000")
	}
	result, e := wire.NewQuestionApplicationClient(c.transport.WithContext(ctx)).Observe(key, waitMS)
	return checked(result, e)
}
func checked(result QuestionObservation, e error) (QuestionObservation, error) {
	if e != nil {
		return QuestionObservation{}, e
	}
	pending := result.Outcome == wire.ObservationOutcomePending
	answered := result.Outcome == wire.ObservationOutcomeAnswered
	if (pending || answered) != (result.Answer != nil) {
		return QuestionObservation{}, errors.New("asks: inconsistent observation")
	}
	if a := result.Answer; a != nil {
		if a.Id == "" || a.Pending != pending || (pending && (a.Option != "" || a.Yes || a.Kept)) || (answered && a.Option == "") {
			return QuestionObservation{}, errors.New("asks: inconsistent answer")
		}
	}
	return result, nil
}
