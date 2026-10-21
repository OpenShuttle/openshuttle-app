package tournament

import (
	"fmt"
	"slices"
	"time"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const TypeFormatSet events.Type = "FormatSet"

// StageKind is how a stage is played.
type StageKind string

const (
	StagePools   StageKind = "pools"   // round-robin pools
	StageBracket StageKind = "bracket" // single elimination
)

// Limits of a format.
const (
	MaxStages          = 8
	MaxPools           = 16
	MaxStageNameLength = 60
)

// DefaultTimedMinutes is the duration of a timed match by default.
const DefaultTimedMinutes = 12

// StageConfig is a stage as the format editor sets it. It is stored in
// FormatSet events and in presets: keep the JSON names.
type StageConfig struct {
	ID string `json:"id"`
	// Name is typed by the desk; empty shows the kind's label.
	Name string              `json:"name"`
	Kind StageKind           `json:"kind"`
	Win  engine.WinCondition `json:"win"`
	// Pools stages: a fixed number of pools (Pools), or pool sizes in order
	// of preference (PoolSizes, 4 then 3 then 6), the number of pools then
	// following the number of entrants. CrossPool compares entrants of the
	// same rank across pools.
	Pools     int               `json:"pools,omitempty"`
	PoolSizes []int             `json:"poolSizes,omitempty"`
	Tiebreaks []engine.Tiebreak `json:"tiebreaks,omitempty"`
	CrossPool engine.CrossPool  `json:"crossPool,omitempty"`
	// Bracket stages: every place is played (losers play for the lower
	// places). Otherwise entrants losing in the same round are tied.
	FullPlacement bool `json:"fullPlacement,omitempty"`
	// Routing sends entrants to later stages; the others keep their final
	// rank here.
	Routing []RouteRule `json:"routing,omitempty"`
}

// FormatSet is the payload of TypeFormatSet: the whole format of a category.
type FormatSet struct {
	Category engine.Category `json:"category"`
	Stages   []StageConfig   `json:"stages"`
}

// Stage is a stage of a category and how far it went.
type Stage struct {
	StageConfig
	// Groups are the pools or, for a bracket, a single group: the first-round
	// slots ("" is a bye). Nil until filled.
	Groups [][]engine.EntrantID
	// Confirmed is set once the stage has matches: the entry stage when its
	// draw is confirmed, later stages as soon as they are filled.
	Confirmed   bool
	ConfirmedAt time.Time
}

// Filled reports whether the stage has entrants.
func (st Stage) Filled() bool { return st.Groups != nil }

// Slots returns the first-round slots of a bracket stage.
func (st Stage) Slots() []engine.EntrantID {
	if st.Kind != StageBracket || len(st.Groups) == 0 {
		return nil
	}
	return st.Groups[0]
}

// DefaultPoolsStage is the entry stage of a new category: 2 pools, best of
// three sets of 21 capped at 30, default tiebreaks.
func DefaultPoolsStage(id string) StageConfig {
	return StageConfig{
		ID:        id,
		Kind:      StagePools,
		Win:       engine.WinCondition{Kind: engine.WinBySets, SetsToWin: 2, PointsPerSet: 21, Cap: 30},
		Pools:     2,
		Tiebreaks: slices.Clone(engine.DefaultTiebreaks),
		CrossPool: engine.CrossPoolRaw,
	}
}

// Stages returns the format of a category, entry stage first.
func (s State) Stages(c engine.Category) []Stage {
	out := make([]Stage, 0, len(s.formats[c]))
	for _, st := range s.formats[c] {
		out = append(out, *st)
	}
	return out
}

// StageConfigs returns the format of a category as the editor sets it.
func (s State) StageConfigs(c engine.Category) []StageConfig {
	out := make([]StageConfig, 0, len(s.formats[c]))
	for _, st := range s.formats[c] {
		out = append(out, cloneConfig(st.StageConfig))
	}
	return out
}

// Stage returns a stage of a category by ID.
func (s State) Stage(c engine.Category, id string) (Stage, bool) {
	if st := s.stage(c, id); st != nil {
		return *st, true
	}
	return Stage{}, false
}

func (s State) stage(c engine.Category, id string) *Stage {
	for _, st := range s.formats[c] {
		if st.ID == id {
			return st
		}
	}
	return nil
}

// stageIndex is the position of a stage in its format, -1 if unknown.
func (s State) stageIndex(c engine.Category, id string) int {
	return slices.IndexFunc(s.formats[c], func(st *Stage) bool { return st.ID == id })
}

// entry returns the entry stage of a category, nil without a category.
func (s State) entry(c engine.Category) *Stage {
	if len(s.formats[c]) == 0 {
		return nil
	}
	return s.formats[c][0]
}

// Phase returns the phase of a stage of a category.
func (s State) Phase(c engine.Category, id string) int {
	return Phases(s.StageConfigs(c))[id]
}

// Phases returns the phase of every stage: 1 for the entry stage, one more
// than the latest stage feeding it, or, for a later stage nothing feeds yet,
// one more than the last pools stage before it (a bracket feeds nobody).
func Phases(stages []StageConfig) map[string]int {
	phases := map[string]int{}
	for i, st := range stages {
		if phases[st.ID] == 0 {
			// Nothing feeds it yet: it comes after the last pools stage before
			// it, the only kind that can feed it.
			phases[st.ID] = 1
			for _, prev := range slices.Backward(stages[:i]) {
				if prev.Kind == StagePools {
					phases[st.ID] = phases[prev.ID] + 1
					break
				}
			}
		}
		for _, to := range st.Targets() {
			phases[to] = max(phases[to], phases[st.ID]+1)
		}
	}
	return phases
}

// Sources returns the stages routing entrants to a stage.
func (s State) Sources(c engine.Category, id string) []string {
	var out []string
	for _, st := range s.formats[c] {
		if slices.Contains(st.Targets(), id) {
			out = append(out, st.ID)
		}
	}
	return out
}

// SetFormat changes the stages of a category, same stages in the same order.
// Stages are added and removed with AddStage and RemoveStage.
func (s State) SetFormat(c engine.Category, stages []StageConfig) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	current := s.formats[c]
	if len(stages) != len(current) {
		return Change{}, engine.NewError(engine.CodeInvalidFormat)
	}
	for i, st := range stages {
		if st.ID != current[i].ID {
			return Change{}, engine.NewError(engine.CodeInvalidFormat)
		}
	}
	return s.formatChange(c, stages)
}

// AddStage adds a stage at the end of the format, with the win condition of
// the last stage.
func (s State) AddStage(c engine.Category, kind StageKind) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	stages := s.StageConfigs(c)
	if len(stages) >= MaxStages {
		return Change{}, engine.NewError(engine.CodeTooManyStages)
	}
	// Only a pools stage can feed the new one: brackets route nobody on.
	if !slices.ContainsFunc(stages, func(st StageConfig) bool { return st.Kind == StagePools }) {
		return Change{}, engine.NewError(engine.CodeBracketIsFinal)
	}
	st := DefaultPoolsStage(fmt.Sprintf("S%d", s.stageNumbers[c]+1))
	if n := len(stages); n > 0 {
		st.Win = stages[n-1].Win
	}
	if kind == StageBracket {
		st.Kind, st.Pools, st.Tiebreaks, st.CrossPool, st.FullPlacement = StageBracket, 0, nil, "", true
	} else if kind != StagePools {
		return Change{}, engine.NewError(engine.CodeInvalidFormat)
	}
	return s.formatChange(c, append(stages, st))
}

// RemoveStage removes a stage that has no entrants yet. Routing to it is
// cleared. The last stage of a format cannot be removed.
func (s State) RemoveStage(c engine.Category, id string) (Change, error) {
	st := s.stage(c, id)
	switch {
	case st == nil:
		return Change{}, engine.NewError(engine.CodeStageNotFound)
	case len(s.formats[c]) == 1:
		return Change{}, engine.NewError(engine.CodeInvalidFormat)
	case s.FormatLocked(c):
		return Change{}, engine.NewError(engine.CodeFormatLocked)
	case st.Filled():
		return Change{}, engine.NewError(engine.CodeStageFilled)
	}
	var stages []StageConfig
	for _, cfg := range s.StageConfigs(c) {
		if cfg.ID == id {
			continue
		}
		cfg.Routing = slices.DeleteFunc(cfg.Routing, func(r RouteRule) bool { return r.To == id })
		stages = append(stages, cfg)
	}
	return s.formatChange(c, stages)
}

// ApplyFormat replaces the whole format of a category, from a preset or
// another tournament. Stage IDs are renumbered from S1: before the draw is
// confirmed, no event refers to a stage yet. Refused once it is confirmed.
func (s State) ApplyFormat(c engine.Category, stages []StageConfig) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	if e := s.entry(c); e != nil && e.Confirmed {
		return Change{}, engine.NewError(engine.CodeDrawConfirmed)
	}
	renumbered, err := renumberFormat(stages)
	if err != nil {
		return Change{}, err
	}
	return s.formatChange(c, renumbered)
}

// renumberFormat numbers the stages of a format from S1, routing included.
func renumberFormat(stages []StageConfig) ([]StageConfig, error) {
	if len(stages) == 0 || len(stages) > MaxStages {
		return nil, engine.NewError(engine.CodeInvalidFormat)
	}
	ids := map[string]string{"": ""}
	renumbered := make([]StageConfig, len(stages))
	for i, st := range stages {
		ids[st.ID] = fmt.Sprintf("S%d", i+1)
		renumbered[i] = cloneConfig(st)
	}
	for i := range renumbered {
		renumbered[i].ID = ids[stages[i].ID]
		for j, r := range renumbered[i].Routing {
			id, ok := ids[r.To]
			if !ok {
				return nil, engine.NewError(engine.CodeInvalidRouting)
			}
			renumbered[i].Routing[j].To = id
		}
	}
	return renumbered, nil
}

// formatChange validates a new format against the current one.
func (s State) formatChange(c engine.Category, stages []StageConfig) (Change, error) {
	if s.FormatLocked(c) {
		return Change{}, engine.NewError(engine.CodeFormatLocked)
	}
	stages = slices.Clone(stages)
	position := map[string]int{}
	for i, st := range stages {
		if st.ID == "" || position[st.ID] != 0 {
			return Change{}, engine.NewError(engine.CodeInvalidFormat)
		}
		position[st.ID] = i + 1
	}
	for i := range stages {
		st, err := validStage(stages[i], len(s.Categories))
		if err != nil {
			return Change{}, err
		}
		for j, r := range st.Routing {
			// Routing only points forward: the format stays acyclic.
			if st.Routing[j], err = validRule(r, position, i+1); err != nil {
				return Change{}, err
			}
		}
		stages[i] = st
	}

	// What already happened constrains the change.
	for _, cur := range s.formats[c] {
		i := position[cur.ID] - 1
		if i < 0 {
			if cur.Filled() {
				return Change{}, engine.NewError(engine.CodeStageFilled)
			}
			continue
		}
		next := stages[i]
		started := s.stageStarted(c, cur.ID)
		switch {
		case cur.Filled() && next.Kind != cur.Kind:
			return Change{}, engine.NewError(engine.CodeStageFilled)
		case cur.Confirmed && next.Kind == StagePools && (next.Pools != cur.Pools || !slices.Equal(next.PoolSizes, cur.PoolSizes)):
			return Change{}, engine.NewError(engine.CodeDrawConfirmed)
		case started && next.FullPlacement != cur.FullPlacement:
			return Change{}, engine.NewError(engine.CodeBracketStarted)
		}
		// Ranks feeding a filled stage are settled.
		for _, other := range s.formats[c] {
			if other == cur || !other.Filled() {
				continue
			}
			if !slices.EqualFunc(rulesTo(cur.Routing, other.ID), rulesTo(next.Routing, other.ID), ruleEqual) {
				return Change{}, engine.NewError(engine.CodeStageFilled)
			}
		}
	}
	if slices.EqualFunc(stages, s.StageConfigs(c), configEqual) {
		return Change{}, nil
	}
	return Change{TypeFormatSet, FormatSet{Category: c, Stages: stages}}, nil
}

// hasTimedStage reports whether a category plays timed matches. A default
// format follows the number of categories: only formats set by the desk
// count.
func (s State) hasTimedStage() bool {
	for _, c := range s.Categories {
		if !s.formatSet[c] {
			continue
		}
		for _, st := range s.formats[c] {
			if st.Win.Kind == engine.WinByTime {
				return true
			}
		}
	}
	return false
}

// stageStarted reports whether a match of the stage was called or decided on
// court (byes do not count).
// FormatLocked reports whether a match of the category was called or
// scored: its format can no longer change.
func (s State) FormatLocked(c engine.Category) bool {
	return slices.ContainsFunc(s.formats[c], func(st *Stage) bool { return s.stageStarted(c, st.ID) })
}

func (s State) stageStarted(c engine.Category, id string) bool {
	return slices.ContainsFunc(s.matches, func(m Match) bool {
		return m.Category == c && m.Stage == id && (m.Called() || m.Score != nil)
	})
}

// newFormat gives a new category its default format: timed pools while a
// single category plays and the app defaults to timed pools.
func (s *State) newFormat(c engine.Category) {
	if s.formats == nil {
		s.formats = map[engine.Category][]*Stage{}
		s.formatSet = map[engine.Category]bool{}
		s.stageNumbers = map[engine.Category]int{}
	}
	s.formats[c] = []*Stage{{StageConfig: DefaultPoolsStage("S1")}}
	s.stageNumbers[c] = max(s.stageNumbers[c], 1)
	s.formatSet[c] = false
	s.refreshDefaultFormats()
}

// refreshDefaultFormats keeps the default win condition of formats the desk
// never set in line with the number of categories.
func (s *State) refreshDefaultFormats() {
	for _, c := range s.Categories {
		if s.formatSet[c] || len(s.formats[c]) == 0 {
			continue
		}
		win := DefaultPoolsStage("").Win
		if s.defaultTimed && len(s.Categories) == 1 {
			win = engine.WinCondition{Kind: engine.WinByTime, Minutes: DefaultTimedMinutes}
		}
		s.formats[c][0].Win = win
	}
}

func (s *State) applyFormatSet(p FormatSet) {
	if s.formats == nil || s.formats[p.Category] == nil {
		s.newFormat(p.Category)
	}
	old := map[string]*Stage{}
	for _, st := range s.formats[p.Category] {
		old[st.ID] = st
	}
	next := make([]*Stage, 0, len(p.Stages))
	for _, cfg := range p.Stages {
		st := &Stage{StageConfig: cloneConfig(cfg)}
		regenerate := false
		if prev := old[cfg.ID]; prev != nil {
			regenerate = prev.Confirmed && prev.FullPlacement != cfg.FullPlacement
			delete(old, cfg.ID)
			st.Groups, st.Confirmed, st.ConfirmedAt = prev.Groups, prev.Confirmed, prev.ConfirmedAt
			// A draft draw no longer matching the stage is dropped.
			if prev.Kind != cfg.Kind || (!prev.Confirmed && cfg.Kind == StagePools &&
				(prev.Pools != cfg.Pools || !slices.Equal(prev.PoolSizes, cfg.PoolSizes))) {
				st.Groups, st.Confirmed = nil, false
			}
		}
		var n int
		if _, err := fmt.Sscanf(cfg.ID, "S%d", &n); err == nil {
			s.stageNumbers[p.Category] = max(s.stageNumbers[p.Category], n)
		}
		next = append(next, st)
		if regenerate {
			defer s.generateStageMatches(p.Category, st)
		}
	}
	for id := range old {
		s.dropStageMatches(p.Category, id)
	}
	s.formats[p.Category] = next
	s.formatSet[p.Category] = true
}
