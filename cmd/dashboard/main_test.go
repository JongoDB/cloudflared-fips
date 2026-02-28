package main

import (
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// envOrFlag
// ---------------------------------------------------------------------------

func TestEnvOrFlag_FlagPriority(t *testing.T) {
	os.Setenv("TEST_DASH_ENVORFLAG", "env-value")
	defer os.Unsetenv("TEST_DASH_ENVORFLAG")

	got := envOrFlag("flag-value", "TEST_DASH_ENVORFLAG")
	if got != "flag-value" {
		t.Errorf("envOrFlag with flag = %q, want flag-value", got)
	}
}

func TestEnvOrFlag_EnvFallback(t *testing.T) {
	os.Setenv("TEST_DASH_ENVORFLAG2", "from-env")
	defer os.Unsetenv("TEST_DASH_ENVORFLAG2")

	got := envOrFlag("", "TEST_DASH_ENVORFLAG2")
	if got != "from-env" {
		t.Errorf("envOrFlag with empty flag = %q, want from-env", got)
	}
}

func TestEnvOrFlag_BothEmpty(t *testing.T) {
	os.Unsetenv("TEST_DASH_NOEXIST")
	got := envOrFlag("", "TEST_DASH_NOEXIST")
	if got != "" {
		t.Errorf("envOrFlag both empty = %q, want empty", got)
	}
}

func TestEnvOrFlag_FlagWithWhitespace(t *testing.T) {
	got := envOrFlag("  value  ", "TEST_DASH_NOEXIST2")
	if got != "  value  " {
		t.Errorf("envOrFlag should preserve whitespace = %q", got)
	}
}
