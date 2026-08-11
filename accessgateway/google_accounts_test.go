package accessgateway

import (
	"regexp"
	"testing"
)

func TestRandomPasswordIsSuitableForWorkspaceUser(t *testing.T) {
	value, err := randomPassword()
	if err != nil {
		t.Fatal(err)
	}
	if len(value) != 16 {
		t.Fatalf("password length = %d", len(value))
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(value) || !regexp.MustCompile(`[a-z]`).MatchString(value) || !regexp.MustCompile(`[0-9]`).MatchString(value) {
		t.Fatalf("password lacks required character classes: %q", value)
	}
}
