// Package geo provides geolocation utilities for privacy-preserving location handling.
package geo

import "strings"

// DefaultPrecision is the default geohash precision for public display.
// A precision of 6 characters provides approximately ±0.61 km accuracy,
// which is suitable for coarse location without pinpointing exact venues.
const DefaultPrecision = 6

// validGeohashChars is a lookup map for valid base32 characters used in geohashes.
// Geohash uses a custom base32 alphabet excluding 'a', 'i', 'l', and 'o'.
var validGeohashChars = map[rune]bool{
	'0': true, '1': true, '2': true, '3': true, '4': true,
	'5': true, '6': true, '7': true, '8': true, '9': true,
	'b': true, 'c': true, 'd': true, 'e': true, 'f': true,
	'g': true, 'h': true, 'j': true, 'k': true, 'm': true,
	'n': true, 'p': true, 'q': true, 'r': true, 's': true,
	't': true, 'u': true, 'v': true, 'w': true, 'x': true,
	'y': true, 'z': true,
}

// RoundGeohash truncates a geohash string to the specified precision for privacy.
// It ensures coarse location display by limiting the geohash resolution.
//
// Parameters:
//   - input: the geohash string to round
//   - precision: the desired length (typically 5-6 characters)
//
// Returns:
//   - The truncated geohash if valid
//   - Empty string if input is empty or contains invalid characters
//   - The original input unchanged if it is shorter than precision
func RoundGeohash(input string, precision int) string {
	if input == "" {
		return ""
	}

	if precision < 1 {
		return ""
	}

	// Convert to lowercase for consistent validation
	lower := strings.ToLower(input)

	// Validate that all characters are valid geohash characters
	for _, c := range lower {
		if !validGeohashChars[c] {
			return ""
		}
	}

	// If input is shorter than precision, return as is
	if len(lower) <= precision {
		return lower
	}

	// Truncate to precision
	return lower[:precision]
}
