package brain

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"rictusd/modules/core"
)

// MissingRequire represents a require/include that points to a file
// RictusD could not resolve relative to the project.
type MissingRequire struct {
	File   string `json:"file"`   // e.g., "index.php"
	Target string `json:"target"` // e.g., "/app/bootstrap"
}

// PHPReport summarizes basic PHP-level observations for a project.
type PHPReport struct {
	TotalFiles         int              `json:"total_files"`
	MissingStrict      int              `json:"missing_strict"`
	MissingDocHint     int              `json:"missing_doc_hint"`
	SampleNoDoc        []string         `json:"sample_no_doc"` // some example paths
	MissingRequireCount int             `json:"missing_require_count"`
	MissingRequires    []MissingRequire `json:"missing_requires"`
}

// PHPScanner performs read-only PHP file analysis.
type PHPScanner struct {
	core *core.Core
}

// NewPHPScanner constructs a PHPScanner bound to the daemon core.
func NewPHPScanner(c *core.Core) *PHPScanner {
	return &PHPScanner{core: c}
}

// AnalyzeProject walks the project directory and inspects PHP files for
// simple signals: presence of strict_types, docblock hints, and unresolved
// require/include paths.
func (s *PHPScanner) AnalyzeProject(p core.Project) (PHPReport, error) {
	report := PHPReport{
		SampleNoDoc:     make([]string, 0),
		MissingRequires: make([]MissingRequire, 0),
	}

	root := p.Path

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			s.core.Log.Warnf("phpscan: walk error on %s: %v", path, err)
			return nil
		}

		if d.IsDir() {
			// Skip vendor-style dirs in the future if needed.
			return nil
		}

		if filepath.Ext(d.Name()) != ".php" {
			return nil
		}

		report.TotalFiles++

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}

		hasStrict, hasDoc, missingReqs, err := s.inspectPHPFile(root, rel, path)
		if err != nil {
			s.core.Log.Warnf("phpscan: inspect error on %s: %v", path, err)
			return nil
		}

		if !hasStrict {
			report.MissingStrict++
		}
		if !hasDoc {
			report.MissingDocHint++
			if len(report.SampleNoDoc) < 5 {
				report.SampleNoDoc = append(report.SampleNoDoc, rel)
			}
		}

		if len(missingReqs) > 0 {
			report.MissingRequireCount += len(missingReqs)
			report.MissingRequires = append(report.MissingRequires, missingReqs...)
		}

		return nil
	})

	if err != nil {
		return report, fmt.Errorf("phpscan: walk project: %w", err)
	}

	return report, nil
}

// inspectPHPFile performs a very lightweight scan of the top of a PHP file
// to check for strict_types, docblock-like content, and unresolved requires.
func (s *PHPScanner) inspectPHPFile(root, relPath, fullPath string) (hasStrict bool, hasDoc bool, missing []MissingRequire, err error) {
	missing = make([]MissingRequire, 0)

	f, err := os.Open(fullPath)
	if err != nil {
		return false, false, missing, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		trim := strings.TrimSpace(line)
		if strings.Contains(trim, "declare(strict_types=1") {
			hasStrict = true
		}
		if strings.HasPrefix(trim, "/**") {
			hasDoc = true
		}

		// Very simple require/include detection.
		if strings.HasPrefix(trim, "require ") ||
			strings.HasPrefix(trim, "require(") ||
			strings.HasPrefix(trim, "require_once ") ||
			strings.HasPrefix(trim, "require_once(") ||
			strings.HasPrefix(trim, "include ") ||
			strings.HasPrefix(trim, "include(") ||
			strings.HasPrefix(trim, "include_once ") ||
			strings.HasPrefix(trim, "include_once(") {

			target := extractRequireTarget(trim)
			if target != "" && !s.resolveRequire(root, fullPath, target) {
				missing = append(missing, MissingRequire{
					File:   relPath,
					Target: target,
				})
			}
		}

		// We only need a quick impression from the top of the file.
		if lineCount > 200 {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return hasStrict, hasDoc, missing, err
	}

	return hasStrict, hasDoc, missing, nil
}

// extractRequireTarget pulls out a simple string literal from a require/include line.
func extractRequireTarget(line string) string {
	// Look for first quote after the keyword.
	idx := strings.IndexAny(line, "\"'")
	if idx == -1 {
		return ""
	}
	quote := line[idx]
	rest := line[idx+1:]
	end := strings.IndexRune(rest, rune(quote))
	if end == -1 {
		return ""
	}
	return rest[:end]
}

// resolveRequire tries to determine whether a require/include target resolves
// to an existing file relative to the project root. It returns true if it can
// find a plausible file, false otherwise.
func (s *PHPScanner) resolveRequire(root, fullPath, target string) bool {
	// Treat leading "/" as project-root-relative (web-style), not filesystem root.
	var candidateRel string
	if strings.HasPrefix(target, "/") {
		candidateRel = target[1:]
	} else {
		// Relative to the directory of the current file.
		dir := filepath.Dir(fullPath)
		relFromRoot, err := filepath.Rel(root, dir)
		if err != nil {
			relFromRoot = ""
		}
		if relFromRoot == "." || relFromRoot == "" {
			candidateRel = target
		} else {
			candidateRel = filepath.Join(relFromRoot, target)
		}
	}

	candidates := []string{
		filepath.Join(root, candidateRel),
		filepath.Join(root, candidateRel+".php"),
	}

	for _, cpath := range candidates {
		if st, err := os.Stat(cpath); err == nil && !st.IsDir() {
			return true
		}
	}

	return false
}
