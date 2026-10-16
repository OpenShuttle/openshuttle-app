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
