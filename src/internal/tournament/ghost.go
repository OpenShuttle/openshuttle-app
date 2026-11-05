package tournament

import "github.com/OpenShuttle/openshuttle-app/src/internal/events"

// Ghost matches (ADR 11) left a court empty for a while in its turn. Court
// rules (ADR 12) replaced them: files holding their events still open, the
// history lists them, and the schedule ignores them.
const (
	// TypeGhostMatchAdded recorded a ghost match. Ignored.
	TypeGhostMatchAdded events.Type = "GhostMatchAdded"
	// TypeGhostMatchRemoved removed a ghost match. Ignored.
	TypeGhostMatchRemoved events.Type = "GhostMatchRemoved"
)

// GhostMatchAdded is the payload of TypeGhostMatchAdded: a court left empty
// for Minutes after the match After ("" as soon as it was free). The
// ghost's ID came from the event seq ("g12").
type GhostMatchAdded struct {
	Court   int     `json:"court"`
	Minutes int     `json:"minutes"`
	After   MatchID `json:"after,omitempty"`
}

// GhostMatchRemoved is the payload of TypeGhostMatchRemoved.
type GhostMatchRemoved struct {
	ID string `json:"id"`
}
