package brain

import (
	"fmt"

	"rictusd/modules/core"
)

// SuggestEngine produces non-destructive suggestions based on project maps.
// It never writes to disk or executes anything; it only thinks and speaks.
type SuggestEngine struct {
	core *core.Core
}

// NewSuggestEngine constructs a SuggestEngine bound to the daemon core.
func NewSuggestEngine(c *core.Core) *SuggestEngine {
	return &SuggestEngine{core: c}
}

// SuggestionsForProject takes a ProjectMap and returns human-readable
// suggestion strings that Mind can weave into natural language.
func (s *SuggestEngine) SuggestionsForProject(pm ProjectMap) []string {
	out := make([]string, 0)

	// If there is nothing there, there's not much to say.
	if pm.TotalFiles == 0 && pm.TotalDirs == 0 {
		out = append(out, "the project appears to be effectively empty; you may want to confirm the path or initialize a proper structure.")
		return out
	}

	// Simple structural observations.
	if pm.MaxDepth > 8 {
		out = append(out,
			fmt.Sprintf("the directory tree is quite deep (max depth %d); you may want to consider flattening or grouping modules more clearly.", pm.MaxDepth))
	}

	// High-level language hints.
	if pm.PHPFiles > 0 && pm.GoFiles == 0 && pm.JSFiles == 0 {
		out = append(out,
			"this looks primarily like a PHP project; once you are ready, I can focus future analysis on your PHP standards (strict_types, DocBlocks, PSR style).")
	}

	if pm.PHPFiles > 0 && pm.GoFiles > 0 {
		out = append(out,
			"I see both PHP and Go in this tree; if these are separate concerns, you may want to isolate them into clearer module boundaries.")
	}

	if pm.TotalFiles > 0 && pm.OtherFiles > pm.TotalFiles/2 {
		out = append(out,
			"there are quite a lot of non-PHP/Go/JS files; you may want to verify which of those are still relevant and which are legacy or stray artifacts.")
	}

	// Basic sanity hint if depth is shallow but file count is high.
	if pm.MaxDepth <= 3 && pm.TotalFiles > 200 {
		out = append(out,
			"there are many files in a relatively shallow structure; introducing a small amount of modular grouping might make navigation cleaner.")
	}

	// If nothing triggered, still say something gentle.
	if len(out) == 0 {
		out = append(out,
			"nothing alarming stands out structurally from this first pass; when you are ready, we can move into deeper, file-level analysis.")
	}

	return out
}
