package tasks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"rictusd/modules/core"
)

var ErrNotFound = errors.New("task not found")

// Task is a simple, local task entry managed by RictusD.
type Task struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
	DoneAt    string    `json:"done_at,omitempty"`
}

// Store keeps tasks in a single JSON file under Core.Data.
type Store struct {
	core *core.Core
	path string
	list []Task
}

func NewStore(c *core.Core) *Store {
	s := &Store{
		core: c,
		path: filepath.Join(c.Data, "tasks.json"),
	}
	s.load()
	return s
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.list = nil
			return
		}
		s.core.Log.Errorf("tasks: read %s: %v", s.path, err)
		s.list = nil
		return
	}

	var out []Task
	if err := json.Unmarshal(data, &out); err != nil {
		s.core.Log.Errorf("tasks: decode %s: %v", s.path, err)
		s.list = nil
		return
	}

	s.list = out
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o644)
}

// Add creates a new task with the next numeric ID.
func (s *Store) Add(text string) (*Task, error) {
	nextID := 1
	for _, t := range s.list {
		if t.ID >= nextID {
			nextID = t.ID + 1
		}
	}

	now := time.Now().UTC()
	t := Task{
		ID:        nextID,
		Text:      text,
		Done:      false,
		CreatedAt: now,
	}

	s.list = append(s.list, t)

	if err := s.save(); err != nil {
		return nil, err
	}

	return &t, nil
}

// List returns a sorted snapshot of all tasks.
func (s *Store) List() []Task {
	out := make([]Task, len(s.list))
	copy(out, s.list)

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out
}

// Complete marks a task done by id.
func (s *Store) Complete(id int) error {
	for i := range s.list {
		if s.list[i].ID == id {
			if s.list[i].Done {
				return nil
			}
			s.list[i].Done = true
			s.list[i].DoneAt = time.Now().UTC().Format(time.RFC3339)
			return s.save()
		}
	}
	return ErrNotFound
}
