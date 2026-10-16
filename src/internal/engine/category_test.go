package engine

import "testing"

func TestCategory(t *testing.T) {
	tests := []struct {
		c       Category
		valid   bool
		doubles bool
	}{
		{MensSingles, true, false},
		{WomensSingles, true, false},
		{MensDoubles, true, true},
		{WomensDoubles, true, true},
		{MixedDoubles, true, true},
		{"DH", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		if got := tt.c.Valid(); got != tt.valid {
			t.Errorf("%q.Valid() = %v, want %v", tt.c, got, tt.valid)
		}
		if got := tt.c.IsDoubles(); got != tt.doubles {
			t.Errorf("%q.IsDoubles() = %v, want %v", tt.c, got, tt.doubles)
		}
	}
}

func TestSharesSex(t *testing.T) {
	compatible := map[[2]Category]bool{
		{MensSingles, WomensSingles}: true, {MensSingles, WomensDoubles}: true,
		{MensDoubles, WomensSingles}: true, {MensDoubles, WomensDoubles}: true,
	}
	for _, a := range Categories {
		for _, b := range Categories {
			if a == b {
				continue
			}
			want := !compatible[[2]Category{a, b}] && !compatible[[2]Category{b, a}]
			if got := a.SharesSex(b); got != want {
				t.Errorf("%s/%s SharesSex = %v, want %v", a, b, got, want)
			}
		}
	}
}
