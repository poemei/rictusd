package core

import "strings"

// CommandKind classifies what the user is asking RictusD to do.
type CommandKind int

const (
	CommandUnknown CommandKind = iota
	CommandStatus
	CommandLawStatus
	CommandRouter
	CommandAnalyzeRouter
	CommandRegisterProject
	CommandMapProject
	CommandSuggestProject
	CommandAnalyze // Arg: project name or empty for default
	CommandPatch   // Arg: file name or empty for default
	CommandApply   // Arg: file name or empty for default
	CommandFeedbackApproved
	CommandFeedbackRejected
)

// Command is the parsed representation of a user message.
type Command struct {
	Kind CommandKind
	Arg  string
}

// DispatchCommand parses the raw message into a Command.
// All the "analyze X", "map project Y", "patch file.php" style
// logic lives here so the Mind can stay simpler.
func DispatchCommand(message string) Command {
	raw := strings.TrimSpace(message)
	lower := strings.ToLower(raw)

	if lower == "" {
		return Command{Kind: CommandUnknown}
	}

	// Simple exact matches first.
	switch lower {
	case "status":
		return Command{Kind: CommandStatus}
	case "router":
		return Command{Kind: CommandRouter}
	case "analyze router":
		return Command{Kind: CommandAnalyzeRouter}
	}

	// Law status variants.
	if lower == "law status" || strings.Contains(lower, "lawbook status") {
		return Command{Kind: CommandLawStatus}
	}

	// Feedback hooks.
	switch lower {
	case "approved", "good", "looks good":
		return Command{Kind: CommandFeedbackApproved}
	case "rejected", "nope", "bad":
		return Command{Kind: CommandFeedbackRejected}
	}

	// Register project.
	if strings.HasPrefix(lower, "register project ") {
		arg := strings.TrimSpace(raw[len("register project "):])
		return Command{Kind: CommandRegisterProject, Arg: arg}
	}
	if strings.HasPrefix(lower, "add project ") {
		arg := strings.TrimSpace(raw[len("add project "):])
		return Command{Kind: CommandRegisterProject, Arg: arg}
	}

	// Map project.
	if strings.HasPrefix(lower, "map project ") {
		arg := strings.TrimSpace(raw[len("map project "):])
		return Command{Kind: CommandMapProject, Arg: arg}
	}
	if strings.HasPrefix(lower, "map ") {
		arg := strings.TrimSpace(raw[len("map "):])
		return Command{Kind: CommandMapProject, Arg: arg}
	}

	// Suggest project.
	if strings.HasPrefix(lower, "suggest project ") {
		arg := strings.TrimSpace(raw[len("suggest project "):])
		return Command{Kind: CommandSuggestProject, Arg: arg}
	}
	if strings.HasPrefix(lower, "suggest ") {
		arg := strings.TrimSpace(raw[len("suggest "):])
		return Command{Kind: CommandSuggestProject, Arg: arg}
	}

	// Analyze project.
	if strings.HasPrefix(lower, "analyze project ") {
		arg := strings.TrimSpace(raw[len("analyze project "):])
		return Command{Kind: CommandAnalyze, Arg: arg}
	}
	if strings.HasPrefix(lower, "analyze ") {
		arg := strings.TrimSpace(raw[len("analyze "):])
		return Command{Kind: CommandAnalyze, Arg: arg}
	}
	if lower == "analyze" {
		return Command{Kind: CommandAnalyze, Arg: ""}
	}

	// Patch file.
	if strings.HasPrefix(lower, "patch ") {
		arg := strings.TrimSpace(raw[len("patch "):])
		return Command{Kind: CommandPatch, Arg: arg}
	}
	if lower == "patch" {
		return Command{Kind: CommandPatch, Arg: ""}
	}

	// Apply patch.
	if strings.HasPrefix(lower, "apply ") {
		arg := strings.TrimSpace(raw[len("apply "):])
		return Command{Kind: CommandApply, Arg: arg}
	}
	if lower == "apply" {
		return Command{Kind: CommandApply, Arg: ""}
	}

	return Command{Kind: CommandUnknown}
}
