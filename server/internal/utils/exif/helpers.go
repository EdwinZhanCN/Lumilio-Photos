package exif

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseDateTime parses various datetime formats commonly found in EXIF data
func parseDateTime(dateStr string) (time.Time, error) {
	// Common datetime formats used in EXIF data
	formats := []string{
		"2006:01:02 15:04:05",        // Standard EXIF format
		"2006-01-02 15:04:05",        // ISO format variant
		"2006:01:02 15:04:05-07:00",  // EXIF with timezone
		"2006-01-02T15:04:05Z",       // ISO 8601 UTC
		"2006-01-02T15:04:05-07:00",  // ISO 8601 with timezone
		"2006:01:02 15:04:05.000",    // EXIF with milliseconds
		"2006-01-02T15:04:05.000Z",   // ISO with milliseconds
		"2006:01:02 15:04:05.000000", // EXIF with microseconds
	}

	// Clean the input string
	dateStr = strings.TrimSpace(dateStr)

	// Try each format
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse datetime: %s", dateStr)
}

// parseGPSCoordinate parses GPS coordinates from various EXIF formats
func parseGPSCoordinate(coordStr string) (float64, error) {
	// Clean the input
	coordStr = strings.TrimSpace(coordStr)

	// Remove common suffixes
	coordStr = strings.TrimSuffix(coordStr, " deg")
	coordStr = strings.TrimSuffix(coordStr, "°")

	// Try direct float parsing first
	if val, err := strconv.ParseFloat(coordStr, 64); err == nil {
		return val, nil
	}

	// Handle degree/minute/second format
	if strings.Contains(coordStr, "deg") || strings.Contains(coordStr, "°") {
		return parseDMSCoordinate(coordStr)
	}

	// Handle decimal degrees with direction
	if strings.HasSuffix(coordStr, " N") || strings.HasSuffix(coordStr, " S") ||
		strings.HasSuffix(coordStr, " E") || strings.HasSuffix(coordStr, " W") {

		direction := coordStr[len(coordStr)-1:]
		numStr := strings.TrimSpace(coordStr[:len(coordStr)-2])

		if val, err := strconv.ParseFloat(numStr, 64); err == nil {
			// Make negative for South and West
			if direction == "S" || direction == "W" {
				val = -val
			}
			return val, nil
		}
	}

	return 0, fmt.Errorf("unable to parse GPS coordinate: %s", coordStr)
}

// parseDMSCoordinate parses degree/minute/second format coordinates
// Example: "37 deg 46' 29.66\" N" -> 37.774906
func parseDMSCoordinate(dmsStr string) (float64, error) {
	// This is a simplified DMS parser
	// Full implementation would handle all variations of DMS notation

	// Extract direction (N, S, E, W)
	direction := ""
	if strings.HasSuffix(dmsStr, " N") || strings.HasSuffix(dmsStr, " S") ||
		strings.HasSuffix(dmsStr, " E") || strings.HasSuffix(dmsStr, " W") {
		direction = dmsStr[len(dmsStr)-1:]
		dmsStr = strings.TrimSpace(dmsStr[:len(dmsStr)-2])
	}

	// Split by degree marker
	parts := strings.Split(dmsStr, " deg")
	if len(parts) < 2 {
		parts = strings.Split(dmsStr, "°")
	}

	if len(parts) > 0 {
		if deg, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil {
			result := deg

			// For now, just return degrees (full DMS parsing would be more complex)
			// TODO: Implement full minutes and seconds parsing

			// Apply direction
			if direction == "S" || direction == "W" {
				result = -result
			}

			return result, nil
		}
	}

	return 0, fmt.Errorf("unable to parse DMS coordinate: %s", dmsStr)
}

// parseBitrate parses bitrate values from various formats
func parseBitrate(bitrateStr string) (int, error) {
	original := bitrateStr
	bitrateStr = strings.ToLower(strings.TrimSpace(bitrateStr))

	// Determine multiplier based on units
	multiplier := 1
	if strings.Contains(bitrateStr, "mbps") || strings.Contains(bitrateStr, "mb/s") {
		multiplier = 1000000
		bitrateStr = strings.ReplaceAll(bitrateStr, "mbps", "")
		bitrateStr = strings.ReplaceAll(bitrateStr, "mb/s", "")
	} else if strings.Contains(bitrateStr, "kbps") || strings.Contains(bitrateStr, "kb/s") {
		multiplier = 1000
		bitrateStr = strings.ReplaceAll(bitrateStr, "kbps", "")
		bitrateStr = strings.ReplaceAll(bitrateStr, "kb/s", "")
	} else {
		// Remove common suffixes
		bitrateStr = strings.TrimSuffix(bitrateStr, " bps")
		bitrateStr = strings.TrimSuffix(bitrateStr, " b/s")
	}

	// Clean up whitespace
	bitrateStr = strings.TrimSpace(bitrateStr)

	// Parse the numeric value
	if val, err := strconv.ParseFloat(bitrateStr, 64); err == nil {
		return int(val) * multiplier, nil
	}

	return 0, fmt.Errorf("unable to parse bitrate: %s", original)
}

// parseSampleRate parses sample rate values from various formats
func parseSampleRate(sampleRateStr string) (int, error) {
	original := sampleRateStr
	sampleRateStr = strings.ToLower(strings.TrimSpace(sampleRateStr))

	// Determine multiplier based on units
	multiplier := 1
	if strings.Contains(sampleRateStr, "khz") {
		multiplier = 1000
		sampleRateStr = strings.ReplaceAll(sampleRateStr, "khz", "")
	} else if strings.Contains(sampleRateStr, "mhz") {
		multiplier = 1000000
		sampleRateStr = strings.ReplaceAll(sampleRateStr, "mhz", "")
	} else {
		// Remove hz suffix
		sampleRateStr = strings.TrimSuffix(sampleRateStr, " hz")
		sampleRateStr = strings.TrimSuffix(sampleRateStr, "hz")
	}

	// Clean up whitespace
	sampleRateStr = strings.TrimSpace(sampleRateStr)

	// Parse the numeric value
	if val, err := strconv.ParseFloat(sampleRateStr, 64); err == nil {
		return int(val) * multiplier, nil
	}

	return 0, fmt.Errorf("unable to parse sample rate: %s", original)
}

// parseYear extracts year from various date formats
func parseYear(yearStr string) (int, error) {
	yearStr = strings.TrimSpace(yearStr)

	// Try direct integer parsing first
	if year, err := strconv.Atoi(yearStr); err == nil {
		if isValidYear(year) {
			return year, nil
		}
	}

	// Try to parse as date and extract year
	if t, err := parseDateTime(yearStr); err == nil {
		return t.Year(), nil
	}

	// Try to extract 4-digit year from string
	for i := 0; i <= len(yearStr)-4; i++ {
		if year, err := strconv.Atoi(yearStr[i : i+4]); err == nil {
			if isValidYear(year) {
				return year, nil
			}
		}
	}

	return 0, fmt.Errorf("unable to parse year: %s", yearStr)
}

// isValidYear checks if a year is within reasonable bounds
func isValidYear(year int) bool {
	currentYear := time.Now().Year()
	return year >= 1800 && year <= currentYear+10 // Allow some future dates
}

// normalizeString normalizes a string by trimming whitespace and handling encoding issues
func normalizeString(s string) string {
	// Trim whitespace
	s = strings.TrimSpace(s)

	// Remove null bytes that sometimes appear in EXIF data
	s = strings.ReplaceAll(s, "\x00", "")

	// Handle common encoding issues
	if s == "" || s == "null" || s == "undefined" {
		return ""
	}

	return s
}

// parseNumericValue attempts to parse a numeric value from a string with error handling
func parseNumericValue(s string, targetType string) (interface{}, error) {
	s = normalizeString(s)
	if s == "" {
		return nil, fmt.Errorf("empty string")
	}

	switch targetType {
	case "int":
		return strconv.Atoi(s)
	case "float32":
		val, err := strconv.ParseFloat(s, 32)
		return float32(val), err
	case "float64":
		return strconv.ParseFloat(s, 64)
	default:
		return nil, fmt.Errorf("unsupported target type: %s", targetType)
	}
}

// extractFirstValidValue returns the first non-empty value from a slice of strings
func extractFirstValidValue(values []string) string {
	for _, value := range values {
		normalized := normalizeString(value)
		if normalized != "" {
			return normalized
		}
	}
	return ""
}

// parseFrameRate parses frame rate values which can be in various formats
func parseFrameRate(frameRateStr string) (float64, error) {
	frameRateStr = normalizeString(frameRateStr)

	// Remove common suffixes
	frameRateStr = strings.TrimSuffix(frameRateStr, " fps")
	frameRateStr = strings.TrimSuffix(frameRateStr, "fps")
	frameRateStr = strings.TrimSpace(frameRateStr)

	// Handle fraction format (e.g., "30000/1001")
	if strings.Contains(frameRateStr, "/") {
		parts := strings.Split(frameRateStr, "/")
		if len(parts) == 2 {
			numerator, err1 := strconv.ParseFloat(parts[0], 64)
			denominator, err2 := strconv.ParseFloat(parts[1], 64)

			if err1 == nil && err2 == nil && denominator != 0 {
				return numerator / denominator, nil
			}
		}
	}

	// Direct float parsing
	return strconv.ParseFloat(frameRateStr, 64)
}
