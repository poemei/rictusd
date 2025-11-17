package brain

import "strings"

// Role describes a rough guess at what a file does in a typical MVC-ish project.
type Role string

const (
	RoleUnknown    Role = "unknown"
	RoleRouter     Role = "router"
	RoleController Role = "controller"
	RoleView       Role = "view"
	RoleConfig     Role = "config"
	RoleModel      Role = "model"
)

// ClassifyPath makes a simple guess based on path/name hints.
func ClassifyPath(relPath string) Role {
	l := strings.ToLower(relPath)

	switch {
	case strings.Contains(l, "router") || strings.Contains(l, "route"):
		return RoleRouter
	case strings.Contains(l, "controller"):
		return RoleController
	case strings.Contains(l, "view") || strings.Contains(l, "template"):
		return RoleView
	case strings.Contains(l, "config") || strings.Contains(l, "conf"):
		return RoleConfig
	case strings.Contains(l, "model"):
		return RoleModel
	default:
		return RoleUnknown
	}
}
