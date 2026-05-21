package units

import "fmt"

// MillicoresToCores converts integer millicores to fractional cores.
func MillicoresToCores(m int64) float64 {
	return float64(m) / 1000.0
}

// FormatCores formats a core count using millicores notation (e.g. 0.012 → "12m", 1.5 → "1500m").
func FormatCores(cores float64) string {
	return fmt.Sprintf("%.0fm", cores*1000)
}

// FormatBytes converts bytes to a human-readable string.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// BytesToMiB converts bytes to mebibytes.
func BytesToMiB(b float64) float64 {
	return b / (1024 * 1024)
}

// MiBToBytes converts mebibytes to bytes.
func MiBToBytes(mib float64) float64 {
	return mib * 1024 * 1024
}
