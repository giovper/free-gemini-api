package gemini

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ModelConfig represents a Gemini model configuration
type ModelConfig struct {
	ID             string   `json:"id"`
	DisplayName    string   `json:"display_name"`
	Description    string   `json:"description,omitempty"`
	ModeID         string   `json:"mode_id"`         // Internal hex hash (e.g. 56fdd199312815e2)
	HeaderTemplate string   `json:"header_template"` // JSPB template string
	ThinkingLevel  int      `json:"thinking_level,omitempty"`
	MinTier        string   `json:"min_tier"` // "free" or "subscription"
	Aliases        []string `json:"aliases,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty"`
	IsDefault      bool     `json:"is_default"`
	Source         string   `json:"source"` // "builtin", "discovered", "custom"
}

// BuildHeaderValue constructs the x-goog-ext-525001261-jspb header value.
//
// Format verified via live CDP capture (2026-10-14):
//
//	[1,null,null,null,null,null,null,null,[caps],null,null,null,null,null,8,<thinkingLevel>,null,null,null,null]
//
// Thinking level sits at index 15:
//
//	Low=1, Medium=4, High=2
//
// For non-thinking models the classic short format is used:
//
//	[1,null,null,null,"<modeID>",null,null,0,[caps],null,null,2]
func (m *ModelConfig) BuildHeaderValue() string {
	if m.HeaderTemplate != "" {
		s := m.HeaderTemplate
		if strings.Contains(s, "%s") && strings.Contains(s, "%d") {
			lvl := m.ThinkingLevel
			if lvl < 0 {
				lvl = 1
			}
			return fmt.Sprintf(s, m.ModeID, lvl)
		}
		if strings.Contains(s, "%d") {
			lvl := m.ThinkingLevel
			if lvl < 0 {
				lvl = 1
			}
			return fmt.Sprintf(s, lvl)
		}
		if strings.Contains(s, "%s") {
			return fmt.Sprintf(s, m.ModeID)
		}
		return s
	}
	if m.ThinkingLevel > 0 {
		// Argon-style thinking format with ModeID at index 4 and thinking level at index 15
		return fmt.Sprintf(`[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`, m.ModeID, m.ThinkingLevel)
	}
	// Default non-thinking format with modeID
	return fmt.Sprintf(`[1,null,null,null,"%s",null,null,0,[4],null,null,2]`, m.ModeID)
}

// ModelRegistry manages the thread-safe collection of available models
type ModelRegistry struct {
	mu           sync.RWMutex
	models       map[string]*ModelConfig // ID (lowercase) -> ModelConfig
	aliasMap     map[string]string       // Alias (lowercase) -> canonical ID
	defaultModel string
	storagePath  string
}

var (
	globalModelRegistry *ModelRegistry
	registryOnce        sync.Once
)

// Default models catalog
var DefaultBuiltinModels = []ModelConfig{
	// ─── Gemini 4 Series (Frontier Intelligence) ──────────────────────────────
	// NOTE: Gemini 4 Carbon è disponibile solo per account Ultra.
	// Il suo modeID non è stato catturato (non accessibile da account Pro).
	// Se in futuro ottieni accesso Ultra, cattura l'hash con HAR Export e aggiungilo qui.

	// NOTE: Dal HAR catturato il 2026-11-15, Google Web usa la stringa "argon"
	// come modeID. L'hash legacy "95a4c0abcba24364" è mantenuto come alias.
	{
		ID:             "gemini-4-argon",
		DisplayName:    "Google Gemini 4 Argon",
		Description:    "Complex tasks — next-generation frontier intelligence (Medium=4)",
		ModeID:         "argon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4, // Default Level: medium
		MinTier:        "auth",
		Aliases:        []string{"4-argon", "gemini-argon", "argon", "gemini-4", "4", "95a4c0abcba24364", "f44d2958d1f0189a"},
		Capabilities:   []string{"chat", "stream", "vision", "tools", "complex_tasks", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-argon-thinking-low",
		DisplayName:    "Google Gemini 4 Argon (Thinking Low)",
		Description:    "Complex tasks with quick and efficient thinking (Low=1)",
		ModeID:         "argon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "auth",
		Aliases:        []string{"gemini-4-argon-low", "argon-thinking-low", "argon-low", "thinking-low"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "complex_tasks"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-argon-thinking-medium",
		DisplayName:    "Google Gemini 4 Argon (Thinking Medium)",
		Description:    "Complex tasks with balanced thinking depth (Medium=4)",
		ModeID:         "argon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "auth",
		Aliases:        []string{"gemini-4-argon-medium", "argon-thinking-medium", "argon-medium", "thinking-medium"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "complex_tasks"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-argon-thinking-high",
		DisplayName:    "Google Gemini 4 Argon (Thinking High)",
		Description:    "Complex tasks with extra thorough thinking depth (High=2)",
		ModeID:         "argon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  2,
		MinTier:        "auth",
		Aliases:        []string{"gemini-4-argon-high", "argon-thinking-high", "argon-high", "thinking-high"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "complex_tasks"},
		IsDefault:      false,
		Source:         "builtin",
	},

	// NOTE: Il modeID di Hydrogen "56fdd199312815e2" è stato catturato dal HAR il 2026-11-15.
	// È lo stesso hash usato anche da Flash 3.8 — Google potrebbe condividere l'infrastruttura.
	{
		ID:             "gemini-4-hydrogen",
		DisplayName:    "Google Gemini 4 Hydrogen",
		Description:    "Everyday help — fast and efficient daily driver (High=2)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  2, // Default Level: high
		MinTier:        "free",
		Aliases:        []string{"4-hydrogen", "gemini-hydrogen", "hydrogen"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools", "complex_architecture", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-hydrogen-thinking-low",
		DisplayName:    "Google Gemini 4 Hydrogen (Thinking Low)",
		Description:    "Extreme reasoning with quick thinking pass (Low=1)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "free",
		Aliases:        []string{"gemini-4-hydrogen-low", "hydrogen-thinking-low", "hydrogen-low"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-hydrogen-thinking-medium",
		DisplayName:    "Google Gemini 4 Hydrogen (Thinking Medium)",
		Description:    "Extreme reasoning with balanced depth (Medium=4)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "free",
		Aliases:        []string{"gemini-4-hydrogen-medium", "hydrogen-thinking-medium", "hydrogen-medium"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-hydrogen-thinking-high",
		DisplayName:    "Google Gemini 4 Hydrogen (Thinking High)",
		Description:    "Extreme reasoning with thorough depth (High=2)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  2,
		MinTier:        "free",
		Aliases:        []string{"gemini-4-hydrogen-high", "hydrogen-thinking-high", "hydrogen-high"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-hydrogen-thinking-xhigh",
		DisplayName:    "Google Gemini 4 Hydrogen (Thinking Extra High)",
		Description:    "Deepest tier of multi-step reasoning & long-horizon planning (xHigh=8)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  8,
		MinTier:        "free",
		Aliases:        []string{"gemini-4-hydrogen-xhigh", "hydrogen-thinking-xhigh", "hydrogen-xhigh"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},

	// NOTE: Dal HAR catturato il 2026-11-21 (net-log-gemini-neon.har), Google Web identifica
	// Gemini 4 Neon con il modeID hash "e6fa609c3fa255c0" e la stringa simbolica "neon",
	// con family index 3 nel protocollo JSPB. Supporta tutti i livelli di deep reasoning.
	{
		ID:             "gemini-4-neon",
		DisplayName:    "Google Gemini 4 Neon",
		Description:    "Cutting-edge frontier intelligence with dynamic adaptive reasoning (Medium=4)",
		ModeID:         "neon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  4, // Default Level: medium
		MinTier:        "auth",
		Aliases:        []string{"4-neon", "gemini-neon", "neon", "Neon4p0", "e6fa609c3fa255c0"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools", "complex_tasks", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-neon-thinking-minimal",
		DisplayName:    "Google Gemini 4 Neon (Thinking Minimal)",
		Description:    "Frontier intelligence with instant, low-latency reasoning (Minimal=0)",
		ModeID:         "neon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  0,
		MinTier:        "auth",
		Aliases:        []string{"gemini-4-neon-minimal", "neon-thinking-minimal", "neon-minimal"},
		Capabilities:   []string{"chat", "stream", "thinking", "fast", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-neon-thinking-low",
		DisplayName:    "Google Gemini 4 Neon (Thinking Low)",
		Description:    "Frontier intelligence with quick and efficient reasoning (Low=1)",
		ModeID:         "neon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "auth",
		Aliases:        []string{"gemini-4-neon-low", "neon-thinking-low", "neon-low"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-neon-thinking-medium",
		DisplayName:    "Google Gemini 4 Neon (Thinking Medium)",
		Description:    "Frontier intelligence with balanced depth reasoning (Medium=4)",
		ModeID:         "neon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "auth",
		Aliases:        []string{"gemini-4-neon-medium", "neon-thinking-medium", "neon-medium"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "complex_tasks", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-neon-thinking-high",
		DisplayName:    "Google Gemini 4 Neon (Thinking High)",
		Description:    "Frontier intelligence with deep, multi-step analytical reasoning (High=2)",
		ModeID:         "neon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  2,
		MinTier:        "auth",
		Aliases:        []string{"gemini-4-neon-high", "neon-thinking-high", "neon-high"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "complex_tasks", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-4-neon-thinking-xhigh",
		DisplayName:    "Google Gemini 4 Neon (Thinking XHigh)",
		Description:    "Frontier intelligence with maximum depth reasoning exploration (XHigh=8)",
		ModeID:         "neon",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  8,
		MinTier:        "auth",
		Aliases:        []string{"gemini-4-neon-xhigh", "neon-thinking-xhigh", "neon-xhigh"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "complex_tasks", "frontier"},
		IsDefault:      false,
		Source:         "builtin",
	},

	// ─── Fake Models (per testare cosa succede con modeID non validi) ──────────
	{
		ID:             "gemini-fake-model-1",
		DisplayName:    "FAKE Model 1 (Hash Inventato)",
		Description:    "Test model con un hash hex inventato — per verificare la risposta di Google a un modeID inesistente",
		ModeID:         "aaaaaaaaaaaaaaaa",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "free",
		Aliases:        []string{"fake-1", "fake-hash"},
		Capabilities:   []string{"chat", "test"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-fake-model-2",
		DisplayName:    "FAKE Model 2 (Nome Inventato)",
		Description:    "Test model con un nome stringa inventato — per verificare la risposta di Google a un modello inesistente",
		ModeID:         "gemini-6-nonexisting",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "free",
		Aliases:        []string{"fake-2", "fake-name"},
		Capabilities:   []string{"chat", "test"},
		IsDefault:      false,
		Source:         "builtin",
	},

	// ─── Gemini 3.x Series (Text & Multimodal) ────────────────────────────────
	{
		ID:             "gemini-3.8-flash",
		DisplayName:    "Google Gemini 3.8 Flash",
		Description:    "All-around help — balanced multi-modal flagship (Medium=4)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4, // Default Level: medium
		MinTier:        "free",
		Aliases:        []string{"3.8-flash", "gemini-flash", "flash", "default"},
		Capabilities:   []string{"chat", "stream", "vision", "video", "tools"},
		IsDefault:      true,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.8-flash-thinking-low",
		DisplayName:    "Google Gemini 3.8 Flash (Thinking Low)",
		Description:    "Fast reasoning with quick thinking (Low=1)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.8-flash-low", "flash-thinking-low", "flash-low", "3.8-flash-low"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.8-flash-thinking-medium",
		DisplayName:    "Google Gemini 3.8 Flash (Thinking Medium)",
		Description:    "Fast reasoning with balanced thinking depth (Medium=4)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.8-flash-medium", "flash-thinking-medium", "flash-medium", "3.8-flash-medium"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.8-flash-thinking-high",
		DisplayName:    "Google Gemini 3.8 Flash (Thinking High)",
		Description:    "Fast reasoning with extra thorough thinking depth (High=2)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  2,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.8-flash-high", "flash-thinking-high", "flash-high", "3.8-flash-high"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code"},
		IsDefault:      false,
		Source:         "builtin",
	},

	{
		ID:             "gemini-3.7-flash",
		DisplayName:    "Google Gemini 3.7 Flash",
		Description:    "Advanced hybrid reasoning & multimodal intelligence (Medium=4)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4, // Default Level: medium
		MinTier:        "free",
		Aliases:        []string{"3.7-flash", "gemini-3.7"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.7-flash-thinking-low",
		DisplayName:    "Google Gemini 3.7 Flash (Thinking Low)",
		Description:    "Hybrid reasoning with quick thinking (Low=1)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.7-flash-low", "3.7-flash-low"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.7-flash-thinking-medium",
		DisplayName:    "Google Gemini 3.7 Flash (Thinking Medium)",
		Description:    "Hybrid reasoning with balanced depth (Medium=4)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.7-flash-medium", "3.7-flash-medium"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.7-flash-thinking-high",
		DisplayName:    "Google Gemini 3.7 Flash (Thinking High)",
		Description:    "Hybrid reasoning with deep deliberation (High=2)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  2,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.7-flash-high", "3.7-flash-high"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code"},
		IsDefault:      false,
		Source:         "builtin",
	},

	{
		ID:             "gemini-3.6-flash",
		DisplayName:    "Google Gemini 3.6 Flash",
		Description:    "Granular reasoning & high-efficiency multimodal (Medium=4)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4, // Default Level: medium
		MinTier:        "free",
		Aliases:        []string{"3.6-flash", "gemini-3.6"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.6-flash-thinking-minimal",
		DisplayName:    "Google Gemini 3.6 Flash (Thinking Minimal)",
		Description:    "Near-zero deliberation for minimal latency (Minimal=0)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,0,[4],null,null,2]`,
		ThinkingLevel:  0,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.6-flash-minimal", "3.6-flash-minimal"},
		Capabilities:   []string{"chat", "stream", "vision", "tools", "fastest"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.6-flash-thinking-low",
		DisplayName:    "Google Gemini 3.6 Flash (Thinking Low)",
		Description:    "Lightweight reasoning pass (Low=1)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.6-flash-low", "3.6-flash-low"},
		Capabilities:   []string{"chat", "stream", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.6-flash-thinking-medium",
		DisplayName:    "Google Gemini 3.6 Flash (Thinking Medium)",
		Description:    "Balanced reasoning depth (Medium=4)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.6-flash-medium", "3.6-flash-medium"},
		Capabilities:   []string{"chat", "stream", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.6-flash-thinking-high",
		DisplayName:    "Google Gemini 3.6 Flash (Thinking High)",
		Description:    "Maximum deliberation & deep chain-of-thought (High=2)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  2,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.6-flash-high", "3.6-flash-high"},
		Capabilities:   []string{"chat", "stream", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},

	{
		ID:             "gemini-3.5-flash",
		DisplayName:    "Google Gemini 3.5 Flash",
		Description:    "Fast general-purpose assistant (Medium=4)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4, // Default Level: medium
		MinTier:        "free",
		Aliases:        []string{"3.5-flash", "gemini-3.5"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.5-flash-thinking-minimal",
		DisplayName:    "Google Gemini 3.5 Flash (Thinking Minimal)",
		Description:    "Near-zero deliberation for minimal latency (Minimal=0)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,0,[4],null,null,2]`,
		ThinkingLevel:  0,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.5-flash-minimal", "3.5-flash-minimal"},
		Capabilities:   []string{"chat", "stream", "vision", "tools", "fastest"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.5-flash-thinking-low",
		DisplayName:    "Google Gemini 3.5 Flash (Thinking Low)",
		Description:    "Lightweight reasoning pass (Low=1)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.5-flash-low", "3.5-flash-low"},
		Capabilities:   []string{"chat", "stream", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.5-flash-thinking-medium",
		DisplayName:    "Google Gemini 3.5 Flash (Thinking Medium)",
		Description:    "Balanced reasoning depth (Medium=4)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.5-flash-medium", "3.5-flash-medium"},
		Capabilities:   []string{"chat", "stream", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.5-flash-thinking-high",
		DisplayName:    "Google Gemini 3.5 Flash (Thinking High)",
		Description:    "Maximum deliberation & deep chain-of-thought (High=2)",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  2,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.5-flash-high", "3.5-flash-high"},
		Capabilities:   []string{"chat", "stream", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},

	{
		ID:             "gemini-3.5-flash-lite",
		DisplayName:    "Google Gemini 3.5 Flash-Lite",
		Description:    "Fastest answers — ultra low latency quick answers (Minimal=0)",
		ModeID:         "8c46e95b1a07cecc",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,0,[4],null,null,2]`,
		ThinkingLevel:  0, // Default Level: minimal
		MinTier:        "free",
		Aliases:        []string{"3.5-flash-lite", "flash-lite", "gemini-flash-lite", "gemini-lite", "lite", "fbb127bbb056c959"},
		Capabilities:   []string{"chat", "stream", "vision", "tools", "fastest"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.5-flash-lite-thinking-minimal",
		DisplayName:    "Google Gemini 3.5 Flash-Lite (Thinking Minimal)",
		Description:    "Ultra-low latency with near-zero thinking tokens (Minimal=0)",
		ModeID:         "8c46e95b1a07cecc",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,0,[4],null,null,2]`,
		ThinkingLevel:  0,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.5-flash-lite-minimal", "flash-lite-minimal"},
		Capabilities:   []string{"chat", "stream", "vision", "tools", "fastest"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.5-flash-lite-thinking-low",
		DisplayName:    "Google Gemini 3.5 Flash-Lite (Thinking Low)",
		Description:    "Ultra-lightweight reasoning on flash-lite (Low=1)",
		ModeID:         "8c46e95b1a07cecc",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.5-flash-lite-low", "flash-lite-low"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools", "fastest"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.5-flash-lite-thinking-medium",
		DisplayName:    "Google Gemini 3.5 Flash-Lite (Thinking Medium)",
		Description:    "Balanced reasoning depth on flash-lite (Medium=4)",
		ModeID:         "8c46e95b1a07cecc",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  4,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.5-flash-lite-medium", "flash-lite-medium"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.5-flash-lite-thinking-high",
		DisplayName:    "Google Gemini 3.5 Flash-Lite (Thinking High)",
		Description:    "Deep deliberation on flash-lite (High=2)",
		ModeID:         "8c46e95b1a07cecc",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  2,
		MinTier:        "free",
		Aliases:        []string{"gemini-3.5-flash-lite-high", "flash-lite-high"},
		Capabilities:   []string{"chat", "stream", "vision", "thinking", "tools"},
		IsDefault:      false,
		Source:         "builtin",
	},

	// ─── Gemini 3.1 Pro (ModeID: e6fa609c3fa255c0, idx 14 = 3 verified via live HAR) ──────
	{
		ID:             "gemini-3.1-pro",
		DisplayName:    "Google Gemini 3.1 Pro",
		Description:    "Advanced reasoning — complex coding and multi-turn problem solving",
		ModeID:         "e6fa609c3fa255c0",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "auth",
		Aliases:        []string{"3.1-pro", "gemini-pro", "pro", "gemini-3.0-pro", "gemini-2.5-pro", "advanced", "61530e79959ab139"},
		Capabilities:   []string{"chat", "stream", "vision", "tools", "complex_reasoning"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.1-pro-thinking-high",
		DisplayName:    "Google Gemini 3.1 Pro (Thinking High)",
		Description:    "Pro reasoning with extra thorough thinking depth (High=2)",
		ModeID:         "e6fa609c3fa255c0",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  2,
		MinTier:        "auth",
		Aliases:        []string{"pro-thinking-high", "pro-high"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "complex_reasoning"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.1-pro-thinking-medium",
		DisplayName:    "Google Gemini 3.1 Pro (Thinking Medium)",
		Description:    "Pro reasoning with balanced thinking depth (Standard Thinking)",
		ModeID:         "e6fa609c3fa255c0",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  1, // Note: Pro upstream only accepts 1 (standard) and 2 (high). Level 4 triggers anomaly 37.
		MinTier:        "auth",
		Aliases:        []string{"pro-thinking-medium", "pro-medium"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "complex_reasoning"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.1-pro-thinking-low",
		DisplayName:    "Google Gemini 3.1 Pro (Thinking Low)",
		Description:    "Pro reasoning with quick thinking (Low=1)",
		ModeID:         "e6fa609c3fa255c0",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "auth",
		Aliases:        []string{"pro-thinking-low", "pro-low"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code", "complex_reasoning"},
		IsDefault:      false,
		Source:         "builtin",
	},
	{
		ID:             "gemini-3.8-thinking",
		DisplayName:    "Google Gemini 3.8 Flash Thinking",
		Description:    "Deep reasoning with step-by-step thinking scratchpad trace",
		ModeID:         "56fdd199312815e2",
		HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,1,%d,null,null,null,null]`,
		ThinkingLevel:  1,
		MinTier:        "free",
		Aliases:        []string{"gemini-thinking", "flash-thinking", "thinking"},
		Capabilities:   []string{"chat", "stream", "thinking", "math", "code"},
		IsDefault:      false,
		Source:         "builtin",
	},
}

// GetModelRegistry returns the singleton ModelRegistry
var defaultBuiltinModels = DefaultBuiltinModels

func GetModelRegistry() *ModelRegistry {
	registryOnce.Do(func() {
		globalModelRegistry = &ModelRegistry{
			models:       make(map[string]*ModelConfig),
			aliasMap:     make(map[string]string),
			defaultModel: "gemini-3.8-flash",
			storagePath:  "models.json",
		}

		// Register built-in models
		for _, m := range defaultBuiltinModels {
			mCopy := m
			globalModelRegistry.registerInternal(&mCopy)
		}

		// Load custom or discovered models from disk
		globalModelRegistry.LoadFromFile()
	})
	return globalModelRegistry
}

func (r *ModelRegistry) registerInternal(m *ModelConfig) {
	key := strings.ToLower(strings.TrimSpace(m.ID))
	r.models[key] = m

	for _, alias := range m.Aliases {
		aKey := strings.ToLower(strings.TrimSpace(alias))
		if aKey != "" {
			r.aliasMap[aKey] = key
		}
	}

	if m.IsDefault {
		r.defaultModel = key
	}
}

// Register adds or updates a model in the registry and persists to disk
func (r *ModelRegistry) Register(m ModelConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.registerInternal(&m)
	_ = r.saveToFileInternal()
	log.Printf("🤖 [ModelRegistry] Registered model: %s (%s, ModeID: %s)", m.ID, m.DisplayName, m.ModeID)
}

// Unregister removes a model from the registry and updates disk persistence
func (r *ModelRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := strings.ToLower(strings.TrimSpace(id))
	delete(r.models, key)
	for a, k := range r.aliasMap {
		if k == key {
			delete(r.aliasMap, a)
		}
	}
	_ = r.saveToFileInternal()
}

// Resolve looks up a model by ID or alias, falling back to default model
func (r *ModelRegistry) Resolve(name string) *ModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clean := strings.ToLower(strings.TrimSpace(name))
	if clean == "" {
		if def, ok := r.models[r.defaultModel]; ok {
			return def
		}
		// Fallback to first available
		for _, m := range r.models {
			return m
		}
	}

	// 1. Direct ID match
	if m, ok := r.models[clean]; ok {
		return m
	}

	// 2. Direct Alias match
	if canonicalID, ok := r.aliasMap[clean]; ok {
		if m, ok := r.models[canonicalID]; ok {
			return m
		}
	}

	// 3. Substring / Semantic matching for convenience
	hasXHigh := strings.Contains(clean, "xhigh") || strings.Contains(clean, "extra-high") || strings.Contains(clean, "max")
	hasHigh := strings.Contains(clean, "high") && !hasXHigh
	hasLow := strings.Contains(clean, "low") || strings.Contains(clean, "fast")
	hasMinimal := strings.Contains(clean, "minimal") || strings.Contains(clean, "min") || strings.HasSuffix(clean, "-0")
	hasMed := strings.Contains(clean, "medium") || strings.Contains(clean, "med")

	// Gemini 4 Family

	if strings.Contains(clean, "hydrogen") {
		if hasXHigh {
			if m, ok := r.models["gemini-4-hydrogen-thinking-xhigh"]; ok {
				return m
			}
		}
		if hasHigh {
			if m, ok := r.models["gemini-4-hydrogen-thinking-high"]; ok {
				return m
			}
		}
		if hasLow {
			if m, ok := r.models["gemini-4-hydrogen-thinking-low"]; ok {
				return m
			}
		}
		if hasMed {
			if m, ok := r.models["gemini-4-hydrogen-thinking-medium"]; ok {
				return m
			}
		}
		if m, ok := r.models["gemini-4-hydrogen"]; ok {
			return m
		}
	}

	if strings.Contains(clean, "argon") {
		if hasHigh {
			if m, ok := r.models["gemini-4-argon-thinking-high"]; ok {
				return m
			}
		}
		if hasLow {
			if m, ok := r.models["gemini-4-argon-thinking-low"]; ok {
				return m
			}
		}
		if hasMed {
			if m, ok := r.models["gemini-4-argon-thinking-medium"]; ok {
				return m
			}
		}
		if m, ok := r.models["gemini-4-argon"]; ok {
			return m
		}
	}

	if strings.Contains(clean, "neon") {
		if hasXHigh {
			if m, ok := r.models["gemini-4-neon-thinking-xhigh"]; ok {
				return m
			}
		}
		if hasHigh {
			if m, ok := r.models["gemini-4-neon-thinking-high"]; ok {
				return m
			}
		}
		if hasLow {
			if m, ok := r.models["gemini-4-neon-thinking-low"]; ok {
				return m
			}
		}
		if hasMinimal {
			if m, ok := r.models["gemini-4-neon-thinking-minimal"]; ok {
				return m
			}
		}
		if hasMed {
			if m, ok := r.models["gemini-4-neon-thinking-medium"]; ok {
				return m
			}
		}
		if m, ok := r.models["gemini-4-neon"]; ok {
			return m
		}
	}

	// Gemini 3.x Flash Family
	if strings.Contains(clean, "3.7") {
		if hasHigh {
			if m, ok := r.models["gemini-3.7-flash-thinking-high"]; ok {
				return m
			}
		}
		if hasLow {
			if m, ok := r.models["gemini-3.7-flash-thinking-low"]; ok {
				return m
			}
		}
		if hasMed {
			if m, ok := r.models["gemini-3.7-flash-thinking-medium"]; ok {
				return m
			}
		}
		if m, ok := r.models["gemini-3.7-flash"]; ok {
			return m
		}
	}

	if strings.Contains(clean, "3.6") {
		if hasMinimal {
			if m, ok := r.models["gemini-3.6-flash-thinking-minimal"]; ok {
				return m
			}
		}
		if hasHigh {
			if m, ok := r.models["gemini-3.6-flash-thinking-high"]; ok {
				return m
			}
		}
		if hasLow {
			if m, ok := r.models["gemini-3.6-flash-thinking-low"]; ok {
				return m
			}
		}
		if hasMed {
			if m, ok := r.models["gemini-3.6-flash-thinking-medium"]; ok {
				return m
			}
		}
		if m, ok := r.models["gemini-3.6-flash"]; ok {
			return m
		}
	}

	if strings.Contains(clean, "3.5") && (strings.Contains(clean, "lite") || strings.Contains(clean, "flash-lite")) {
		if hasMinimal {
			if m, ok := r.models["gemini-3.5-flash-lite-thinking-minimal"]; ok {
				return m
			}
		}
		if hasHigh {
			if m, ok := r.models["gemini-3.5-flash-lite-thinking-high"]; ok {
				return m
			}
		}
		if hasLow {
			if m, ok := r.models["gemini-3.5-flash-lite-thinking-low"]; ok {
				return m
			}
		}
		if hasMed {
			if m, ok := r.models["gemini-3.5-flash-lite-thinking-medium"]; ok {
				return m
			}
		}
		if m, ok := r.models["gemini-3.5-flash-lite"]; ok {
			return m
		}
	}

	if strings.Contains(clean, "3.5") {
		if hasMinimal {
			if m, ok := r.models["gemini-3.5-flash-thinking-minimal"]; ok {
				return m
			}
		}
		if hasHigh {
			if m, ok := r.models["gemini-3.5-flash-thinking-high"]; ok {
				return m
			}
		}
		if hasLow {
			if m, ok := r.models["gemini-3.5-flash-thinking-low"]; ok {
				return m
			}
		}
		if hasMed {
			if m, ok := r.models["gemini-3.5-flash-thinking-medium"]; ok {
				return m
			}
		}
		if m, ok := r.models["gemini-3.5-flash"]; ok {
			return m
		}
	}

	if strings.Contains(clean, "lite") {
		if hasMinimal {
			if m, ok := r.models["gemini-3.5-flash-lite-thinking-minimal"]; ok {
				return m
			}
		}
		if hasHigh {
			if m, ok := r.models["gemini-3.5-flash-lite-thinking-high"]; ok {
				return m
			}
		}
		if hasLow {
			if m, ok := r.models["gemini-3.5-flash-lite-thinking-low"]; ok {
				return m
			}
		}
		if hasMed {
			if m, ok := r.models["gemini-3.5-flash-lite-thinking-medium"]; ok {
				return m
			}
		}
		if m, ok := r.models["gemini-3.5-flash-lite"]; ok {
			return m
		}
	}

	if strings.Contains(clean, "pro") {
		if canonicalID, ok := r.aliasMap["pro"]; ok {
			if m, ok := r.models[canonicalID]; ok {
				return m
			}
		}
	}
	if strings.Contains(clean, "thinking") {
		if canonicalID, ok := r.aliasMap["thinking"]; ok {
			if m, ok := r.models[canonicalID]; ok {
				return m
			}
		}
	}
	if strings.Contains(clean, "3.8") || strings.Contains(clean, "flash") {
		if hasHigh {
			if m, ok := r.models["gemini-3.8-flash-thinking-high"]; ok {
				return m
			}
		}
		if hasLow {
			if m, ok := r.models["gemini-3.8-flash-thinking-low"]; ok {
				return m
			}
		}
		if hasMed {
			if m, ok := r.models["gemini-3.8-flash-thinking-medium"]; ok {
				return m
			}
		}
		if m, ok := r.models["gemini-3.8-flash"]; ok {
			return m
		}
	}

	// Fallback to default
	if def, ok := r.models[r.defaultModel]; ok {
		return def
	}
	return &defaultBuiltinModels[0]
}

// Get looks up a model by ID or alias without fallback
func (r *ModelRegistry) Get(name string) (*ModelConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clean := strings.ToLower(strings.TrimSpace(name))
	if m, ok := r.models[clean]; ok {
		return m, true
	}
	if canonicalID, ok := r.aliasMap[clean]; ok {
		if m, ok := r.models[canonicalID]; ok {
			return m, true
		}
	}
	return nil, false
}

// GetAll returns a list of all registered models
func (r *ModelRegistry) GetAll() []ModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]ModelConfig, 0, len(r.models))
	for _, m := range r.models {
		list = append(list, *m)
	}
	return list
}

// LoadFromFile loads custom and discovered models from models.json
func (r *ModelRegistry) LoadFromFile() {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(r.storagePath)
	if err != nil {
		altPath := filepath.Join("gemini", r.storagePath)
		if altData, altErr := os.ReadFile(altPath); altErr == nil {
			data = altData
		} else {
			return
		}
	}

	var customModels []ModelConfig
	if err := json.Unmarshal(data, &customModels); err != nil {
		log.Printf("⚠️ [ModelRegistry] Failed to parse %s: %v", r.storagePath, err)
		return
	}

	count := 0
	for _, m := range customModels {
		mCopy := m
		r.registerInternal(&mCopy)
		count++
	}
	if count > 0 {
		log.Printf("🤖 [ModelRegistry] Loaded %d custom/discovered models from %s", count, r.storagePath)
	}
}

func (r *ModelRegistry) saveToFileInternal() error {
	var toSave []ModelConfig
	for _, m := range r.models {
		// Save non-builtin or customized models
		if m.Source != "builtin" {
			toSave = append(toSave, *m)
		}
	}

	if len(toSave) == 0 {
		return nil
	}

	_ = os.MkdirAll(filepath.Dir(r.storagePath), 0755)
	data, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.storagePath, data, 0644)
}

// SaveToFile exports the current registry to a specific file path
func (r *ModelRegistry) SaveToFile(filePath string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []ModelConfig
	for _, m := range r.models {
		all = append(all, *m)
	}

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}
