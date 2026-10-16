package engine

// Code is a stable error code. The frontend translates it; Go never returns
// human-readable text.
type Code string

const (
	CodeInvalidWinCondition Code = "INVALID_WIN_CONDITION"
	CodeInvalidScore        Code = "INVALID_SCORE"
	CodeInvalidSetScore     Code = "INVALID_SET_SCORE"
	CodeInvalidSetCount     Code = "INVALID_SET_COUNT"
	CodeInvalidTimedScore   Code = "INVALID_TIMED_SCORE"
	CodeInvalidCategory     Code = "INVALID_CATEGORY"
)

// Error is a domain error carrying a stable code. Set is the 1-based index of
// the offending set when the error is about one set, 0 otherwise.
type Error struct {
	Code Code
	Set  int
}

func (e *Error) Error() string {
	return string(e.Code)
}

func newError(code Code) *Error {
	return &Error{Code: code}
}
