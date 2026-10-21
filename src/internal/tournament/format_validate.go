package tournament

import (
	"slices"
	"unicode/utf8"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// ValidateFormat checks a format on its own, as a preset: its stages and
// routing, for a tournament of one category.
func ValidateFormat(stages []StageConfig) ([]StageConfig, error) {
	return validFormat(stages, 1)
}

// validFormat checks a format for a tournament of that many categories.
func validFormat(stages []StageConfig, categories int) ([]StageConfig, error) {
	if len(stages) == 0 || len(stages) > MaxStages {
		return nil, engine.NewError(engine.CodeInvalidFormat)
	}
	out := make([]StageConfig, len(stages))
	position := map[string]int{}
	for i, st := range stages {
		if st.ID == "" || position[st.ID] != 0 {
			return nil, engine.NewError(engine.CodeInvalidFormat)
		}
		position[st.ID] = i + 1
	}
	for i, st := range stages {
		st, err := validStage(st, categories)
		if err != nil {
			return nil, err
		}
		for j, r := range st.Routing {
			if st.Routing[j], err = validRule(r, position, i+1); err != nil {
				return nil, err
			}
		}
		out[i] = st
	}
	return out, nil
}

// validStage checks and normalizes a stage: fields of the other kind are
// dropped. Timed matches need a single category.
func validStage(st StageConfig, categories int) (StageConfig, error) {
	st = cloneConfig(st)
	st.Name = collapseSpaces(st.Name)
	if utf8.RuneCountInString(st.Name) > MaxStageNameLength {
		return st, engine.NewError(engine.CodeInvalidName)
	}
	if err := st.Win.Validate(); err != nil {
		return st, err
	}
	if st.Win.Kind == engine.WinByTime {
		if categories > 1 {
			return st, engine.NewError(engine.CodeTimedNeedsOneCategory)
		}
		st.Win = engine.WinCondition{Kind: engine.WinByTime, Minutes: st.Win.Minutes, Estimate: st.Win.Estimate}
	} else {
		st.Win.Minutes = 0
	}
	switch st.Kind {
	case StagePools:
		if len(st.PoolSizes) > 0 {
			if !validPoolSizes(st.PoolSizes) {
				return st, engine.NewError(engine.CodeInvalidFormat)
			}
			st.Pools = 0
		} else if st.Pools < 1 || st.Pools > MaxPools {
			return st, engine.NewError(engine.CodeInvalidFormat)
		}
		if !ValidTiebreaks(st.Tiebreaks) {
			return st, engine.NewError(engine.CodeInvalidFormat)
		}
		if st.CrossPool == "" {
			st.CrossPool = engine.CrossPoolRaw
		}
		if !st.CrossPool.Valid() {
			return st, engine.NewError(engine.CodeInvalidFormat)
		}
		st.FullPlacement = false
	case StageBracket:
		// A bracket is the last phase of its entrants: it ranks them all.
		if len(st.Routing) > 0 {
			return st, engine.NewError(engine.CodeBracketIsFinal)
		}
		st.Pools, st.PoolSizes, st.Tiebreaks, st.CrossPool = 0, nil, nil, ""
	default:
		return st, engine.NewError(engine.CodeInvalidFormat)
	}
	if len(st.Routing) > MaxRules {
		return st, engine.NewError(engine.CodeInvalidRouting)
	}
	if len(st.Routing) == 0 {
		st.Routing = nil
	}
	return st, nil
}

// ValidTiebreaks accepts a non-empty list of known criteria, each at most once.
func ValidTiebreaks(tbs []engine.Tiebreak) bool {
	if len(tbs) == 0 {
		return false
	}
	seen := map[engine.Tiebreak]bool{}
	for _, tb := range tbs {
		if seen[tb] || !slices.Contains(engine.DefaultTiebreaks, tb) {
			return false
		}
		seen[tb] = true
	}
	return true
}

// rulesTo lists the rules sending entrants to a stage.
func rulesTo(rules []RouteRule, id string) []RouteRule {
	var out []RouteRule
	for _, r := range rules {
		if r.To == id {
			out = append(out, r)
		}
	}
	return out
}

func ruleEqual(a, b RouteRule) bool {
	return a.Kind == b.Kind && a.To == b.To && a.Rank == b.Rank && a.Count == b.Count && slices.Equal(a.Ranks, b.Ranks)
}

func cloneConfig(st StageConfig) StageConfig {
	st.Tiebreaks = slices.Clone(st.Tiebreaks)
	st.PoolSizes = slices.Clone(st.PoolSizes)
	st.Routing = slices.Clone(st.Routing)
	for i := range st.Routing {
		st.Routing[i].Ranks = slices.Clone(st.Routing[i].Ranks)
	}
	return st
}

func configEqual(a, b StageConfig) bool {
	return a.ID == b.ID && a.Name == b.Name && a.Kind == b.Kind && a.Win == b.Win && a.Pools == b.Pools &&
		slices.Equal(a.PoolSizes, b.PoolSizes) && a.CrossPool == b.CrossPool && a.FullPlacement == b.FullPlacement &&
		slices.Equal(a.Tiebreaks, b.Tiebreaks) && slices.EqualFunc(a.Routing, b.Routing, ruleEqual)
}
