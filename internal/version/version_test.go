package version_test

import (
	"strings"
	"testing"

	"github.com/jorgenosberg/agentcfg/internal/version"
)

func TestString_ContainsVersion(t *testing.T) {
	s := version.String()
	if s == "" {
		t.Fatal("version.String() returned empty string")
	}
	// Default values set in version.go: "dev", "none", "unknown".
	if !strings.Contains(s, "dev") {
		t.Errorf("expected 'dev' in version string: %q", s)
	}
}

func TestString_HasParentheses(t *testing.T) {
	s := version.String()
	// Format: "<version> (<commit>, <date>)"
	if !strings.Contains(s, "(") || !strings.Contains(s, ")") {
		t.Errorf("expected parenthesised commit+date in version string: %q", s)
	}
}
