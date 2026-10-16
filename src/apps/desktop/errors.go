package main

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// errorCause is what the frontend receives as the "cause" of a rejected call.
type errorCause struct {
	Code engine.Code `json:"code"`
	Set  int         `json:"set,omitempty"`
}

// marshalError turns any error returned by a bound method into a stable code.
// Unexpected errors are logged and reported as INTERNAL: Go never sends
// human-readable text to the frontend.
func marshalError(err error) []byte {
	cause := errorCause{Code: engine.CodeInternal}
	var e *engine.Error
	if errors.As(err, &e) {
		cause = errorCause{Code: e.Code, Set: e.Set}
	} else {
		slog.Error("unexpected error", "err", err)
	}
	b, _ := json.Marshal(cause)
	return b
}
