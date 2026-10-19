package identity

import (
	"strings"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"ana@example.com", true},
		{"ana.petrovic+sr@mail.example.rs", true},
		{"", false},
		{"ana", false},
		{"ana@localhost", false},
		{"Ana <ana@example.com>", false},
		{"ana@@example.com", false},
		{strings.Repeat("a", 250) + "@x.rs", false},
	}
	for _, tt := range tests {
		if got := validateEmail(tt.email) == ""; got != tt.valid {
			t.Errorf("validateEmail(%q) valid = %v, want %v", tt.email, got, tt.valid)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
	}{
		{"", false},
		{"123456789", false},
		{"1234567890", true},
		{"čćšžđčćšžđ", true}, // counted in characters, not bytes
		{strings.Repeat("a", 128), true},
		{strings.Repeat("a", 129), false},
	}
	for _, tt := range tests {
		if got := validatePassword(tt.password) == ""; got != tt.valid {
			t.Errorf("validatePassword(%q) valid = %v, want %v", tt.password, got, tt.valid)
		}
	}
}

func TestValidateDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"Ana Petrović", true},
		{"Ана Петровић", true},
		{"", false},
		{"Ana\nPetrović", false},
		{strings.Repeat("a", 80), true},
		{strings.Repeat("a", 81), false},
	}
	for _, tt := range tests {
		if got := validateDisplayName(tt.name) == ""; got != tt.valid {
			t.Errorf("validateDisplayName(%q) valid = %v, want %v", tt.name, got, tt.valid)
		}
	}
}

func TestValidateRole(t *testing.T) {
	for role, valid := range map[Role]bool{RoleStudent: true, RoleTeacher: true, "": false, "admin": false, "Student": false} {
		if got := validateRole(role) == ""; got != valid {
			t.Errorf("validateRole(%q) valid = %v, want %v", role, got, valid)
		}
	}
}
