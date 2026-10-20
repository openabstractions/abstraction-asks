package client

import (
	"github.com/openabstractions/abstraction-asks/go/typedpermission"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/fileid"
	"github.com/openabstractions/abstraction-identity/remote"
)

func nativePermissionValidators() typedpermission.Validators {
	return typedpermission.Validators{NativeProgram: fileid.ValidSubjectProgram, RemoteProgram: remote.ValidSubjectProgram, PackagedProgramPrefix: identity.PackagedProgramPrefix}
}
