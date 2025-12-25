package utils

import "fmt"

// FormatSize formats a byte count into a human-readable string
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ParseSize parses a human-readable size string to bytes
func ParseSize(s string) (int64, error) {
	var value float64
	var unit string
	_, err := fmt.Sscanf(s, "%f %s", &value, &unit)
	if err != nil {
		// Try without space
		_, err = fmt.Sscanf(s, "%f%s", &value, &unit)
		if err != nil {
			// Try just number (assume bytes)
			_, err = fmt.Sscanf(s, "%f", &value)
			if err != nil {
				return 0, err
			}
			return int64(value), nil
		}
	}

	multiplier := int64(1)
	switch unit {
	case "B", "b":
		multiplier = 1
	case "KB", "kb", "K", "k":
		multiplier = 1024
	case "MB", "mb", "M", "m":
		multiplier = 1024 * 1024
	case "GB", "gb", "G", "g":
		multiplier = 1024 * 1024 * 1024
	case "TB", "tb", "T", "t":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return int64(value * float64(multiplier)), nil
}
