package client

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/openabstractions/abstraction-asks/go/typedpermission"
	identity "github.com/openabstractions/abstraction-identity"
	"github.com/openabstractions/abstraction-identity/fileid"
	"github.com/openabstractions/abstraction-identity/remote"
	"time"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
	"github.com/openabstractions/abstraction-identity/listen"
)

type PermissionIntent = wire.PermissionIntent
type PermissionObservation = wire.PermissionObservation
type TypedPermissionDetails = wire.TypedPermissionDetails
type PermissionDisplay = wire.PermissionDisplay
type PermissionDisplaySelection = wire.PermissionDisplaySelection
type PermissionOutcome = wire.PermissionOutcome

const (
	PermissionOutcomePending     = wire.PermissionOutcomePending
	PermissionOutcomeAnswered    = wire.PermissionOutcomeAnswered
	PermissionOutcomeUnknown     = wire.PermissionOutcomeUnknown
	PermissionOutcomeGone        = wire.PermissionOutcomeGone
	PermissionOutcomeInvalid     = wire.PermissionOutcomeInvalid
	PermissionOutcomeConflict    = wire.PermissionOutcomeConflict
	PermissionOutcomeForbidden   = wire.PermissionOutcomeForbidden
	PermissionOutcomeUnavailable = wire.PermissionOutcomeUnavailable
	PermissionOutcomePermitted   = wire.PermissionOutcomePermitted
	PermissionOutcomeDenied      = wire.PermissionOutcomeDenied
)

func PermissionOutcomeValues() []PermissionOutcome { return wire.PermissionOutcomeValues() }

type PermissionClient struct{ transport listen.FrameClient }
type PermissionOperator struct{ transport listen.FrameClient }

func NewPermissionWithTransport(t listen.FrameClient) *PermissionClient {
	return &PermissionClient{t.WithDefaults(35*time.Second, 1<<20)}
}
func NewPermission(endpoint string) *PermissionClient {
	return NewPermissionWithTransport(listen.FrameClient{Endpoint: endpoint})
}
func NewPermissionOperatorWithTransport(t listen.FrameClient) *PermissionOperator {
	return &PermissionOperator{t.WithDefaults(5*time.Second, 1<<20)}
}
func NewPermissionOperator(endpoint string) *PermissionOperator {
	return NewPermissionOperatorWithTransport(listen.FrameClient{Endpoint: endpoint})
}

func checkedPermissionObservation(v wire.PermissionObservation, err error) (wire.PermissionObservation, error) {
	if err != nil {
		return wire.PermissionObservation{}, err
	}
	switch v.Outcome {
	case wire.PermissionOutcomePending, wire.PermissionOutcomeAnswered:
		if v.Answer == nil || !operatorWord(v.Answer.ID) || v.Answer.Pending != (v.Outcome == wire.PermissionOutcomePending) || v.PolicyRevision != "" {
			return wire.PermissionObservation{}, errors.New("asks: malformed permission observation")
		}
		affirmative := v.Answer.Option == "allow" || v.Answer.Option == "once"
		if v.Answer.Pending && (v.Answer.Option != "" || v.Answer.Yes || v.Answer.Kept) || !v.Answer.Pending && (!affirmative && v.Answer.Option != "refuse" && v.Answer.Option != "never" || v.Answer.Yes != affirmative || v.Answer.Kept) {
			return wire.PermissionObservation{}, errors.New("asks: malformed permission answer")
		}
	case wire.PermissionOutcomePermitted, wire.PermissionOutcomeDenied:
		if v.Answer != nil || !operatorWord(v.PolicyRevision) {
			return wire.PermissionObservation{}, errors.New("asks: malformed permission decision")
		}
	case wire.PermissionOutcomeUnknown, wire.PermissionOutcomeGone, wire.PermissionOutcomeInvalid, wire.PermissionOutcomeConflict, wire.PermissionOutcomeForbidden, wire.PermissionOutcomeUnavailable:
		if v.Answer != nil || v.PolicyRevision != "" {
			return wire.PermissionObservation{}, errors.New("asks: malformed permission refusal")
		}
	default:
		return wire.PermissionObservation{}, errors.New("asks: unknown permission outcome")
	}
	return v, nil
}
func (c *PermissionClient) RequestContext(ctx context.Context, q wire.PermissionIntent) (wire.PermissionObservation, error) {
	if err := ctx.Err(); err != nil {
		return wire.PermissionObservation{}, err
	}
	v, err := wire.NewPermissionApplicationClient(c.transport.WithContext(ctx)).Request(q)
	return checkedPermissionObservation(v, err)
}
func (c *PermissionClient) ObserveContext(ctx context.Context, key string, waitMS int64) (wire.PermissionObservation, error) {
	if err := ctx.Err(); err != nil {
		return wire.PermissionObservation{}, err
	}
	v, err := wire.NewPermissionApplicationClient(c.transport.WithContext(ctx)).Observe(key, waitMS)
	return checkedPermissionObservation(v, err)
}

// PendingQuestion is one first-use question's observation and, while it is
// pending, the Unix millisecond time it expires (ASK-A14).
type PendingQuestion = wire.PendingQuestion

// ObserveQuestionContext waits up to waitMS for the answer to one first-use
// question about this program, named by a refusal's rights:question_pending
// word or by Request (ASK-A14). Pending is not consent, and an Allow is no
// proof of a committed rule. A current permitted revision supplies the witness
// for one enforcing re-entry. Earlier hosts may return only the stored answer.
func (c *PermissionClient) ObserveQuestionContext(ctx context.Context, id string, waitMS int64) (wire.PendingQuestion, error) {
	if err := ctx.Err(); err != nil {
		return wire.PendingQuestion{}, err
	}
	v, err := wire.NewPermissionApplicationClient(c.transport.WithContext(ctx)).ObserveQuestion(id, waitMS)
	if err != nil {
		return wire.PendingQuestion{}, err
	}
	observation, err := checkedPermissionObservation(v.Observation, nil)
	if err != nil || (v.PendingUntilMs != 0) != (observation.Outcome == wire.PermissionOutcomePending) {
		return wire.PendingQuestion{}, errors.New("asks: malformed question observation")
	}
	return v, nil
}

type PermissionDetailsReader interface {
	ReadPermissionDetailsContext(ctx context.Context, id string) (wire.TypedPermissionDetails, error)
}

// ReadPermissionDetailsContext preserves the legacy authority reader.
// Deprecated: authority presenters use typedpermission.ReadPermissionDetailsContext
// with their explicit native Identity validators. Generic clients project data.
func ReadPermissionDetailsContext(ctx context.Context, r PermissionDetailsReader, account, id string) (wire.TypedPermissionDetails, error) {
	return typedpermission.ReadPermissionDetailsContext(ctx, r, account, id, typedpermission.Validators{PackagedProgramPrefix: identity.PackagedProgramPrefix,
		RemoteProgram: remote.ValidSubjectProgram,
		NativeProgram: fileid.ValidSubjectProgram,
	})
}
func checkedPermissionDetails(v wire.TypedPermissionDetails, account, id string) (wire.TypedPermissionDetails, error) {
	if v.Question.Question.Record != nil && v.Question.Question.Record.ID != id || account != "" && v.Effect != nil && v.Effect.Subject.Account != account {
		return wire.TypedPermissionDetails{}, errors.New("asks: mismatched permission occurrence")
	}
	return ClonePermissionDetails(v), nil
}
func ClonePermissionDetails(v wire.TypedPermissionDetails) wire.TypedPermissionDetails {
	raw, _ := json.Marshal(v)
	var out wire.TypedPermissionDetails
	_ = json.Unmarshal(raw, &out)
	return out
}
func (c *PermissionOperator) ReadPermissionDetailsContext(ctx context.Context, id string) (wire.TypedPermissionDetails, error) {
	if err := ctx.Err(); err != nil {
		return wire.TypedPermissionDetails{}, err
	}
	if !operatorWord(id) {
		return wire.TypedPermissionDetails{}, errors.New("asks: invalid permission ID")
	}
	v, err := wire.NewPermissionOperatorClient(c.transport.WithContext(ctx)).ReadPermissionDetails(id)
	if err != nil {
		return wire.TypedPermissionDetails{}, err
	}
	// The account is bound by the native service; caller-specific consumers check
	// it again with ReadPermissionDetailsContext before presenting authority data.
	account := ""
	if v.Effect != nil {
		account = v.Effect.Subject.Account
	}
	return checkedPermissionDetails(v, account, id)
}
func (c *PermissionOperator) AnswerPermissionIfPendingContext(ctx context.Context, id, option string) (wire.OperatorDecision, error) {
	if err := ctx.Err(); err != nil {
		return wire.OperatorDecision{}, err
	}
	if !operatorWord(id) || (option != "allow" && option != "once" && option != "refuse") {
		return wire.OperatorDecision{}, errors.New("asks: invalid permission answer")
	}
	v, err := wire.NewPermissionOperatorClient(c.transport.WithContext(ctx)).AnswerPermissionIfPending(id, option)
	v, err = checkedDecision(v, err, id, option)
	if err == nil && v.Record != nil && !validOperatorRecord(*v.Record) {
		return wire.OperatorDecision{}, errors.New("asks: wrong permission answer occurrence")
	}
	return v, err
}
func (c *PermissionOperator) ListPermissionsContext(ctx context.Context, cursor string, limit int64) (wire.OperatorPage, error) {
	if err := ctx.Err(); err != nil {
		return wire.OperatorPage{}, err
	}
	if len(cursor) > 256 || limit < 1 || limit > 64 {
		return wire.OperatorPage{}, errors.New("asks: invalid permission page")
	}
	v, err := wire.NewPermissionOperatorClient(c.transport.WithContext(ctx)).ListPermissions(cursor, limit)
	if err != nil {
		return wire.OperatorPage{}, err
	}
	if v.Outcome != wire.OperatorPageOutcomePage {
		if v.Outcome != wire.OperatorPageOutcomeGap && v.Outcome != wire.OperatorPageOutcomeInvalid && v.Outcome != wire.OperatorPageOutcomeForbidden && v.Outcome != wire.OperatorPageOutcomeUnavailable || len(v.Records) > 0 || v.Next != "" || v.Complete {
			return wire.OperatorPage{}, errors.New("asks: malformed permission page refusal")
		}
		return v, nil
	}
	if int64(len(v.Records)) > limit || len(v.Next) > 256 || v.Complete != (v.Next == "") || !v.Complete && (len(v.Records) == 0 || v.Next == cursor) {
		return wire.OperatorPage{}, errors.New("asks: malformed permission page")
	}
	seen := map[string]bool{}
	for _, r := range v.Records {
		if !validOperatorRecord(r) || seen[r.ID] {
			return wire.OperatorPage{}, errors.New("asks: invalid permission page record")
		}
		seen[r.ID] = true
	}
	return v, nil
}
