package law

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"rictusd/modules/core"
)

// Law provides access to the daemon's lawbook (conf/lawbook.md).
type Law struct {
	core      *core.Core
	lawbook   string
	hasLaw    bool
}

// New builds a Law helper bound to the daemon's conf directory.
func New(c *core.Core) *Law {
	path := filepath.Join(c.Conf, "lawbook.md")

	l := &Law{
		core:    c,
		lawbook: path,
		hasLaw:  false,
	}

	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		l.hasLaw = true
	} else {
		c.Log.Warnf("Lawbook not found at %s", path)
	}

	return l
}

// Exists reports whether a lawbook file was detected.
func (l *Law) Exists() bool {
	return l != nil && l.hasLaw
}

// ReadAll returns the full contents of lawbook.md as a string.
func (l *Law) ReadAll() (string, error) {
	if l == nil {
		return "", os.ErrNotExist
	}
	data, err := os.ReadFile(l.lawbook)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Headings returns a slice of all top-level markdown headings
// (lines beginning with '#' after optional leading whitespace).
func (l *Law) Headings() ([]string, error) {
	if l == nil {
		return nil, os.ErrNotExist
	}

	f, err := os.Open(l.lawbook)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var headings []string
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			headings = append(headings, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return headings, err
	}

	return headings, nil
}

