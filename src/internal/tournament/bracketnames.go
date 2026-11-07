package tournament

import "github.com/OpenShuttle/openshuttle-app/src/internal/engine"

// MatchSource is where a side of a bracket match comes from: the winner, or
// the loser, of an earlier match of the bracket.
type MatchSource struct {
	Match Match
	Loser bool
}

// bracketKey finds a bracket match by its place in the tree.
type bracketKey struct {
	category    engine.Category
	stage       string
	start, size int
	index       int
}

// BracketNames names bracket matches as the desk reads them: "QF A", "SF B",
// and a side not known yet as "winner of QF A" or "loser of SF B".
type BracketNames struct {
	matches map[bracketKey]Match
	// letters holds the letter of each match among the played matches of its
	// sub-bracket; none when it plays a single one.
	letters map[MatchID]string
}

// BracketNames indexes the bracket matches of the tournament.
func (s State) BracketNames() BracketNames {
	n := BracketNames{matches: map[bracketKey]Match{}, letters: map[MatchID]string{}}
	type node struct {
		category    engine.Category
		stage       string
		start, size int
	}
	played := map[node][]MatchID{}
	var order []node
	for _, m := range s.matches {
		if !m.IsBracket() {
			continue
		}
		n.matches[bracketKey{m.Category, m.Stage, m.Start, m.Size, m.Index}] = m
		if m.ByeA || m.ByeB {
			continue // never played: no letter
		}
		k := node{m.Category, m.Stage, m.Start, m.Size}
		if _, ok := played[k]; !ok {
			order = append(order, k)
		}
		played[k] = append(played[k], m.ID)
	}
	for _, k := range order {
		if ids := played[k]; len(ids) > 1 {
			for i, id := range ids {
				n.letters[id] = engine.PoolName(i)
			}
		}
	}
	return n
}

// Letter returns the letter of a bracket match within its round of its part
// of the bracket ("A", "B"…), "" when that part plays a single match.
func (n BracketNames) Letter(id MatchID) string {
	return n.letters[id]
}

// Sources returns where each side of a bracket match comes from, nil for a
// side of the first round or a bye. A match never played (a bye side) is
// passed through: its winner is the side it was given.
func (n BracketNames) Sources(m Match) (a, b *MatchSource) {
	if !m.IsBracket() {
		return nil, nil
	}
	return n.source(m, 2*m.Index), n.source(m, 2*m.Index+1)
}

// source finds the match feeding a side of m, i the index of that match in
// the sub-bracket m was split from.
func (n BracketNames) source(m Match, i int) *MatchSource {
	// A sub-bracket (Start, Size) is the winners' half of (Start, 2·Size) when
	// Start opens a block of 2·Size places, the losers' half of
	// (Start-Size, 2·Size) otherwise.
	parent, loser := m.Start, false
	if (m.Start-1)%(2*m.Size) != 0 {
		parent, loser = m.Start-m.Size, true
	}
	from, ok := n.matches[bracketKey{m.Category, m.Stage, parent, 2 * m.Size, i}]
	if !ok {
		return nil // first round
	}
	if from.ByeA || from.ByeB {
		if loser || (from.ByeA && from.ByeB) {
			return nil // the loser of a match never played is a bye
		}
		side := 2 * from.Index
		if from.ByeA {
			side++
		}
		return n.source(from, side)
	}
	return &MatchSource{Match: from, Loser: loser}
}
