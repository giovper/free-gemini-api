package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	loggedDowngradesMu sync.Mutex
	loggedDowngrades   = make(map[string]int64)
)

// IsModelDowngraded checks if the actual model returned by Google upstream
// represents a downgrade from what was requested.
func IsModelDowngraded(requestedModel, actualModel string) bool {
	if actualModel == "" || requestedModel == "" {
		return false
	}

	req := strings.ToLower(strings.TrimSpace(requestedModel))
	act := strings.ToLower(strings.TrimSpace(actualModel))

	// If requested fake diagnostic models: always flag as fallback
	if strings.Contains(req, "fake") {
		return true
	}

	// If requested Gemini 4 models (Argon, Hydrogen, Neon):
	// Google upstream must return a model in the Gemini 4 family or matching the requested tier
	if strings.Contains(req, "argon") {
		return !strings.Contains(act, "argon") && !strings.Contains(act, "hydrogen") && !strings.Contains(act, "neon") && !strings.Contains(act, "gemini 4")
	}
	if strings.Contains(req, "hydrogen") {
		return !strings.Contains(act, "hydrogen") && !strings.Contains(act, "argon") && !strings.Contains(act, "neon") && !strings.Contains(act, "gemini 4")
	}
	if strings.Contains(req, "neon") {
		return !strings.Contains(act, "neon") && !strings.Contains(act, "argon") && !strings.Contains(act, "hydrogen") && !strings.Contains(act, "gemini 4")
	}

	// If requested Gemini 3.1 Pro (or Pro variant):
	// Google upstream must return a model mentioning "pro" or "argon"
	if strings.Contains(req, "pro") {
		return !strings.Contains(act, "pro") && !strings.Contains(act, "argon")
	}

	// If requested Flash full edition (3.8, 3.7, 3.6, 3.5 Flash):
	// Upstream returning "lite" or "flash-lite" is a downgrade
	if (strings.Contains(req, "3.8") || strings.Contains(req, "3.7") || strings.Contains(req, "3.6") || strings.Contains(req, "3.5")) && strings.Contains(req, "flash") && !strings.Contains(req, "lite") {
		if strings.Contains(act, "lite") {
			return true
		}
	} else if strings.Contains(req, "flash") && !strings.Contains(req, "lite") {
		if strings.Contains(act, "lite") {
			return true
		}
	}

	return false
}

// LogModelDowngradeWarning prints a prominent red warning in the console when a model downgrade or fallback is detected.
func LogModelDowngradeWarning(requestedModel, actualModel, workerSource string) {
	red := "\033[1;31m"
	reset := "\033[0m"
	bold := "\033[1m"
	yellow := "\033[1;33m"

	accountName := "default"
	if workerSource != "" {
		accountName = filepath.Base(workerSource)
	}

	note := "Google silently downgraded your request (tier insufficient or session expired)."
	if strings.Contains(actualModel, "37") {
		note = "Google rejected the ModeID (anomaly {\"37\":[0]}) and fell back to standard engine."
	}

	fmt.Fprintf(os.Stderr, "\n%s╔════════════════════════════════════════════════════════════════════════════════════════╗%s\n", red, reset)
	fmt.Fprintf(os.Stderr, "%s║  ⚠️  [FALLBACK / DOWNGRADE WARNING] DETECTED IN GOOGLE UPSTREAM                        ║%s\n", red, reset)
	fmt.Fprintf(os.Stderr, "%s╠════════════════════════════════════════════════════════════════════════════════════════╣%s\n", red, reset)
	fmt.Fprintf(os.Stderr, "%s║%s  Requested Model:  %-66s%s║%s\n", red, bold, requestedModel, red, reset)
	fmt.Fprintf(os.Stderr, "%s║%s  Upstream State:   %-66s%s║%s\n", red, yellow, actualModel+" (FALLBACK)", red, reset)
	fmt.Fprintf(os.Stderr, "%s║%s  Worker Account:   %-66s%s║%s\n", red, bold, accountName, red, reset)
	fmt.Fprintf(os.Stderr, "%s║  Note: %-79s ║%s\n", red, note, reset)
	fmt.Fprintf(os.Stderr, "%s╚════════════════════════════════════════════════════════════════════════════════════════╝%s\n\n", red, reset)
}
