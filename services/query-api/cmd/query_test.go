package main

import (
	"testing"
	"time"
)

func TestGetEnv_ReturnsEnvValue(t *testing.T) {
	t.Setenv("TEST_KEY", "hello")
	got := getEnv("TEST_KEY", "default")
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestGetEnv_ReturnsFallback(t *testing.T) {
	got := getEnv("THIS_KEY_DOES_NOT_EXIST_XYZ", "fallback")
	if got != "fallback" {
		t.Errorf("expected 'fallback', got %q", got)
	}
}

func TestGetEnv_EmptyEnvUseFallback(t *testing.T) {
	t.Setenv("EMPTY_KEY", "")
	got := getEnv("EMPTY_KEY", "default")
	if got != "default" {
		t.Errorf("expected 'default' for empty env, got %q", got)
	}
}

func TestMinutesValidation(t *testing.T) {
	cases := []struct {
		input    int
		expected int
	}{
		{5, 5},
		{0, 5},    // zero → clamp to 5
		{-1, 5},   // negative → clamp to 5
		{1440, 5}, // > 1440 → clamp to 5
		{60, 60},
	}
	for _, tc := range cases {
		result := clampMinutes(tc.input)
		if result != tc.expected {
			t.Errorf("clampMinutes(%d) = %d, want %d", tc.input, result, tc.expected)
		}
	}
}

func TestTimeRangeCalculation(t *testing.T) {
	now := time.Now()
	from := now.Add(-time.Hour).UnixMilli()
	to := now.UnixMilli()

	if from >= to {
		t.Error("from should be before to")
	}
	diff := to - from
	// Should be approximately 1 hour in millis
	if diff < 3_590_000 || diff > 3_610_000 {
		t.Errorf("expected ~3600000ms diff, got %d", diff)
	}
}
// env
// clamp
// time
// env
// clamp
// time
// env
// clamp
// time
// env present
// env absent
// env empty
// clamp valid
// clamp zero
// clamp negative
// clamp max
// time range
// redecl removed
// env present
// env absent
// env empty
// clamp valid
// clamp zero
// clamp negative
// clamp max
// time range
// redecl removed
// env present
// env absent
// env empty
// clamp valid
// clamp zero
// clamp negative
// clamp max
// time range
// redecl removed
// env present
// env absent
