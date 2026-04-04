package provider

import (
	"math"
	"testing"
)

func TestParsePrice(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"$4.99", 4.99},
		{"4.99", 4.99},
		{"$ 4.99", 4.99},
		{"$12.00", 12.00},
		{"$0.99", 0.99},
		{"2/$5.00", 2.50},
		{"3/$9.00", 3.00},
		{"$4.99/lb", 4.99},
		{"$12.99/kg", 12.99},
		{"", 0},
		{"free", 0},
		{"N/A", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parsePrice(tt.input)
			if math.Abs(got-tt.expected) > 0.01 {
				t.Errorf("parsePrice(%q) = %f, want %f", tt.input, got, tt.expected)
			}
		})
	}
}
