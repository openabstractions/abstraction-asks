package client

import (
	"context"
	"errors"

	asks "github.com/openabstractions/abstraction-asks/go"
	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
)

// DetailsReader reads trusted details from the retained asker method.
type DetailsReader interface {
	ReadQuestionAskerContext(ctx context.Context, id string) (QuestionAsker, error)
}

type AskerReader = DetailsReader

// ReadAskerContext reads one occurrence's trusted details and bound asker
// (ASK-O11). Only an explicit unknown_method permits the existing legacy
// metadata reader; errors and malformed current details never fall back.
func ReadAskerContext(ctx context.Context, reader DetailsReader, account, id string) (QuestionAsker, error) {
	result, err := reader.ReadQuestionAskerContext(ctx, id)
	if err == nil {
		if err := validatePermissionDetails(result.Details, account, id); err != nil {
			return QuestionAsker{}, err
		}
		if err := validateAskerProgram(result.Details, result.Program); err != nil {
			return QuestionAsker{}, err
		}
		return result, nil
	}
	var service *wire.ServiceError
	if !errors.As(err, &service) || service.Code != wire.ServiceErrorCodeUnknownMethod {
		return QuestionAsker{}, err
	}
	legacy, ok := reader.(interface {
		ReadAttributedQuestionContext(ctx context.Context, id string) (AttributedQuestion, error)
	})
	if !ok {
		return QuestionAsker{}, err
	}
	attributed, err := legacy.ReadAttributedQuestionContext(ctx, id)
	if err != nil {
		return QuestionAsker{}, err
	}
	details := QuestionDetails{Question: attributed}
	q := attributed.Question
	if q.Record != nil && q.Record.ID != id {
		return QuestionAsker{}, errors.New("asks: mismatched permission occurrence")
	}
	if q.Outcome == OperatorQuestionOutcomeFound && q.Record != nil && q.Record.Key == asks.FirstUseKey {
		p, ok := asks.LegacyPermission(account, q.Record.Key, q.Record.About, q.Record.Text)
		if !ok {
			return QuestionAsker{}, errors.New("asks: invalid legacy permission")
		}
		details.Permission = &p
	}
	return QuestionAsker{Details: details}, nil
}

// ReadDetailsContext selects the trusted details from the same asker read.
func ReadDetailsContext(ctx context.Context, reader DetailsReader, account, id string) (QuestionDetails, error) {
	read, err := ReadAskerContext(ctx, reader, account, id)
	return read.Details, err
}

func validatePermissionDetails(details QuestionDetails, account, id string) error {
	q := details.Question.Question
	if q.Record != nil && q.Record.ID != id {
		return errors.New("asks: mismatched permission occurrence")
	}
	if p := details.Permission; p != nil && (q.Outcome != OperatorQuestionOutcomeFound || q.Record == nil || q.Record.Key != asks.FirstUseKey || !asks.ValidPermissionRequest(*p) || p.Targets[0].Account != account) {
		return errors.New("asks: foreign or malformed permission details")
	}
	if q.Outcome == OperatorQuestionOutcomeFound && q.Record != nil && q.Record.Key == asks.FirstUseKey && details.Permission == nil {
		return errors.New("asks: missing structured permission")
	}
	return nil
}
