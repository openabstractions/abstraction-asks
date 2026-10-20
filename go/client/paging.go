package client

import "context"

// Page is one page of a cursor listing: the shape every asks operator page
// shares.
type Page[R any] struct {
	Outcome  OperatorPageOutcome
	Records  []R
	Next     string
	Complete bool
}

// PageRefusal reports why a ListAll method returned no records. Outcome is the
// service's page outcome, "gap" when the book changed twice, "stalled" when a
// page's Next is empty or repeats its cursor, or "incomplete" when the page
// budget ran out.
type PageRefusal struct{ Outcome string }

func (r *PageRefusal) Error() string { return "asks: list refused: " + r.Outcome }

// ListAllQuestionsContext reads every page of the question book (ASK-O2).
func (c *Operator) ListAllQuestionsContext(ctx context.Context) ([]RecordMetadata, error) {
	return listAll(ctx, func(ctx context.Context, cursor string, limit int64) (Page[RecordMetadata], error) {
		page, err := c.ListQuestionsContext(ctx, cursor, limit)
		return Page[RecordMetadata](page), err
	})
}

// ListAllChoicesContext reads every page of the shared choices (ASK-O2).
func (c *Operator) ListAllChoicesContext(ctx context.Context) ([]ChoiceRecord, error) {
	return listAll(ctx, func(ctx context.Context, cursor string, limit int64) (Page[ChoiceRecord], error) {
		page, err := c.ListChoicesContext(ctx, cursor, limit)
		return Page[ChoiceRecord](page), err
	})
}

// ListAllPermissionsContext reads every page of the typed permission
// questions (ASK-O2).
func (c *PermissionOperator) ListAllPermissionsContext(ctx context.Context) ([]RecordMetadata, error) {
	return listAll(ctx, func(ctx context.Context, cursor string, limit int64) (Page[RecordMetadata], error) {
		page, err := c.ListPermissionsContext(ctx, cursor, limit)
		return Page[RecordMetadata](page), err
	})
}

// ListAllNotificationsContext reads every page of the notifications (ASK-O2).
func (c *NotificationOperator) ListAllNotificationsContext(ctx context.Context) ([]NotificationRecord, error) {
	return listAll(ctx, func(ctx context.Context, cursor string, limit int64) (Page[NotificationRecord], error) {
		page, err := c.ListNotificationsContext(ctx, cursor, limit)
		return Page[NotificationRecord](page), err
	})
}

// listAll is the one owner of the ASK-O2 read-all rule behind every ListAll
// method. A gap restarts once from an empty cursor and discards the records
// read before it. A page that is not complete and whose Next is empty or
// repeats its cursor is refused as stalled. The question, choice and
// permission validators already reject such a page as malformed; the
// notification validator does not, so this check is reachable there.
func listAll[R any](ctx context.Context, list func(ctx context.Context, cursor string, limit int64) (Page[R], error)) ([]R, error) {
	for attempt := 0; attempt < 2; attempt++ {
		var records []R
		cursor, gap := "", false
		for page := 0; page < 64 && !gap; page++ {
			result, err := list(ctx, cursor, 64)
			if err != nil {
				return nil, err
			}
			switch result.Outcome {
			case OperatorPageOutcomePage:
			case OperatorPageOutcomeGap:
				gap = true
				continue
			default:
				return nil, &PageRefusal{Outcome: result.Outcome.String()}
			}
			records = append(records, result.Records...)
			if result.Complete {
				return records, nil
			}
			if result.Next == "" || result.Next == cursor {
				return nil, &PageRefusal{Outcome: "stalled"}
			}
			cursor = result.Next
		}
		if !gap {
			return nil, &PageRefusal{Outcome: "incomplete"}
		}
	}
	return nil, &PageRefusal{Outcome: "gap"}
}
