package client

import (
	"context"
	"errors"
	"github.com/openabstractions/abstraction-asks/go/typedpermission"

	wire "github.com/openabstractions/abstraction-asks/go/abstraction/asks/api"
)

type CatalogueContent = wire.CatalogueContent
type QuestionContent = wire.QuestionContent
type AnswerContent = wire.AnswerContent
type AnswerContentPage = wire.AnswerContentPage

func (c *Operator) ReadQuestionContentContext(ctx context.Context, id string) (QuestionContent, error) {
	if err := ctx.Err(); err != nil {
		return QuestionContent{}, err
	}
	if !operatorWord(id) {
		return QuestionContent{}, errors.New("asks: invalid question id")
	}
	result, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).ReadQuestionContent(id)
	if err != nil {
		return QuestionContent{}, err
	}
	if err := validateDetails(result.Question.Details, id); err != nil {
		return QuestionContent{}, err
	}
	if err := validateAskerProgram(result.Question.Details, result.Question.Program); err != nil {
		return QuestionContent{}, err
	}
	if err := validateContent(result); err != nil {
		return QuestionContent{}, err
	}
	return result, nil
}

func validateContent(result QuestionContent) error {
	return typedpermission.CheckContent(result, nativePermissionValidators())
}

// ReadContentContext preserves the explicit older-service fallback. Absent
// content never triggers English recovery; existing authority compatibility
// remains solely in ReadAskerContext.
// Deprecated: use typedpermission.ReadContentContext at the authority presenter.
func ReadContentContext(ctx context.Context, reader DetailsReader, account, id string) (QuestionContent, error) {
	return typedpermission.ReadContentContext(ctx, reader, account, id, nativePermissionValidators())
}

func (c *Operator) ListAnswerContentsContext(ctx context.Context, cursor string, limit int64) (AnswerContentPage, error) {
	if err := ctx.Err(); err != nil {
		return AnswerContentPage{}, err
	}
	if len(cursor) > 256 || limit < 1 || limit > 64 {
		return AnswerContentPage{}, errors.New("asks: invalid answer audit range")
	}
	page, err := wire.NewQuestionOperatorClient(c.transport.WithContext(ctx)).ListAnswerContents(cursor, limit)
	if err != nil {
		var service *wire.ServiceError
		if !errors.As(err, &service) || service.Code != wire.ServiceErrorCodeUnknownMethod {
			return AnswerContentPage{}, err
		}
		legacy, err := c.ListAnswersContext(ctx, cursor, limit)
		if err != nil {
			return AnswerContentPage{}, err
		}
		page = AnswerContentPage{Outcome: legacy.Outcome, Answers: []AnswerContent{}, Next: legacy.Next, Complete: legacy.Complete, CompleteSinceMs: legacy.CompleteSinceMs}
		for _, answer := range legacy.Answers {
			page.Answers = append(page.Answers, AnswerContent{Answer: answer})
		}
	}
	if page.Outcome != OperatorPageOutcomePage {
		if len(page.Answers) != 0 || page.Next != "" || page.Complete || page.CompleteSinceMs != 0 {
			return AnswerContentPage{}, errors.New("asks: inconsistent answer content refusal")
		}
		return page, nil
	}
	if int64(len(page.Answers)) > limit || len(page.Next) > 256 || page.Complete != (page.Next == "") || page.CompleteSinceMs < 0 {
		return AnswerContentPage{}, errors.New("asks: inconsistent answer content page")
	}
	for i, content := range page.Answers {
		answer := content.Answer
		if !typedpermission.ValidAnswerEntry(answer, nativePermissionValidators()) || i > 0 && answer.AnsweredMs > page.Answers[i-1].Answer.AnsweredMs ||
			content.Content != nil && (answer.Kind != AnswerKindQuestion || !typedpermission.ValidCatalogueContent(answer.Key, answer.About, *content.Content, nativePermissionValidators())) {
			return AnswerContentPage{}, errors.New("asks: invalid audited content")
		}
	}
	return page, nil
}
