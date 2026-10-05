package quota

import (
	"testing"
)

func TestParseQuotaString(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"800GB", 800 * 1024 * 1024 * 1024},
		{"1TB", 1024 * 1024 * 1024 * 1024},
		{"500MB", 500 * 1024 * 1024},
		{" 2.5 GB ", uint64(2.5 * 1024 * 1024 * 1024)},
	}

	for _, tt := range tests {
		got := ParseQuotaString(tt.input)
		if got != tt.expected {
			t.Errorf("ParseQuotaString(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	b := uint64(800 * 1024 * 1024 * 1024)
	formatted := FormatBytes(b)
	if formatted != "800.00 GB" {
		t.Errorf("expected 800.00 GB, got %s", formatted)
	}
}
