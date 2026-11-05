package tournament

import (
	"maps"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/tournament/forecast"
)

// CourtRuleKind is what a court rule does (ADR 12).
type CourtRuleKind string

const (
	// CourtPriority: the court takes a ready match of the scope before the
	// matches ahead of it in the schedule order, the lowest level first.
	CourtPriority CourtRuleKind = "priority"
	// CourtOnly: the court takes nothing else while the scope has matches
	// left (with several, any of their scopes).
	CourtOnly CourtRuleKind = "only"
	// CourtNever: the forecast and the proposals never put a match of the
	// scope on the court; the desk still can, with a warning.
	CourtNever CourtRuleKind = "never"
)

// MaxCourtPriority is the lowest priority level (1 is the highest).
const MaxCourtPriority = 9

// CourtScope is what a court rule targets: a category, a stage of it, a
// pool of a pools stage ("A") or a round of a bracket stage, named by how
// many entrants it starts with and the first place it plays for (2 and 1:
// the final; 2 and 3: the third place; 4 and 5: places 5 to 8).
type CourtScope struct {
	Category engine.Category `json:"category"`
	Stage    string          `json:"stage,omitempty"`
	Pool     string          `json:"pool,omitempty"`
	Round    int             `json:"round,omitempty"`
	From     int             `json:"from,omitempty"`
}

// CourtRule is a rule of a court: a kind, a level for a priority, a scope.
type CourtRule struct {
	Kind  CourtRuleKind `json:"kind"`
	Level int           `json:"level,omitempty"`
	Scope CourtScope    `json:"scope"`
}

// CourtRules returns the rules of each court, as ScheduleSettings reads
// them.
func (s State) CourtRules() map[int][]CourtRule {
	return s.ScheduleSettings().Rules
}

// AddCourtRules gives several courts rules, one per scope (a priority on
// pools A and B: two rules), in one event; a court's rule of the same scope
// is replaced, so a court has one rule per scope (the last one given wins).
func (s State) AddCourtRules(courts []int, rules []CourtRule) (Change, error) {
	if len(courts) == 0 {
		return Change{}, engine.NewError(engine.CodeInvalidCourt)
	}
	for _, c := range courts {
		if c < 1 || c > s.Courts() {
			return Change{}, engine.NewError(engine.CodeInvalidCourt)
		}
	}
	if len(rules) == 0 {
		return Change{}, engine.NewError(engine.CodeInvalidSetting)
	}
	checked := make([]CourtRule, len(rules))
	for i, r := range rules {
		var err error
		if checked[i], err = s.checkCourtRule(r); err != nil {
			return Change{}, err
		}
	}
	return s.scheduleChange(func(p *ScheduleSet) {
		for _, c := range courts {
			for _, rule := range checked {
				rs := slices.DeleteFunc(slices.Clone(p.Rules[c]), func(r CourtRule) bool { return r.Scope == rule.Scope })
				p.Rules[c] = append(rs, rule)
			}
		}
	})
}

// RemoveCourtRule removes a court's rule of a scope.
func (s State) RemoveCourtRule(court int, scope CourtScope) (Change, error) {
	if court < 1 || court > s.Courts() {
		return Change{}, engine.NewError(engine.CodeInvalidCourt)
	}
	return s.scheduleChange(func(p *ScheduleSet) {
		p.Rules[court] = slices.DeleteFunc(slices.Clone(p.Rules[court]), func(r CourtRule) bool { return r.Scope == scope })
	})
}

// ClearCourtRules removes every rule of a court.
func (s State) ClearCourtRules(court int) (Change, error) {
	if court < 1 || court > s.Courts() {
		return Change{}, engine.NewError(engine.CodeInvalidCourt)
	}
	return s.scheduleChange(func(p *ScheduleSet) { delete(p.Rules, court) })
}

// checkCourtRule validates a rule, defaults filled in: level 1 for a
// priority (none otherwise), place 1 for a bracket round.
func (s State) checkCourtRule(r CourtRule) (CourtRule, error) {
	switch r.Kind {
	case CourtPriority:
		if r.Level == 0 {
			r.Level = 1
		}
		if r.Level < 1 || r.Level > MaxCourtPriority {
			return r, engine.NewError(engine.CodeInvalidSetting)
		}
	case CourtOnly, CourtNever:
		r.Level = 0
	default:
		return r, engine.NewError(engine.CodeInvalidSetting)
	}
	if r.Scope.Round > 0 && r.Scope.From == 0 {
		r.Scope.From = 1
	}
	if code := s.checkCourtScope(r.Scope); code != "" {
		return r, engine.NewError(code)
	}
	return r, nil
}

// checkCourtScope returns why a scope names nothing of the tournament, ""
// when it is valid.
func (s State) checkCourtScope(sc CourtScope) engine.Code {
	if !s.HasCategory(sc.Category) {
		return engine.CodeCategoryNotFound
	}
	if sc.Stage == "" {
		if sc.Pool != "" || sc.Round != 0 || sc.From != 0 {
			return engine.CodeInvalidSetting
		}
		return ""
	}
	st, ok := s.Stage(sc.Category, sc.Stage)
	if !ok {
		return engine.CodeStageNotFound
	}
	switch {
	case sc.Pool != "" && (st.Kind != StagePools || poolIndex(sc.Pool) < 0 || sc.Round != 0 || sc.From != 0):
		return engine.CodeInvalidSetting
	case sc.Round != 0 && (st.Kind != StageBracket || sc.Round < 2 || sc.Round&(sc.Round-1) != 0 || sc.From < 1):
		return engine.CodeInvalidSetting
	case sc.Round == 0 && sc.From != 0:
		return engine.CodeInvalidSetting
	}
	return ""
}

// poolIndex is the index of a pool name ("A" is 0), -1 when it names none.
func poolIndex(name string) int {
	for i := range 702 { // up to "ZZ"
		if engine.PoolName(i) == name {
			return i
		}
	}
	return -1
}

// courtRules reads the rules of a schedule setup: rules of courts out of
// range or naming what the tournament no longer has are left out. Courts
// reserved by hand in older files (reserved mode, now gone) read as an only
// rule for their category.
func (s State) courtRules(p ScheduleSet) map[int][]CourtRule {
	out := map[int][]CourtRule{}
	for court, rules := range p.Rules {
		if court < 1 || court > s.Courts() {
			continue
		}
		for _, r := range rules {
			if s.checkCourtScope(r.Scope) == "" {
				out[court] = append(out[court], r)
			}
		}
	}
	if p.Mode == legacyReserved {
		for court, c := range p.Courts {
			if court >= 1 && court <= s.Courts() && s.HasCategory(c) && len(out[court]) == 0 {
				out[court] = []CourtRule{{Kind: CourtOnly, Scope: CourtScope{Category: c}}}
			}
		}
	}
	for _, court := range slices.Sorted(maps.Keys(out)) {
		slices.SortStableFunc(out[court], func(a, b CourtRule) int { return ruleRank(a) - ruleRank(b) })
	}
	return out
}

// ruleRank sorts a court's rules: only rules, priorities by level, then
// never rules.
func ruleRank(r CourtRule) int {
	switch r.Kind {
	case CourtOnly:
		return 0
	case CourtPriority:
		return r.Level
	}
	return MaxCourtPriority + 1
}

// forecastRules turns the rules into the forecast's.
func forecastRules(rules map[int][]CourtRule) map[int][]forecast.Rule {
	out := map[int][]forecast.Rule{}
	for court, rs := range rules {
		for _, r := range rs {
			out[court] = append(out[court], forecast.Rule{Kind: forecast.RuleKind(r.Kind), Level: r.Level, Scope: forecastScope(r.Scope)})
		}
	}
	return out
}

func forecastScope(sc CourtScope) forecast.Scope {
	out := forecast.Scope{Category: sc.Category, Stage: sc.Stage, RoundSize: sc.Round, PlaceFrom: sc.From}
	if sc.Pool != "" {
		out.Pool = poolIndex(sc.Pool) + 1
	}
	return out
}

// CourtScopeLeft reports whether a scope still has matches to play, its
// stand-ins of stages not filled yet included: a rule ends with its scope.
func (s State) CourtScopeLeft(sc CourtScope) bool {
	fs := forecastScope(sc)
	return slices.ContainsFunc(s.scheduleItems(), fs.Matches)
}

// CourtScopes lists what a rule can target: per category, its stages with
// matches left, each pools stage with its pools, each bracket stage with
// its rounds (a stage not filled yet from its preview).
func (s State) CourtScopes() []CourtScope {
	var out []CourtScope
	seen := map[CourtScope]bool{}
	add := func(sc CourtScope) {
		if !seen[sc] {
			seen[sc] = true
			out = append(out, sc)
		}
	}
	items := s.scheduleItems()
	for _, c := range s.Categories {
		add(CourtScope{Category: c})
		for _, st := range s.Stages(c) {
			var parts []CourtScope
			for _, it := range items {
				if it.Category != c || it.Stage != st.ID {
					continue
				}
				if it.Pool >= 0 {
					parts = append(parts, CourtScope{Category: c, Stage: st.ID, Pool: engine.PoolName(it.Pool)})
				} else {
					parts = append(parts, CourtScope{Category: c, Stage: st.ID, Round: it.RoundSize, From: it.PlaceFrom})
				}
			}
			if len(parts) == 0 {
				continue
			}
			add(CourtScope{Category: c, Stage: st.ID})
			slices.SortStableFunc(parts, compareScopes)
			for _, p := range parts {
				add(p)
			}
		}
	}
	return out
}

// compareScopes orders pools by name, bracket rounds by place then from
// the first round to the last.
func compareScopes(a, b CourtScope) int {
	if a.Pool != b.Pool {
		if len(a.Pool) != len(b.Pool) {
			return len(a.Pool) - len(b.Pool)
		}
		if a.Pool < b.Pool {
			return -1
		}
		return 1
	}
	if a.From != b.From {
		return a.From - b.From
	}
	return b.Round - a.Round
}

// CourtGate tells, for the state as it is, whether a court's rules let the
// forecast put a match there: calling it by hand anyway is a warning, never
// refused. A court with only rules whose scopes are over takes anything.
func (s State) CourtGate() func(court int, m Match) bool {
	rules := s.CourtRules()
	items := s.scheduleItems()
	running := map[CourtScope]bool{}
	for _, rs := range rules {
		for _, r := range rs {
			if _, ok := running[r.Scope]; !ok {
				running[r.Scope] = slices.ContainsFunc(items, forecastScope(r.Scope).Matches)
			}
		}
	}
	return func(court int, m Match) bool {
		it := forecast.Item{Category: m.Category, Stage: m.Stage, Pool: m.Pool, RoundSize: m.Size, PlaceFrom: m.Start}
		only, inOnly := false, false
		for _, r := range rules[court] {
			holds := forecastScope(r.Scope).Matches(it)
			switch {
			case r.Kind == CourtNever && holds:
				return false
			case r.Kind == CourtOnly && running[r.Scope]:
				only = true
				inOnly = inOnly || holds
			}
		}
		return !only || inOnly
	}
}
