package brain

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"rictusd/modules/core"
)

// ProjectMap is a high-level structural summary of a project.
type ProjectMap struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	TotalFiles  int      `json:"total_files"`
	TotalDirs   int      `json:"total_dirs"`
	Languages   []string `json:"languages"`
	PHPFiles    int      `json:"php_files"`
	GoFiles     int      `json:"go_files"`
	JSFiles     int      `json:"js_files"`
	OtherFiles  int      `json:"other_files"`
	MaxDepth    int      `json:"max_depth"`
	RootEntries int      `json:"root_entries"`
	HasReadme   bool     `json:"has_readme"`
}

// Mapper performs read-only mapping of project directory structures.
type Mapper struct {
	core *core.Core
}

// NewMapper creates a new Mapper bound to the daemon core.
func NewMapper(c *core.Core) *Mapper {
	return &Mapper{core: c}
}

// MapProject walks the project's directory tree, builds a ProjectMap, and
// writes it to data/maps/<project-name>.json.
func (m *Mapper) MapProject(p core.Project) (ProjectMap, error) {
	pm := ProjectMap{
		Name: p.Name,
		Path: p.Path,
	}

	info, err := os.ReadDir(p.Path)
	if err != nil {
		return pm, fmt.Errorf("read root dir: %w", err)
	}
	pm.RootEntries = len(info)

	hasReadme := false
	for _, entry := range info {
		if entry.IsDir() {
			continue
		}
		nameUpper := strings.ToUpper(entry.Name())
		if strings.HasPrefix(nameUpper, "README") {
			hasReadme = true
			break
		}
	}
	pm.HasReadme = hasReadme

	langs := make(map[string]struct{})

	maxDepth := 0
	err = filepath.WalkDir(p.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log and continue rather than aborting the entire walk.
			m.core.Log.Warnf("mapper: walk error on %s: %v", path, err)
			return nil
		}

		// Skip hidden .git/.svn/etc if desired; for now, skip .git.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, relErr := filepath.Rel(p.Path, path)
		if relErr == nil {
			depth := depthOf(rel)
			if depth > maxDepth {
				maxDepth = depth
			}
		}

		if d.IsDir() {
			pm.TotalDirs++
			return nil
		}

		pm.TotalFiles++

		ext := filepath.Ext(d.Name())
		switch ext {
		case ".php":
			pm.PHPFiles++
			langs["php"] = struct{}{}
		case ".go":
			pm.GoFiles++
			langs["go"] = struct{}{}
		case ".js":
			pm.JSFiles++
			langs["javascript"] = struct{}{}
		default:
			pm.OtherFiles++
			if ext != "" {
				langs[ext] = struct{}{}
			}
		}

		return nil
	})

	if err != nil {
		return pm, fmt.Errorf("walk project: %w", err)
	}

	pm.MaxDepth = maxDepth

	// Flatten language set.
	for k := range langs {
		pm.Languages = append(pm.Languages, k)
	}

	if err := m.writeMap(pm); err != nil {
		return pm, err
	}

	m.core.Log.Infof("mapper: mapped project %q at %s (files=%d, dirs=%d, has_readme=%v)",
		pm.Name, pm.Path, pm.TotalFiles, pm.TotalDirs, pm.HasReadme)

	return pm, nil
}

// writeMap persists the ProjectMap to the data/maps directory as JSON.
func (m *Mapper) writeMap(pm ProjectMap) error {
	mapsDir := filepath.Join(m.core.Data, "maps")
	if err := os.MkdirAll(mapsDir, 0o755); err != nil {
		return fmt.Errorf("create maps dir: %w", err)
	}

	tmpPath := filepath.Join(mapsDir, pm.Name+".json.tmp")
	finalPath := filepath.Join(mapsDir, pm.Name+".json")

	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project map: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp project map: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename project map: %w", err)
	}

	return nil
}

// depthOf returns the directory depth of a relative path like "a/b/c".
func depthOf(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}

	depth := 0
	for _, ch := range rel {
		if ch == filepath.Separator {
			depth++
		}
	}
	// If there's at least one segment, depth is separators+1.
	return depth + 1
}
