package convo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"rictusd/modules/core"
)

// Message is a single entry in the conversation log.
type Message struct {
	Timestamp string `json:"timestamp"`
	Role      string `json:"role"`   // "user" or "daemon"
	Text      string `json:"text"`
}

// Store writes conversation messages to a JSONL file.
type Store struct {
	core *core.Core
	path string
}

// NewStore creates a new conversation store.
func NewStore(c *core.Core) *Store {
	p := filepath.Join(c.Data, "convo.jsonl")
	return &Store{
		core: c,
		path: p,
	}
}

// Append writes a single message to the log.
func (s *Store) Append(role, text string) {
	if s == nil {
		return
	}

	msg := Message{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Role:      role,
		Text:      text,
	}

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		s.core.Log.Errorf("convo: open file: %v", err)
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(&msg); err != nil {
		s.core.Log.Errorf("convo: encode message: %v", err)
	}
}
