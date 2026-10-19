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

	CodeTooManyCategories  Code = "TOO_MANY_CATEGORIES"
	CodeCategoryExists     Code = "CATEGORY_EXISTS"
	CodeCategoryNotFound   Code = "CATEGORY_NOT_FOUND"
	CodeCategoryNotEmpty   Code = "CATEGORY_NOT_EMPTY"
	CodeCategoriesShareSex Code = "CATEGORIES_SHARE_SEX"
	CodeInvalidPlayerName  Code = "INVALID_PLAYER_NAME"
	CodeInvalidClub        Code = "INVALID_CLUB"
	CodeWrongPlayerCount   Code = "WRONG_PLAYER_COUNT"
	CodeEntrantNotFound    Code = "ENTRANT_NOT_FOUND"
	CodePlayerNotFound     Code = "PLAYER_NOT_FOUND"

	CodeInvalidFormat         Code = "INVALID_FORMAT"
	CodeTimedNeedsOneCategory Code = "TIMED_NEEDS_ONE_CATEGORY"
	CodeInvalidSeed           Code = "INVALID_SEED"
	CodeNotEnoughEntrants     Code = "NOT_ENOUGH_ENTRANTS"
	CodeNoDraw                Code = "NO_DRAW"
	CodeDrawConfirmed         Code = "DRAW_CONFIRMED"
	CodeInvalidPool           Code = "INVALID_POOL"
	CodeEntrantNotInDraw      Code = "ENTRANT_NOT_IN_DRAW"

	CodeInvalidCourts     Code = "INVALID_COURTS"
	CodeInvalidRest       Code = "INVALID_REST"
	CodeInvalidPriority   Code = "INVALID_PRIORITY"
	CodeMatchNotFound     Code = "MATCH_NOT_FOUND"
	CodeMatchStarted      Code = "MATCH_STARTED"
	CodeMatchNotPlaying   Code = "MATCH_NOT_PLAYING"
	CodeMatchWalkover     Code = "MATCH_WALKOVER"
	CodeInvalidCourt      Code = "INVALID_COURT"
	CodeCourtBusy         Code = "COURT_BUSY"
	CodeEntrantPlaying    Code = "ENTRANT_PLAYING"
	CodeMatchesStarted    Code = "MATCHES_STARTED"
	CodeEntrantInPools    Code = "ENTRANT_IN_POOLS"
	CodeEntrantNotInPools Code = "ENTRANT_NOT_IN_POOLS"

	CodeNoScore             Code = "NO_SCORE"
	CodeEntrantNotWithdrawn Code = "ENTRANT_NOT_WITHDRAWN"
	CodeNoTie               Code = "NO_TIE"

	CodeStageNotFound  Code = "STAGE_NOT_FOUND"
	CodeTooManyStages  Code = "TOO_MANY_STAGES"
	CodeStageFilled    Code = "STAGE_FILLED"
	CodeNoPoolSplit    Code = "NO_POOL_SPLIT"
	CodePresetNotFound Code = "PRESET_NOT_FOUND"
	CodePresetReadOnly Code = "PRESET_READ_ONLY"
	CodeInvalidRouting Code = "INVALID_ROUTING"
	// CodeBracketIsFinal: a bracket routes nobody on, and a new stage needs a
	// pools stage to feed it.
	CodeBracketIsFinal Code = "BRACKET_IS_FINAL"
	// CodeFormatLocked: a match of the category started, its format is fixed.
	CodeFormatLocked   Code = "FORMAT_LOCKED"
	CodeClubNotFound   Code = "CLUB_NOT_FOUND"
	CodeAlreadyEntered Code = "ALREADY_ENTERED"
	CodeInvalidFile    Code = "INVALID_FILE"
	// Schedule: nothing waiting to put in order, or a move before the order
	// is validated.
	CodeNothingToSchedule    Code = "NOTHING_TO_SCHEDULE"
	CodeScheduleNotValidated Code = "SCHEDULE_NOT_VALIDATED"
	CodePoolsNotComplete     Code = "POOLS_NOT_COMPLETE"
	CodeTieUnresolved        Code = "TIE_UNRESOLVED"
	CodeBracketStarted       Code = "BRACKET_STARTED"
	CodeBracketNotDrawn      Code = "BRACKET_NOT_DRAWN"
	CodeInvalidSlot          Code = "INVALID_SLOT"
	CodeMatchNotReady        Code = "MATCH_NOT_READY"
	CodeBracketNextStarted   Code = "BRACKET_NEXT_STARTED"

	CodeWriteFailed Code = "WRITE_FAILED"

	CodeRoundInProgress Code = "ROUND_IN_PROGRESS"
	CodeNoRound         Code = "NO_ROUND"
	CodeNoMatchReady    Code = "NO_MATCH_READY"

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
	CodeFileTooOld         Code = "FILE_TOO_OLD"
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
