package version

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Version
		ok   bool
	}{
		{"1.0.1", Version{1, 0, 1}, true},
		{"2.13.0", Version{2, 13, 0}, true},
		{"0.0.0", Version{0, 0, 0}, true},
		{"1.0", Version{}, false},
		{"1.0.1.2", Version{}, false},
		{"v1.0.1", Version{}, false},
		{"1.0.1-beta", Version{}, false},
		{"01.0.1", Version{}, false},
		{"1.-1.0", Version{}, false},
		{"1.+1.0", Version{}, false},
		{"", Version{}, false},
		{Dev, Version{}, false},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("Parse(%q) = %v, %v; want %v ok=%v", tt.in, got, err, tt.want, tt.ok)
		}
		if tt.ok && got.String() != tt.in {
			t.Errorf("String() = %q, want %q", got.String(), tt.in)
		}
	}
}

func TestCompatible(t *testing.T) {
	desktop := Version{1, 4, 2}
	if !desktop.Compatible(Version{1, 0, 7}) {
		t.Error("same MAJOR should be compatible")
	}
	if desktop.Compatible(Version{2, 0, 0}) {
		t.Error("different MAJOR should not be compatible")
	}
}
