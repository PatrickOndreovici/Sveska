package identity

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	minPasswordLen    = 10
	maxPasswordLen    = 128 // bounds hashing cost for very long inputs
	maxEmailLen       = 254
	maxDisplayNameLen = 80
)

func normalizeEmail(email string) string {
	return strings.TrimSpace(email)
}

func validateEmail(email string) string {
	if email == "" {
		return "Enter your email address."
	}
	if len(email) > maxEmailLen {
		return "Email address is too long."
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@")+1:], ".") {
		return "Enter a valid email address, like name@example.com."
	}
	return ""
}

func validatePassword(password string) string {
	n := utf8.RuneCountInString(password)
	switch {
	case n == 0:
		return "Enter a password."
	case n < minPasswordLen:
		return "Use at least 10 characters."
	case n > maxPasswordLen:
		return "Use at most 128 characters."
	}
	return ""
}

func normalizeDisplayName(name string) string {
	return strings.TrimSpace(name)
}

func validateDisplayName(name string) string {
	if name == "" {
		return "Enter your name."
	}
	if utf8.RuneCountInString(name) > maxDisplayNameLen {
		return "Use at most 80 characters."
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "Remove line breaks and control characters."
		}
	}
	return ""
}

func validateRole(role Role) string {
	if role != RoleStudent && role != RoleTeacher {
		return "Choose student or teacher."
	}
	return ""
}
