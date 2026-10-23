package learning

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

func addProblem(fields map[string]string, field, problem string) {
	if problem != "" {
		fields[field] = problem
	}
}

// validateLength counts characters, not bytes, like char_length in the CHECKs.
func validateLength(s string, max int) string {
	if utf8.RuneCountInString(s) > max {
		return fmt.Sprintf("Use at most %d characters.", max)
	}
	return ""
}

// uuidPattern is the canonical form ids come back in. Anything else cannot
// name a row, and PostgreSQL would fail on it instead of finding nothing.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(id string) bool {
	return uuidPattern.MatchString(id)
}
