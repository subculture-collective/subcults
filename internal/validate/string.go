// Package validate provides centralized input validation and sanitization utilities
// for the Subcults API. It includes protection against SQL injection, XSS, SSRF,
// and other common web vulnerabilities.
package validate

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

// String validation errors
var (
	ErrStringTooShort    = errors.New("string is too short")
	ErrStringTooLong     = errors.New("string is too long")
	ErrInvalidCharacters = errors.New("string contains invalid characters")
	ErrSQLKeyword        = errors.New("string contains SQL keywords")
	ErrEmpty             = errors.New("string is empty")
)

// Common SQL keywords to detect potential SQL injection attempts
// This is a basic defense layer; parameterized queries are the primary defense
var sqlKeywords = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER",
	"TRUNCATE", "EXEC", "EXECUTE", "UNION", "JOIN", "WHERE", "FROM",
	"--", "/*", "*/", ";--", "xp_", "sp_",
}

// StringConstraints defines validation constraints for a string.
type StringConstraints struct {
	MinLength        int              // Minimum length (0 = no minimum)
	MaxLength        int              // Maximum length (0 = no maximum)
	AllowedPattern   *regexp.Regexp   // Optional regex pattern for allowed characters
	DisallowedWords  []string         // Optional list of disallowed words (case-insensitive)
	CheckSQLKeywords bool             // Whether to check for SQL keywords
	AllowEmpty       bool             // Whether empty strings are allowed
	TrimSpace        bool             // Whether to trim whitespace before validation
}

// String validates a string against the given constraints.
// Returns the validated (and optionally trimmed) string and an error if validation fails.
func String(s string, constraints StringConstraints) (string, error) {
	// Optionally trim whitespace
	if constraints.TrimSpace {
		s = strings.TrimSpace(s)
	}

	// Check if empty
	if s == "" {
		if !constraints.AllowEmpty {
			return "", ErrEmpty
		}
		return s, nil
	}

	// Get actual character count (not byte count)
	length := utf8.RuneCountInString(s)

	// Check minimum length
	if constraints.MinLength > 0 && length < constraints.MinLength {
		return "", fmt.Errorf("%w: got %d chars, need at least %d", ErrStringTooShort, length, constraints.MinLength)
	}

	// Check maximum length
	if constraints.MaxLength > 0 && length > constraints.MaxLength {
		return "", fmt.Errorf("%w: got %d chars, maximum is %d", ErrStringTooLong, length, constraints.MaxLength)
	}

	// Check allowed pattern
	if constraints.AllowedPattern != nil && !constraints.AllowedPattern.MatchString(s) {
		return "", fmt.Errorf("%w: does not match required pattern", ErrInvalidCharacters)
	}

	// Check SQL keywords if enabled
	if constraints.CheckSQLKeywords {
		if err := checkSQLKeywords(s); err != nil {
			return "", err
		}
	}

	// Check disallowed words
	if len(constraints.DisallowedWords) > 0 {
		upper := strings.ToUpper(s)
		for _, word := range constraints.DisallowedWords {
			if strings.Contains(upper, strings.ToUpper(word)) {
				return "", fmt.Errorf("string contains disallowed word: %q", word)
			}
		}
	}

	return s, nil
}

// checkSQLKeywords checks if the string contains common SQL keywords.
// This is a basic heuristic check; parameterized queries are the real defense.
func checkSQLKeywords(s string) error {
	upper := strings.ToUpper(s)
	for _, keyword := range sqlKeywords {
		if strings.Contains(upper, keyword) {
			return fmt.Errorf("%w: contains %q", ErrSQLKeyword, keyword)
		}
	}
	return nil
}

// SanitizeHTML escapes HTML special characters to prevent XSS attacks.
// This should be called on all user-generated text that will be displayed in HTML.
func SanitizeHTML(s string) string {
	return html.EscapeString(s)
}

// SanitizeString performs both validation and HTML sanitization.
// Returns the sanitized string and an error if validation fails.
func SanitizeString(s string, constraints StringConstraints) (string, error) {
	validated, err := String(s, constraints)
	if err != nil {
		return "", err
	}
	return SanitizeHTML(validated), nil
}

// SceneName validates a scene name according to Subcults requirements:
// - 1-100 characters
// - Letters, numbers, spaces, dash, underscore, period only
// - No SQL keywords
func SceneName(name string) (string, error) {
	pattern := regexp.MustCompile(`^[A-Za-z0-9 _\-\.]+$`)
	return SanitizeString(name, StringConstraints{
		MinLength:        1,
		MaxLength:        100,
		AllowedPattern:   pattern,
		CheckSQLKeywords: true,
		AllowEmpty:       false,
		TrimSpace:        true,
	})
}

// EventTitle validates an event title according to Subcults requirements:
// - 1-200 characters
// - No SQL keywords
func EventTitle(title string) (string, error) {
	return SanitizeString(title, StringConstraints{
		MinLength:        1,
		MaxLength:        200,
		CheckSQLKeywords: true,
		AllowEmpty:       false,
		TrimSpace:        true,
	})
}

// PostContent validates post content according to Subcults requirements:
// - Required (not empty)
// - Max 5000 characters
func PostContent(content string) (string, error) {
	return SanitizeString(content, StringConstraints{
		MinLength:        1,
		MaxLength:        5000,
		CheckSQLKeywords: false, // Allow more freedom in post content
		AllowEmpty:       false,
		TrimSpace:        true,
	})
}

// Description validates a description field:
// - Optional (can be empty)
// - Max 5000 characters
func Description(desc string) (string, error) {
	return SanitizeString(desc, StringConstraints{
		MinLength:        0,
		MaxLength:        5000,
		CheckSQLKeywords: false, // Allow more freedom in descriptions
		AllowEmpty:       true,
		TrimSpace:        true,
	})
}
