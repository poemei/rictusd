package patch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rictusd/modules/core"
)

// Engine performs in-memory patching of source files.
// It never writes to disk; callers decide what to do with the result.
type Engine struct {
	core *core.Core
}

// NewEngine constructs a new Engine bound to the daemon core.
func NewEngine(c *core.Core) *Engine {
	return &Engine{core: c}
}

// PatchPHPFile reads a PHP file under the given project and returns a patched
// version with a normalized PSR-ish header: opening tag, strict_types,
// and a docblock, then the rest of the file.
func (e *Engine) PatchPHPFile(p core.Project, relPath string) (string, error) {
	full := filepath.Join(p.Path, relPath)

	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("read PHP file: %w", err)
	}

	original := string(data)
	patched := patchPHPContent(original, relPath, p.Name)

	return patched, nil
}

// patchPHPContent rewrites the top of the file to:
//
// <?php
//
// declare(strict_types=1);
//
// /**
//  * Entry point for ProjectName.
//  *
//  * @package ProjectName
//  */
//
// ...rest of original file (with any old header stripped)...
func patchPHPContent(src, fileName, projectName string) string {
	// Normalize newlines
	src = strings.ReplaceAll(src, "\r\n", "\n")

	if !strings.Contains(src, "<?php") {
		// Not a PHP file we want to touch.
		return src
	}

	lines := strings.Split(src, "\n")

	// Walk from the top and strip:
	// - leading blank lines
	// - any "<?php" lines
	// - any blank lines after that
	i := 0
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			i++
			continue
		}
		if strings.HasPrefix(t, "<?php") {
			i++
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}
			continue
		}
		break
	}

	// Now strip any leading declare() and top-level docblock.
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			i++
			continue
		}

		// Strip declare(...)
		if strings.HasPrefix(t, "declare(") {
			i++
			continue
		}

		// Strip leading docblock (/** ... */)
		if strings.HasPrefix(t, "/**") {
			i++
			for i < len(lines) {
				if strings.Contains(lines[i], "*/") {
					i++
					break
				}
				i++
			}
			// Skip any blank lines after the docblock
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}
			continue
		}

		// Anything else we keep as body.
		break
	}

	body := strings.Join(lines[i:], "\n")
	body = strings.TrimLeft(body, "\n")

	headerLines := []string{
		"<?php",
		"",
		"declare(strict_types=1);",
		"",
		"/**",
		" * Entry point for " + projectName + ".",
		" *",
		" * @package " + projectName,
		" */",
		"",
	}
	header := strings.Join(headerLines, "\n")

	if strings.TrimSpace(body) == "" {
		return header + "\n"
	}

	return header + "\n" + body
}

// ApplyFile writes the given content to the target file under the project.
// It creates a simple .bak backup of the previous file content if it exists.
func (e *Engine) ApplyFile(p core.Project, relPath, content string) error {
	full := filepath.Join(p.Path, relPath)

	// Best-effort backup
	if old, err := os.ReadFile(full); err == nil {
		_ = os.WriteFile(full+".bak", old, 0644)
	}

	return os.WriteFile(full, []byte(content), 0644)
}
