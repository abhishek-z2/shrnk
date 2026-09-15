package shortcode

import "testing"

func TestEncode(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{61, "z"},
		{62, "10"},
		{63, "11"},
		{125, "21"},
	}

	for _, tt := range tests {
		got := Encode(tt.input)

		if got != tt.expected {
			t.Errorf("Encode(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
