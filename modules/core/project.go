package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Project represents a single registered project RictusD is allowed to inspect.
type Project struct {
	Name         string   `json:"name"`
	Path         string   `json:"path"`
	Type         string   `json:"type"`          // e.g., "php", "mixed", "unknown"
	Languages    []string `json:"languages"`     // optional, can be filled in later
	RegisteredAt string   `json:"registered_at"` // RFC3339
}

// ProjectRegistry manages the list of registered projects on disk.
// It is intentionally decoupled from any internal Core type and only needs
// the daemon's data directory.
type ProjectRegistry struct {
	dataDir  string
	filePath string
	projects []Project
}

// NewProjectRegistry creates a registry bound to the given data directory.
// It will attempt to load any existing registry from disk but will not treat
// a missing file as an error.
func NewProjectRegistry(dataDir string) *ProjectRegistry {
	path := filepath.Join(dataDir, "projects.json")
	r := &ProjectRegistry{
		dataDir:  dataDir,
		filePath: path,
		projects: make([]Project, 0),
	}

	if err := r.load(); err != nil {
		// We do not have a logger here on purpose; keep it simple and silent.
		fmt.Printf("project registry: load failed: %v\n", err)
	}

	return r
}

// load reads the projects file if it exists.
func (r *ProjectRegistry) load() error {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No registry yet is fine; start empty.
			return nil
		}
		return fmt.Errorf("read projects.json: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	var list []Project
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("decode projects.json: %w", err)
	}

	r.projects = list
	return nil
}

// save writes the current registry to disk.
func (r *ProjectRegistry) save() error {
	tmp := r.filePath + ".tmp"

	data, err := json.MarshalIndent(r.projects, "", "  ")
	if err != nil {
		return fmt.Errorf("encode projects.json: %w", err)
	}

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp projects.json: %w", err)
	}

	if err := os.Rename(tmp, r.filePath); err != nil {
		return fmt.Errorf("rename projects.json: %w", err)
	}

	return nil
}

// Register registers a new project at the given path.
// It verifies the path exists and is a directory, creates a simple project
// record, and persists it to disk. If a project with the same path already
// exists, it simply returns the existing record.
func (r *ProjectRegistry) Register(path string) (Project, error) {
	if path == "" {
		return Project{}, fmt.Errorf("project path is empty")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return Project{}, fmt.Errorf("resolve path: %w", err)
	}

	st, err := os.Stat(abs)
	if err != nil {
		return Project{}, fmt.Errorf("stat path: %w", err)
	}

	if !st.IsDir() {
		return Project{}, fmt.Errorf("path is not a directory: %s", abs)
	}

	// Check if already registered.
	for _, p := range r.projects {
		if p.Path == abs {
			fmt.Printf("project registry: path already registered: %s\n", abs)
			return p, nil
		}
	}

	name := filepath.Base(abs)
	if name == "" || name == "/" {
		name = abs
	}

	proj := Project{
		Name:         name,
		Path:         abs,
		Type:         "unknown",
		Languages:    []string{},
		RegisteredAt: time.Now().UTC().Format(time.RFC3339),
	}

	r.projects = append(r.projects, proj)

	if err := r.save(); err != nil {
		return Project{}, err
	}

	fmt.Printf("project registry: registered project %q at %s\n", proj.Name, proj.Path)
	return proj, nil
}

// List returns a copy of all registered projects.
func (r *ProjectRegistry) List() []Project {
	out := make([]Project, len(r.projects))
	copy(out, r.projects)
	return out
}

// FindByName returns the first project whose name matches (case-insensitive).
func (r *ProjectRegistry) FindByName(name string) (Project, bool) {
	if name == "" {
		return Project{}, false
	}

	lower := strings.ToLower(name)
	for _, p := range r.projects {
		if strings.ToLower(p.Name) == lower {
			return p, true
		}
	}

	return Project{}, false
}
