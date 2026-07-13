package stringutil

import "strings"

// Truncate shortens text to maxLen characters, appending "..." if truncated.
func Truncate(text string, maxLen int) string {
	if maxLen < 3 {
		return text[:maxLen]
	}
	if len(text) < maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// Capitalize uppercases the first letter of each word.
func Capitalize(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	for i := 1; i < len(words); i++ {
		words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
	}
	return strings.Join(words, " ")
}

// WordCount returns the number of whitespace-separated words.
func WordCount(text string) int {
	return len(strings.Fields(text))
}

// IsPalindrome checks whether text reads the same forwards and backwards,
// ignoring case and spaces.
func IsPalindrome(text string) bool {
	cleaned := strings.ToLower(strings.ReplaceAll(text, " ", ""))
	n := len(cleaned)
	for i := 0; i < n/2; i++ {
		if cleaned[i] != cleaned[n-1-i] {
			return false
		}
	}
	return true
}

// SnakeToTitle converts a snake_case string to Title Case.
func SnakeToTitle(text string) string {
	parts := strings.Split(text, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}
