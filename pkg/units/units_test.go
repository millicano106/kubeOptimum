package units

import "testing"

func TestFormatCores(t *testing.T) {
	tests := []struct {
		cores float64
		want  string
	}{
		{0.012, "12m"},
		{0.5, "500m"},
		{1.0, "1000m"},
		{1.5, "1500m"},
		{2.0, "2000m"},
	}
	for _, tt := range tests {
		got := FormatCores(tt.cores)
		if got != tt.want {
			t.Errorf("FormatCores(%v) = %q, want %q", tt.cores, got, tt.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		b    int64
		want string
	}{
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{512 * 1024 * 1024, "512.0 MiB"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.b)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.b, got, tt.want)
		}
	}
}
