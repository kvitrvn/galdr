package metadata

import (
	"strconv"
	"strings"
)

// parseLeadingInt parses the leading decimal integer from s and
// returns it. It accepts values like "2024", "2024-05-21", "3", or
// "3/12". Non-numeric input returns ok=false.
func parseLeadingInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0, false
	}
	return n, true
}
