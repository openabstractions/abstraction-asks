package client

import (
	"context"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/presenter"
)

// PresenterContext uses the same trusted local transport as the operator.
func (c *Operator) PresenterContext(ctx context.Context) *wire.PresenterClient {
	return wire.NewPresenterClient(c.transport.WithContext(ctx))
}

// PermissionOperator binds typed permission reads to this same local runtime.
func (c *Operator) PermissionOperator() *PermissionOperator {
	return NewPermissionOperatorWithTransport(c.transport)
}
