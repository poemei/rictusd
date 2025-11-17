package convo

import (
	"fmt"
	"strings"

	"rictusd/modules/brain"
)

// Insight generates natural-language summaries from structured data.
type Insight struct{}

// NewInsight constructs a new Insight helper.
func NewInsight() *Insight {
	return &Insight{}
}

// ProjectSummary returns a short natural-language description of a project map.
func (i *Insight) ProjectSummary(pm brain.ProjectMap, address string) string {
	if pm.TotalFiles == 0 && pm.TotalDirs == 0 {
		return address + ", that project appears to be empty from my vantage point. " +
			"Once there is real structure there, I can give you a meaningful overview."
	}

	var langs string
	if len(pm.Languages) == 0 {
		langs = "no clear language signatures yet"
	} else {
		langs = strings.Join(pm.Languages, ", ")
	}

	return fmt.Sprintf(
		"%s, here is what I see in project %q at %s. "+
			"It contains %d files across %d directories, with a maximum depth of %d. "+
			"I detected: %d PHP files, %d Go files, %d JavaScript files, and %d other files. "+
			"Language hints: %s. "+
			"This is still read-only analysis; I have not touched or changed anything.",
		address,
		pm.Name,
		pm.Path,
		pm.TotalFiles,
		pm.TotalDirs,
		pm.MaxDepth,
		pm.PHPFiles,
		pm.GoFiles,
		pm.JSFiles,
		pm.OtherFiles,
		langs,
	)
}
