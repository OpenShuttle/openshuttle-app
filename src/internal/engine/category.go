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
