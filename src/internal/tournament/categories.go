package tournament

import (
	"slices"

	"github.com/OpenShuttle/openshuttle-app/src/internal/engine"
	"github.com/OpenShuttle/openshuttle-app/src/internal/events"
)

const (
	TypeCategoryAdded   events.Type = "CategoryAdded"
	TypeCategoryRemoved events.Type = "CategoryRemoved"
)

// MaxCategories is the most categories a v1 tournament runs at once.
const MaxCategories = 2

// CategoryAdded is the payload of TypeCategoryAdded.
type CategoryAdded struct {
	Category engine.Category `json:"category"`
}

// CategoryRemoved is the payload of TypeCategoryRemoved.
type CategoryRemoved struct {
	Category engine.Category `json:"category"`
}

// HasCategory reports whether the tournament plays this category.
func (s State) HasCategory(c engine.Category) bool {
	return slices.Contains(s.Categories, c)
}

// AddCategory adds a category: at most two, each type once.
func (s State) AddCategory(c engine.Category) (Change, error) {
	switch {
	case !c.Valid():
		return Change{}, engine.NewError(engine.CodeInvalidCategory)
	case s.HasCategory(c):
		return Change{}, engine.NewError(engine.CodeCategoryExists)
	case len(s.Categories) >= MaxCategories:
		return Change{}, engine.NewError(engine.CodeTooManyCategories)
	case slices.ContainsFunc(s.Categories, c.SharesSex):
		// Men's or women's players would be split between two categories.
		return Change{}, engine.NewError(engine.CodeCategoriesShareSex)
	case s.hasTimedStage():
		// Timed rounds start and end together on every court: a single category only.
		return Change{}, engine.NewError(engine.CodeTimedNeedsOneCategory)
	}
	return Change{TypeCategoryAdded, CategoryAdded{Category: c}}, nil
}

// AvailableCategories returns the categories that can still be added, in
// display order.
func (s State) AvailableCategories() []engine.Category {
	out := []engine.Category{}
	for _, c := range engine.Categories {
		if _, err := s.AddCategory(c); err == nil {
			out = append(out, c)
		}
	}
	return out
}

// RemoveCategory removes a category with no entrant. Removing a category
// with entrants is refused: it would silently drop them.
func (s State) RemoveCategory(c engine.Category) (Change, error) {
	if !s.HasCategory(c) {
		return Change{}, engine.NewError(engine.CodeCategoryNotFound)
	}
	for _, e := range s.Entrants {
		if e.Category == c {
			return Change{}, engine.NewError(engine.CodeCategoryNotEmpty)
		}
	}
	return Change{TypeCategoryRemoved, CategoryRemoved{Category: c}}, nil
}

func (s *State) applyCategoryAdded(p CategoryAdded) {
	if !s.HasCategory(p.Category) {
		s.Categories = append(s.Categories, p.Category)
	}
}

func (s *State) applyCategoryRemoved(p CategoryRemoved) {
	s.Categories = slices.DeleteFunc(s.Categories, func(c engine.Category) bool { return c == p.Category })
	delete(s.stages, p.Category)
	delete(s.draws, p.Category)
}
