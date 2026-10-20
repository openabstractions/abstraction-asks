package client

import (
	"context"
	"github.com/openabstractions/abstraction-asks/go/typedpermission"
)

type DetailsReader = typedpermission.DetailsReader
type AskerReader = DetailsReader

// ReadAskerContext preserves ASK-O11's explicitly selected old-service reader.
// Deprecated: use typedpermission.ReadAskerContext at the authority presenter.
func ReadAskerContext(ctx context.Context, r DetailsReader, account, id string) (QuestionAsker, error) {
	return typedpermission.ReadAskerContext(ctx, r, account, id, nativePermissionValidators())
}

// Deprecated: use typedpermission.ReadDetailsContext at the authority presenter.
func ReadDetailsContext(ctx context.Context, r DetailsReader, account, id string) (QuestionDetails, error) {
	return typedpermission.ReadDetailsContext(ctx, r, account, id, nativePermissionValidators())
}
func validatePermissionDetails(v QuestionDetails, account, id string) error {
	return typedpermission.CheckPermissionDetails(v, account, id, nativePermissionValidators())
}
