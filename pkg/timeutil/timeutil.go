package timeutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseWindow parses a lookback window string like "7d", "24h", "30m" into a time.Duration.
// Supports d (days), h (hours), m (minutes), s (seconds).
func ParseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty window string")
	}

	unit := s[len(s)-1]
	valueStr := s[:len(s)-1]

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid window %q: %w", s, err)
	}

	switch unit {
	case 'd':
		return time.Duration(value * float64(24*time.Hour)), nil
	case 'h':
		return time.Duration(value * float64(time.Hour)), nil
	case 'm':
		return time.Duration(value * float64(time.Minute)), nil
	case 's':
		return time.Duration(value * float64(time.Second)), nil
	default:
		return 0, fmt.Errorf("unknown time unit %q in %q (use d, h, m, s)", string(unit), s)
	}
}

// FormatWindow formats a duration back into a compact window string.
func FormatWindow(d time.Duration) string {
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
