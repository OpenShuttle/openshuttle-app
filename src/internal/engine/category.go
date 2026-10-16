package engine

// Category is the kind of match played: singles or doubles, men, women or
// mixed. Labels are translated by the frontend (MS/SH, MD/DH…).
type Category string

const (
	MensSingles   Category = "MS"
	WomensSingles Category = "WS"
	MensDoubles   Category = "MD"
	WomensDoubles Category = "WD"
	MixedDoubles  Category = "XD"
)

// Categories lists every category, in display order.
var Categories = []Category{MensSingles, WomensSingles, MensDoubles, WomensDoubles, MixedDoubles}

// Valid reports whether c is a known category.
func (c Category) Valid() bool {
	for _, known := range Categories {
		if c == known {
			return true
		}
	}
	return false
}

// IsDoubles reports whether entrants of this category are pairs.
func (c Category) IsDoubles() bool {
	return c == MensDoubles || c == WomensDoubles || c == MixedDoubles
}

// Sexes reports which players a category involves: men, women, or both for
// mixed doubles.
func (c Category) Sexes() (men, women bool) {
	switch c {
	case MensSingles, MensDoubles:
		return true, false
	case WomensSingles, WomensDoubles:
		return false, true
	case MixedDoubles:
		return true, true
	}
	return false, false
}

// SharesSex reports whether two categories involve players of the same sex.
// Such categories cannot run in the same tournament: a player may only enter
// one category, and club tournaments draw from the same players.
func (c Category) SharesSex(other Category) bool {
	m1, w1 := c.Sexes()
	m2, w2 := other.Sexes()
	return (m1 && m2) || (w1 && w2)
}
