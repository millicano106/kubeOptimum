package timeutil

import (
	"testing"
	"time"
)

func TestParseWindow(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"60s", 60 * time.Second, false},
		{"", 0, true},
		{"7x", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseWindow(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseWindow(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if err == nil && got != tt.want {
			t.Errorf("ParseWindow(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFormatWindow(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{7 * 24 * time.Hour, "7d"},
		{24 * time.Hour, "1d"},
		{3 * time.Hour, "3h"},
		{90 * time.Minute, "90m"},
	}
	for _, tt := range tests {
		got := FormatWindow(tt.d)
		if got != tt.want {
			t.Errorf("FormatWindow(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
