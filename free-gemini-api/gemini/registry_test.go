package gemini

import (
	"strings"
	"testing"
)

func TestModelRegistryResolve(t *testing.T) {
	reg := GetModelRegistry()

	tests := []struct {
		input          string
		expectedID     string
		expectedHex    string
		headerContains string // what to check in BuildHeaderValue (empty = use expectedHex)
	}{
		{"gemini-3.8-flash", "gemini-3.8-flash", "56fdd199312815e2", ",1,4,"},
		{"flash", "gemini-3.8-flash", "56fdd199312815e2", ",1,4,"},
		{"gemini-flash", "gemini-3.8-flash", "56fdd199312815e2", ",1,4,"},
		{"gemini-3.5-flash-lite", "gemini-3.5-flash-lite", "8c46e95b1a07cecc", ""},
		{"flash-lite", "gemini-3.5-flash-lite", "8c46e95b1a07cecc", ""},
		{"lite", "gemini-3.5-flash-lite", "8c46e95b1a07cecc", ""},
		{"gemini-3.1-pro", "gemini-3.1-pro", "e6fa609c3fa255c0", ""},
		{"gemini-pro", "gemini-3.1-pro", "e6fa609c3fa255c0", ""},
		{"pro", "gemini-3.1-pro", "e6fa609c3fa255c0", ""},
		// Gemini 4 Argon: Default is Medium (4), ModeID is "argon" (idx14=1)
		{"gemini-4-argon", "gemini-4-argon", "argon", ",1,4,"},
		{"argon", "gemini-4-argon", "argon", ",1,4,"},
		{"argon-high", "gemini-4-argon-thinking-high", "argon", ",1,2,"},
		{"argon-medium", "gemini-4-argon-thinking-medium", "argon", ",1,4,"},
		{"argon-low", "gemini-4-argon-thinking-low", "argon", ",1,1,"},
		// Gemini 4 Hydrogen: Default is High (2), ModeID is 56fdd199312815e2 (idx14=1)
		{"gemini-4-hydrogen", "gemini-4-hydrogen", "56fdd199312815e2", ",1,2,"},
		{"hydrogen", "gemini-4-hydrogen", "56fdd199312815e2", ",1,2,"},
		{"hydrogen-high", "gemini-4-hydrogen-thinking-high", "56fdd199312815e2", ",1,2,"},
		{"hydrogen-xhigh", "gemini-4-hydrogen-thinking-xhigh", "56fdd199312815e2", ",1,8,"},
		// Fake models for testing invalid modeIDs
		{"gemini-fake-model-1", "gemini-fake-model-1", "aaaaaaaaaaaaaaaa", ",1,4,"},
		{"fake-1", "gemini-fake-model-1", "aaaaaaaaaaaaaaaa", ",1,4,"},
		{"gemini-fake-model-2", "gemini-fake-model-2", "gemini-6-nonexisting", ",1,4,"},
		{"fake-2", "gemini-fake-model-2", "gemini-6-nonexisting", ",1,4,"},
		// Gemini 3.7 / 3.6 / 3.5 Flash series
		{"gemini-3.7-flash", "gemini-3.7-flash", "56fdd199312815e2", ",1,4,"},
		{"3.7-flash", "gemini-3.7-flash", "56fdd199312815e2", ",1,4,"},
		{"gemini-3.6-flash", "gemini-3.6-flash", "56fdd199312815e2", ",1,4,"},
		{"3.6-flash-minimal", "gemini-3.6-flash-thinking-minimal", "56fdd199312815e2", ""},
		{"gemini-3.5-flash", "gemini-3.5-flash", "56fdd199312815e2", ",1,4,"},
		{"thinking", "gemini-3.8-thinking", "56fdd199312815e2", ""},
		{"", "gemini-3.8-flash", "56fdd199312815e2", ""},
		{"unknown-model-xyz", "gemini-3.8-flash", "56fdd199312815e2", ""},
	}

	for _, tt := range tests {
		m := reg.Resolve(tt.input)
		if m == nil {
			t.Fatalf("Resolve(%q) returned nil", tt.input)
		}
		if m.ID != tt.expectedID {
			t.Errorf("Resolve(%q) ID = %v, expected %v", tt.input, m.ID, tt.expectedID)
		}
		if m.ModeID != tt.expectedHex {
			t.Errorf("Resolve(%q) ModeID = %v, expected %v", tt.input, m.ModeID, tt.expectedHex)
		}

		headerVal := m.BuildHeaderValue()
		checkStr := tt.headerContains
		if checkStr == "" {
			checkStr = tt.expectedHex
		}
		if !strings.Contains(headerVal, checkStr) {
			t.Errorf("BuildHeaderValue() %q does not contain %q", headerVal, checkStr)
		}
	}
}

func TestModelRegistryCustomRegister(t *testing.T) {
	reg := GetModelRegistry()

	custom := ModelConfig{
		ID:          "gemini-test-custom",
		DisplayName: "Gemini Custom Model",
		ModeID:      "12345678abcdef",
		Aliases:     []string{"custom-test"},
		Source:      "custom",
	}

	reg.Register(custom)
	defer reg.Unregister("gemini-test-custom")

	resolved := reg.Resolve("custom-test")
	if resolved.ID != "gemini-test-custom" {
		t.Fatalf("Expected resolved custom model ID 'gemini-test-custom', got %s", resolved.ID)
	}
	if resolved.ModeID != "12345678abcdef" {
		t.Fatalf("Expected ModeID '12345678abcdef', got %s", resolved.ModeID)
	}
}

// TestArgonThinkingLevels verifies that the JSPB header thinking level values
// match what was observed in live CDP traffic (2026-10-14):
//
//	Low=1, Medium=4, High=2  (at index 15)
func TestArgonThinkingLevels(t *testing.T) {
	reg := GetModelRegistry()

	cases := []struct {
		model     string
		wantLevel string
	}{
		{"argon-low", ",1,1,"},
		{"argon-medium", ",1,4,"},
		{"argon-high", ",1,2,"},
	}

	for _, c := range cases {
		m := reg.Resolve(c.model)
		if m == nil {
			t.Fatalf("Resolve(%q) returned nil", c.model)
		}
		h := m.BuildHeaderValue()
		if !strings.Contains(h, c.wantLevel) {
			t.Errorf("Resolve(%q).BuildHeaderValue() = %q, want to contain %q", c.model, h, c.wantLevel)
		}
		// Argon thinking headers contain ModeID at index 4 and thinking level at index 15
		if !strings.Contains(h, "argon") {
			t.Errorf("Resolve(%q).BuildHeaderValue() = %q, should embed ModeID at index 4", c.model, h)
		}
	}
}
