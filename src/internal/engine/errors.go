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

	CodeTooManyCategories Code = "TOO_MANY_CATEGORIES"
	CodeCategoryExists    Code = "CATEGORY_EXISTS"
	CodeCategoryNotFound  Code = "CATEGORY_NOT_FOUND"
	CodeCategoryNotEmpty  Code = "CATEGORY_NOT_EMPTY"
	CodeInvalidPlayerName Code = "INVALID_PLAYER_NAME"
	CodeInvalidClub       Code = "INVALID_CLUB"
	CodeWrongPlayerCount  Code = "WRONG_PLAYER_COUNT"
	CodeEntrantNotFound   Code = "ENTRANT_NOT_FOUND"
	CodePlayerNotFound    Code = "PLAYER_NOT_FOUND"

	CodeInvalidFormat         Code = "INVALID_FORMAT"
	CodeTimedNeedsOneCategory Code = "TIMED_NEEDS_ONE_CATEGORY"
	CodeInvalidSeed           Code = "INVALID_SEED"
	CodeNotEnoughEntrants     Code = "NOT_ENOUGH_ENTRANTS"
	CodeNoDraw                Code = "NO_DRAW"
	CodeDrawConfirmed         Code = "DRAW_CONFIRMED"
	CodeInvalidPool           Code = "INVALID_POOL"
	CodeEntrantNotInDraw      Code = "ENTRANT_NOT_IN_DRAW"

	CodeNothingToUndo  Code = "NOTHING_TO_UNDO"
	CodeInvalidName    Code = "INVALID_NAME"
	CodeNoTournament   Code = "NO_TOURNAMENT"
	CodeInvalidSetting Code = "INVALID_SETTING"
	CodeInternal       Code = "INTERNAL"
	CodeInvalidEvent   Code = "INVALID_EVENT"

	CodeFileExists         Code = "FILE_EXISTS"
	CodeFileNotFound       Code = "FILE_NOT_FOUND"
	CodeNotATournamentFile Code = "NOT_A_TOURNAMENT_FILE"
	CodeFileTooNew         Code = "FILE_TOO_NEW"
)

// Error is a domain error carrying a stable code. Set is the 1-based index of
// the offending set when the error is about one set, 0 otherwise.
type Error struct {
	Code Code `json:"code"`
	Set  int  `json:"set,omitempty"`
}

func (e *Error) Error() string {
	return string(e.Code)
}

// NewError returns an error with the given code.
func NewError(code Code) *Error {
	return &Error{Code: code}
}

func newError(code Code) *Error {
	return &Error{Code: code}
}
