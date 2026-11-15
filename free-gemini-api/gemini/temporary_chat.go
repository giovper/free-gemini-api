package gemini

import (
	"os"
	"sync"
)

var (
	temporaryChatsEnabled bool
	temporaryChatsMu      sync.RWMutex
)

// SetTemporaryChats configures whether requests should use temporary (incognito) chat mode
func SetTemporaryChats(enabled bool) {
	temporaryChatsMu.Lock()
	defer temporaryChatsMu.Unlock()
	temporaryChatsEnabled = enabled
}

// IsTemporaryChatsEnabled returns true if temporary chat mode is active
func IsTemporaryChatsEnabled() bool {
	temporaryChatsMu.RLock()
	defer temporaryChatsMu.RUnlock()
	if temporaryChatsEnabled {
		return true
	}
	return os.Getenv("TEMPORARY_CHATS") == "true" || os.Getenv("TEMP") == "true"
}

// ApplyTemporaryChatFlags injects Google Gemini's native temporary chat flags into msgStruct
func ApplyTemporaryChatFlags(msgStruct *[]interface{}) {
	if !IsTemporaryChatsEnabled() {
		return
	}
	// Pad array up to index 45
	for len(*msgStruct) < 46 {
		*msgStruct = append(*msgStruct, nil)
	}
	// inner[41] = [1] indicates temporary chat mode (vs [2] for persistent saved chat)
	(*msgStruct)[41] = []interface{}{1}
	// inner[45] = 1 enables ephemeral conversation flag
	(*msgStruct)[45] = 1
}
