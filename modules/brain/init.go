package brain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rictusd/modules/core"
)

// Initializer performs small, explicit, allowed project initializations
// such as creating a README when one does not exist.
type Initializer struct {
	core *core.Core
}

// NewInitializer constructs an Initializer bound to the daemon core.
func NewInitializer(c *core.Core) *Initializer {
	return &Initializer{core: c}
}

// EnsureReadme checks for a README in the project root and, if none is found,
// creates a simple README.md. It returns whether a README was created, the
// path to the README, and any error encountered.
func (i *Initializer) EnsureReadme(p core.Project) (bool, string, error) {
	entries, err := os.ReadDir(p.Path)
	if err != nil {
		return false, "", fmt.Errorf("read project root: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		nameUpper := strings.ToUpper(entry.Name())
		if strings.HasPrefix(nameUpper, "README") {
			// Already has a README-like file.
			return false, filepath.Join(p.Path, entry.Name()), nil
		}
	}

	// No README found; create a simple README.md
	readmePath := filepath.Join(p.Path, "README.md")
	content := "# " + p.Name + "\n\n" +
		"This README was initialized automatically by RictusD at your request. " +
		"You may expand or replace it as you see fit.\n"

	if err := os.WriteFile(readmePath, []byte(content), 0o644); err != nil {
		return false, "", fmt.Errorf("write README.md: %w", err)
	}

	i.core.Log.Infof("initializer: created README.md for project %q at %s", p.Name, readmePath)
	return true, readmePath, nil
}
