package asks

import "errors"

// Refusal codes are stable protocol identifiers. Error text remains diagnostic.
const (
	CodeInternal         = "internal"
	CodeInvalidRequest   = "invalid_request"
	CodeCallerRefused    = "caller_refused"
	CodeUnknownOperation = "unknown_operation"
	CodeNotAdministrator = "not_administrator"
	CodeWithdrawn        = "withdrawn"
	CodeUnknownQuestion  = "unknown_question"
	CodeUnknownOption    = "unknown_option"
	CodeBadSlot          = "bad_slot"
	CodeNothingPending   = "nothing_pending"
	CodeNoRecord         = "no_record"
)

// RemoteError is a service refusal. Code is empty for a legacy text-only reply.
// Unknown codes are retained; callers must treat them as refusals too.
type RemoteError struct {
	Code    string
	Message string
}

func (e *RemoteError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "asks: " + e.Code
}

// Unwrap preserves native error identity for recognized refusal codes.
func (e *RemoteError) Unwrap() error {
	switch e.Code {
	case CodeUnknownQuestion:
		return ErrUnknownQuestion
	case CodeUnknownOption:
		return ErrUnknownOption
	case CodeBadSlot:
		return ErrBadSlot
	case CodeNothingPending:
		return ErrNothingPending
	case CodeNoRecord:
		return ErrNoRecord
	}
	return nil
}

// Err reports refusal even when a newer server sends a code without prose.
func (r Response) Err() error {
	if r.Error == "" && r.Code == "" {
		return nil
	}
	return &RemoteError{Code: r.Code, Message: r.Error}
}

func failure(err error) Response {
	code := CodeInternal
	switch {
	case errors.Is(err, ErrUnknownQuestion):
		code = CodeUnknownQuestion
	case errors.Is(err, ErrUnknownOption):
		code = CodeUnknownOption
	case errors.Is(err, ErrBadSlot):
		code = CodeBadSlot
	case errors.Is(err, ErrNothingPending):
		code = CodeNothingPending
	case errors.Is(err, ErrNoRecord):
		code = CodeNoRecord
	}
	message := err.Error()
	if message == "" {
		message = "asks: " + code
	}
	return Response{Error: message, Code: code}
}
