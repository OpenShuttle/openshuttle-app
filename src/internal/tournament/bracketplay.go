package tournament

import (
	"fmt"
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
)

// A bracket of size S for places [p, p+S-1] plays one round, then its
// winners form a sub-bracket for places [p, p+S/2-1] and, with full
// placement, its losers a sub-bracket for [p+S/2, p+S-1]. Every bracket
// match is the first round of one sub-bracket: (Start, Size, Index).

// subBracket is a node of a bracket: Size entrants playing for places
// Start..Start+Size-1.
type subBracket struct {
	Start, Size int
}

// subBrackets lists the sub-brackets of a bracket of the given size, round by
// round (parents before children), then by place.
func subBrackets(size int, full bool) []subBracket {
	var out []subBracket
	level := []subBracket{{Start: 1, Size: size}}
	for len(level) > 0 {
		out = append(out, level...)
		var next []subBracket
		for _, n := range level {
			if n.Size < 2 {
				continue
			}
			next = append(next, subBracket{n.Start, n.Size / 2})
			if full {
				next = append(next, subBracket{n.Start + n.Size/2, n.Size / 2})
			}
		}
		level = next
	}
	return out
}

// Place is a final position in a bracket. From < To when entrants are tied
// (losers of the same round without full placement).
type Place struct {
	From, To int
	Entrant  engine.EntrantID
}

// slot is a side of a bracket match while results are resolved.
type slot struct {
	id    engine.EntrantID
	state slotState
}

type slotState int

const (
	slotPending slotState = iota // fed by a match not decided yet
	slotKnown
	slotBye
)

func (s *State) generateBracketMatches(c engine.Category, st *Stage, prefix string) {
	n := 0
	for _, node := range subBrackets(len(st.Slots()), st.FullPlacement) {
		for i := range node.Size / 2 {
			n++
			s.matches = append(s.matches, Match{
				ID:       MatchID(fmt.Sprintf("%s%d", prefix, n)),
				Category: c,
				Stage:    st.ID,
				Pool:     -1,
				Start:    node.Start,
				Size:     node.Size,
				Index:    i,
			})
		}
	}
}

// refreshBrackets fills bracket matches with their entrants from the results
// of the matches feeding them, and computes the places. It runs at the end of
// every projection.
func (s *State) refreshBrackets() {
	s.places = map[string][]Place{}
	for c, stages := range s.formats {
		for _, st := range stages {
			if st.Kind == StageBracket && st.Confirmed {
				s.places[string(c)+"/"+st.ID] = s.refreshBracket(c, st)
			}
		}
	}
}

func (s *State) refreshBracket(c engine.Category, b *Stage) []Place {
	slots := map[subBracket][]slot{}
	root := subBracket{1, len(b.Slots())}
	for _, id := range b.Slots() {
		if id == "" {
			slots[root] = append(slots[root], slot{state: slotBye})
		} else {
			slots[root] = append(slots[root], slot{id: id, state: slotKnown})
		}
	}

	var places []Place
	for _, node := range subBrackets(len(b.Slots()), b.FullPlacement) {
		in := slots[node]
		if node.Size == 1 {
			if len(in) == 1 && in[0].state == slotKnown {
				places = append(places, Place{From: node.Start, To: node.Start, Entrant: in[0].id})
			}
			continue
		}
		winners := subBracket{node.Start, node.Size / 2}
		losers := subBracket{node.Start + node.Size/2, node.Size / 2}
		for i := range node.Size / 2 {
			a, bSide := in[2*i], in[2*i+1]
			mi := slices.IndexFunc(s.matches, func(m Match) bool {
				return m.Category == c && m.Stage == b.ID && m.Start == node.Start && m.Size == node.Size && m.Index == i
			})
			if mi < 0 {
				continue
			}
			m := &s.matches[mi]
			m.A, m.B = "", ""
			m.ByeA, m.ByeB = a.state == slotBye, bSide.state == slotBye
			if a.state == slotKnown {
				m.A = a.id
			}
			if bSide.state == slotKnown {
				m.B = bSide.id
			}
			win, lose := s.bracketResult(*m, a, bSide)
			slots[winners] = append(slots[winners], win)
			if b.FullPlacement {
				slots[losers] = append(slots[losers], lose)
			} else if lose.state == slotKnown {
				places = append(places, Place{From: losers.Start, To: losers.Start + losers.Size - 1, Entrant: lose.id})
			}
		}
	}
	slices.SortStableFunc(places, func(x, y Place) int { return x.From - y.From })
	return places
}

// bracketResult returns the winner and loser slots of a bracket match.
func (s *State) bracketResult(m Match, a, b slot) (win, lose slot) {
	bye := slot{state: slotBye}
	switch {
	case a.state == slotPending || b.state == slotPending:
		return slot{}, slot{}
	case a.state == slotBye && b.state == slotBye:
		return bye, bye
	case a.state == slotBye:
		return b, bye
	case b.state == slotBye:
		return a, bye
	}
	// Both entrants known: a forfeit, then the score.
	outA, outB := s.withdrawnFrom(a.id, m), s.withdrawnFrom(b.id, m)
	switch {
	case outA && outB:
		return bye, bye
	case outA:
		return b, a
	case outB:
		return a, b
	case m.Score == nil:
		return slot{}, slot{}
	case m.Score.Winner() == engine.SideA:
		return a, b
	default:
		return b, a
	}
}

// Places returns the final positions known so far in a bracket.
func (s State) Places(c engine.Category, id string) []Place {
	return slices.Clone(s.places[string(c)+"/"+id])
}

// Playable reports whether both entrants of a match are known: pool matches
// always, bracket matches once their feeders are decided (byes never play).
func (s State) Playable(m Match) bool {
	return m.A != "" && m.B != ""
}

// RoundSize returns how many entrants the round of a bracket match has:
// 2 for a final, 4 for a semi-final.
func (m Match) RoundSize() int { return m.Size }

// winnerOf returns the winner of a decided match, "" otherwise.
func (s State) winnerOf(m Match) engine.EntrantID {
	if !s.Playable(m) {
		return ""
	}
	outA, outB := s.withdrawnFrom(m.A, m), s.withdrawnFrom(m.B, m)
	switch {
	case outA && !outB:
		return m.B
	case outB && !outA:
		return m.A
	case m.Score == nil:
		return ""
	case m.Score.Winner() == engine.SideA:
		return m.A
	}
	return m.B
}

// nextStarted reports whether a match fed by this bracket match has started:
// changing its result would then change who played.
func (s State) nextStarted(m Match) bool {
	if m.Pool >= 0 || m.Size < 4 {
		return false
	}
	b := s.stage(m.Category, m.Stage)
	children := []subBracket{{m.Start, m.Size / 2}}
	if b != nil && b.FullPlacement {
		children = append(children, subBracket{m.Start + m.Size/2, m.Size / 2})
	}
	return slices.ContainsFunc(s.matches, func(n Match) bool {
		if n.Category != m.Category || n.Stage != m.Stage || n.Index != m.Index/2 {
			return false
		}
		if !slices.Contains(children, subBracket{n.Start, n.Size}) {
			return false
		}
		return n.Called() || n.Score != nil
	})
}
