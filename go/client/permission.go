package client

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	asks "github.com/openabstractions/abstraction-asks/go"
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
		if v.Answer == nil || !asks.ValidRequestKey(v.Answer.ID) || v.Answer.Pending != (v.Outcome == wire.PermissionOutcomePending) || v.PolicyRevision != "" {
			return wire.PermissionObservation{}, errors.New("asks: malformed permission observation")
		}
		if v.Answer.Pending && (v.Answer.Option != "" || v.Answer.Yes || v.Answer.Kept) || !v.Answer.Pending && (v.Answer.Option != "allow" && v.Answer.Option != "refuse" || v.Answer.Yes != (v.Answer.Option == "allow") || v.Answer.Kept) {
			return wire.PermissionObservation{}, errors.New("asks: malformed permission answer")
		}
	case wire.PermissionOutcomePermitted, wire.PermissionOutcomeDenied:
		if v.Answer != nil || !asks.ValidRequestKey(v.PolicyRevision) {
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
	if !asks.ValidRequestKey(key) || waitMS < 0 || waitMS > 30000 {
		return wire.PermissionObservation{}, errors.New("asks: invalid permission observation")
	}
	v, err := wire.NewPermissionApplicationClient(c.transport.WithContext(ctx)).Observe(key, waitMS)
	return checkedPermissionObservation(v, err)
}

type PermissionDetailsReader interface {
	ReadPermissionDetailsContext(ctx context.Context, id string) (wire.TypedPermissionDetails, error)
}

// ReadPermissionDetailsContext validates the entire immutable occurrence. There
// is no legacy method or English fallback for typed permission data.
func ReadPermissionDetailsContext(ctx context.Context, r PermissionDetailsReader, account, id string) (wire.TypedPermissionDetails, error) {
	v, err := r.ReadPermissionDetailsContext(ctx, id)
	if err != nil {
		return wire.TypedPermissionDetails{}, err
	}
	return checkedPermissionDetails(v, account, id)
}
func checkedPermissionDetails(v wire.TypedPermissionDetails, account, id string) (wire.TypedPermissionDetails, error) {
	q := v.Question.Question
	if err := validateQuestionRead(q, id); err != nil {
		return wire.TypedPermissionDetails{}, err
	}
	if q.Outcome != wire.OperatorQuestionOutcomeFound {
		if q.Outcome != wire.OperatorQuestionOutcomeUnknown && q.Outcome != wire.OperatorQuestionOutcomeInvalid && q.Outcome != wire.OperatorQuestionOutcomeForbidden && q.Outcome != wire.OperatorQuestionOutcomeUnavailable || q.Record != nil || q.Publisher != "" || q.AnsweredBy != nil || v.Question.Answerer != "" || v.Effect != nil || v.Display != nil || v.EffectDigest != "" || v.Origin != "" || v.Purpose != "" {
			return wire.TypedPermissionDetails{}, errors.New("asks: malformed typed details refusal")
		}
		return v, nil
	}
	if q.Record == nil || !validPermissionRecord(*q.Record) || q.Record.ID != id || v.Effect == nil || v.Display == nil || v.Effect.Subject.Account != account || q.Record.About != v.Effect.Subject.Program || q.Record.Option == "" && q.AnsweredBy != nil || q.Record.Option != "" && (q.AnsweredBy == nil || *q.AnsweredBy != wire.AnswererOperator) || v.Question.Answerer != "" {
		return wire.TypedPermissionDetails{}, errors.New("asks: malformed or foreign typed details")
	}
	p := asks.TypedPermission{Effect: *v.Effect, Digest: v.EffectDigest, Origin: v.Origin, Purpose: v.Purpose, Display: *v.Display}
	if !asks.ValidTypedPermission(p) {
		return wire.TypedPermissionDetails{}, errors.New("asks: invalid frozen permission effect")
	}
	return ClonePermissionDetails(v), nil
}
func validPermissionRecord(r wire.RecordMetadata) bool {
	return validOperatorRecord(r) && r.Key == asks.TypedPermissionKey && r.Text == "OA permission request" && slices.Equal(r.Options, []string{"allow", "refuse"}) && !r.Kept && (r.Option == "" && !r.Yes || r.Option == "allow" && r.Yes || r.Option == "refuse" && !r.Yes)
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
	if !operatorWord(id) || (option != "allow" && option != "refuse") {
		return wire.OperatorDecision{}, errors.New("asks: invalid permission answer")
	}
	v, err := wire.NewPermissionOperatorClient(c.transport.WithContext(ctx)).AnswerPermissionIfPending(id, option)
	v, err = checkedDecision(v, err, id, option)
	if err == nil && v.Record != nil && !validPermissionRecord(*v.Record) {
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
		if !validPermissionRecord(r) || seen[r.ID] {
			return wire.OperatorPage{}, errors.New("asks: invalid permission page record")
		}
		seen[r.ID] = true
	}
	return v, nil
}
