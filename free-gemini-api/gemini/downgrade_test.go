package gemini

import (
	"testing"
)

func TestModelDowngradeDetection(t *testing.T) {
	tests := []struct {
		requested string
		actual    string
		want      bool
	}{
		{"gemini-4-argon", "3.5 Flash-Lite", true},
		{"gemini-4-argon", "3.8 Flash", true},
		{"gemini-4-argon", "4 Argon", false},
		{"gemini-4-argon-thinking-high", "3.5 Flash-Lite", true},
		{"gemini-4-argon-thinking-high", "4 Argon", false},
		{"gemini-3.1-pro", "3.5 Flash-Lite", true},
		{"gemini-3.1-pro", "3.8 Flash", true},
		{"gemini-3.1-pro", "3.1 Pro", false},
		{"gemini-3.1-pro", "4 Argon", false}, // Argon is higher/equal tier
		{"gemini-3.8-flash", "3.5 Flash-Lite", true},
		{"gemini-3.8-flash", "3.8 Flash", false},
		{"gemini-3.5-flash-lite", "3.5 Flash-Lite", false},
		{"gemini-fake-model-1", "3.8 Flash", true},
		{"gemini-fake-model-2", "3.5 Flash-Lite", true},
	}

	for _, tt := range tests {
		got := IsModelDowngraded(tt.requested, tt.actual)
		if got != tt.want {
			t.Errorf("IsModelDowngraded(%q, %q) = %v; want %v", tt.requested, tt.actual, got, tt.want)
		}
	}
}

func TestAnomaly37FallbackDetection(t *testing.T) {
	rawAnomaly := `)]}'

36
[["wrb.fr",null,"[{\"37\":[0]}]"]]
5338
[["wrb.fr",null,"[null,[\"c_123\",\"r_456\"],null,null,[[\"rc_789\",[\"Hello\"],null,null,null,null,null,null,[1],\"en\"]]]"]]`

	res := &GeminiResponse{}
	parseResponse(rawAnomaly, res)

	if !res.FallbackDetected {
		t.Errorf("Expected FallbackDetected = true when raw contains {\"37\":[0]}")
	}
	if res.Text != "Hello" {
		t.Errorf("Expected Text = 'Hello', got %q", res.Text)
	}

	rawNormal := `)]}'

5338
[["wrb.fr",null,"[null,[\"c_123\",\"r_456\"],null,null,[[\"rc_789\",[\"Hello Normal\"],null,null,null,null,null,null,[1],\"en\"]]]"]]`

	resNormal := &GeminiResponse{}
	parseResponse(rawNormal, resNormal)
	if resNormal.FallbackDetected {
		t.Errorf("Expected FallbackDetected = false for normal response")
	}
	if resNormal.Text != "Hello Normal" {
		t.Errorf("Expected Text = 'Hello Normal', got %q", resNormal.Text)
	}
}

func TestTemporaryChatFlags(t *testing.T) {
	SetTemporaryChats(true)
	defer SetTemporaryChats(false)

	if !IsTemporaryChatsEnabled() {
		t.Fatalf("Expected IsTemporaryChatsEnabled to be true")
	}

	msgInner := []interface{}{"hi", 0}
	msgLang := []string{"en"}
	msgContext := []interface{}{"", ""}
	msgStruct := []interface{}{msgInner, msgLang, msgContext}

	ApplyTemporaryChatFlags(&msgStruct)

	if len(msgStruct) < 46 {
		t.Fatalf("Expected msgStruct length >= 46, got %d", len(msgStruct))
	}

	// Verify index 41 has [1]
	arr41, ok := msgStruct[41].([]interface{})
	if !ok || len(arr41) != 1 || arr41[0] != 1 {
		t.Errorf("Expected msgStruct[41] to be [1], got %v", msgStruct[41])
	}

	// Verify index 45 has 1
	if msgStruct[45] != 1 {
		t.Errorf("Expected msgStruct[45] to be 1, got %v", msgStruct[45])
	}
}
