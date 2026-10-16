package engine

// RoundRobinSchedule orders the matches of a pool of n entrants into rounds
// with the circle method: every pair meets once, and nobody plays twice in a
// round. Entrants are indexes 0..n-1; with an odd n, one entrant rests each
// round. Playing round by round lets every entrant rest between matches.
func RoundRobinSchedule(n int) [][][2]int {
	if n < 2 {
		return nil
	}
	slots := make([]int, n)
	for i := range slots {
		slots[i] = i
	}
	if n%2 == 1 {
		slots = append(slots, -1) // bye
	}
	m := len(slots)
	rounds := make([][][2]int, 0, m-1)
	for range m - 1 {
		var round [][2]int
		for i := range m / 2 {
			a, b := slots[i], slots[m-1-i]
			if a < 0 || b < 0 {
				continue
			}
			if a > b {
				a, b = b, a
			}
			round = append(round, [2]int{a, b})
		}
		rounds = append(rounds, round)
		// Keep the first slot fixed, rotate the others clockwise.
		last := slots[m-1]
		copy(slots[2:], slots[1:m-1])
		slots[1] = last
	}
	return rounds
}
