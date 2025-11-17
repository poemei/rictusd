package brain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"rictusd/modules/core"
)

// Event is a single structured brain entry.
type Event struct {
	Timestamp string `json:"timestamp"`
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	Message   string `json:"message"`
}

// Brain appends events to a JSONL file in data/.
type Brain struct {
	core *core.Core
	path string
}

// NewBrain creates a new Brain tied to the daemon's data directory.
func NewBrain(c *core.Core) *Brain {
	p := filepath.Join(c.Data, "events.jsonl")
	return &Brain{
		core: c,
		path: p,
	}
}

// Record appends an event with the current timestamp.
func (b *Brain) Record(kind, source, msg string) {
	if b == nil {
		return
	}

	ev := Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Kind:      kind,
		Source:    source,
		Message:   msg,
	}

	f, err := os.OpenFile(b.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		b.core.Log.Errorf("brain: open events file: %v", err)
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(&ev); err != nil {
		b.core.Log.Errorf("brain: encode event: %v", err)
	}
}
