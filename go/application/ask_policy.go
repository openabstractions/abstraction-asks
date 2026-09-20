package application

import (
	"context"
	"errors"

	identity "github.com/openabstractions/abstraction-identity"
)

// AskPolicy is trusted host configuration that decides, for every application
// Ask after receiving same-account Program proof, whether the bound caller may
// admit a question with this catalogue key. It may consult rights, for example
// the rule abstraction.asks/question.ask on account (ASK-R1). Return
// ErrAskPolicyUnavailable when no decision was obtained; any other error is an
// evaluated refusal. Observe of the caller's own questions is not decided here.
type AskPolicy func(ctx context.Context, peer *identity.Peer, key string) error

// ErrAskPolicyUnavailable marks an AskPolicy error meaning the decision could
// not be obtained. Ask then returns unavailable and admits nothing.
var ErrAskPolicyUnavailable = errors.New("asks: ask policy decision unavailable")

// EnableAskPolicy narrows application Ask to callers the policy admits. Without
// one, Ask keeps same-account Program proof. Configure it before Serve.
func (h *Host) EnableAskPolicy(policy AskPolicy) error {
	h.lifecycle.Lock()
	defer h.lifecycle.Unlock()
	if h.serving || h.ctx.Err() != nil {
		return errors.New("asks: configure the ask policy before Serve")
	}
	if policy == nil {
		return errors.New("asks: explicit ask policy required")
	}
	h.askPolicy = policy
	return nil
}
