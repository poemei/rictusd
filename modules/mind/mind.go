package mind

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"rictusd/modules/brain"
	"rictusd/modules/convo"
	"rictusd/modules/core"
	"rictusd/modules/law"
	"rictusd/modules/patch"
	"rictusd/modules/tasks"
)

// languageConfig is optional config for how RictusD addresses you.
type languageConfig struct {
	Address string `json:"Address"` // e.g., "Madam"
	Phase   int    `json:"Phase"`   // e.g., 1, 2, 3...
}

// Mind is the internal "voice" and behavior layer of RictusD.
type Mind struct {
	core     *core.Core
	brain    *brain.Brain
	convo    *convo.Store
	law      *law.Law
	projects *core.ProjectRegistry
	mapper   *brain.Mapper
	suggest  *brain.SuggestEngine
	init     *brain.Initializer
	phpScan  *brain.PHPScanner
	patchEng *patch.Engine
	tasks    *tasks.Store

	address        string
	phase          int
	lastProject    string
	lastPHPExample string

	lastPatch    map[string]string // key: projectName:relPath
	lastPatchKey string            // last key we patched

	routerCache map[string]string // projectName -> router relative path
}

// New initializes the Mind.
func New(c *core.Core) *Mind {
	m := &Mind{
		core:     c,
		brain:    brain.NewBrain(c),
		convo:    convo.NewStore(c),
		law:      law.New(c),
		projects: core.NewProjectRegistry(c.Data),
		mapper:   brain.NewMapper(c),
		suggest:  brain.NewSuggestEngine(c),
		init:     brain.NewInitializer(c),
		phpScan:  brain.NewPHPScanner(c),
		patchEng: patch.NewEngine(c),
		tasks:    tasks.NewStore(c),
	}

	m.lastPatch = make(map[string]string)
	m.routerCache = make(map[string]string)

	cfg := m.loadLanguageConfig()
	if strings.TrimSpace(cfg.Address) == "" {
		cfg.Address = "Madam"
	}
	if cfg.Phase <= 0 {
		cfg.Phase = 3
	}

	m.address = cfg.Address
	m.phase = cfg.Phase

	c.Log.Infof("Mind initialized: address=%q phase=%d law_exists=%v",
		m.address, m.phase, m.law != nil && m.law.Exists())

	return m
}

// Chat handles a single incoming message and returns the reply text.
func (m *Mind) Chat(message string) string {
	msg := strings.TrimSpace(message)

	// Record inbound message.
	m.convo.Append("user", msg)
	m.brain.Record("chat", "user", msg)
	m.core.Log.Infof("chat message: %q", msg)

	lower := strings.ToLower(msg)

	// Simple, natural-language task commands before core command parsing.
	if m.tasks != nil {
		switch {
		case strings.HasPrefix(lower, "task "):
			reply := m.handleTaskAdd(strings.TrimSpace(msg[5:]))
			m.convo.Append("daemon", reply)
			m.brain.Record("chat", "daemon", reply)
			return reply

		case strings.HasPrefix(lower, "todo "):
			reply := m.handleTaskAdd(strings.TrimSpace(msg[5:]))
			m.convo.Append("daemon", reply)
			m.brain.Record("chat", "daemon", reply)
			return reply

		case lower == "tasks" || lower == "show tasks":
			reply := m.handleTaskList()
			m.convo.Append("daemon", reply)
			m.brain.Record("chat", "daemon", reply)
			return reply

		case strings.HasPrefix(lower, "done "):
			reply := m.handleTaskDone(strings.TrimSpace(msg[5:]))
			m.convo.Append("daemon", reply)
			m.brain.Record("chat", "daemon", reply)
			return reply
		}
	}

	// Let core handle parsing; Mind just sees a Command.
	cmd := core.DispatchCommand(msg)

	var reply string

	switch cmd.Kind {
	case core.CommandFeedbackApproved:
		m.brain.Record("feedback", "note", "approved")
		reply = m.address + ", understood. I’ll treat that as approved."

	case core.CommandFeedbackRejected:
		m.brain.Record("feedback", "note", "rejected")
		reply = m.address + ", understood. I’ll treat that as rejected."

	case core.CommandStatus:
		reply = m.statusReply()

	case core.CommandLawStatus:
		reply = m.lawStatusReply()

	case core.CommandRouter:
		reply = m.handleRouter()

	case core.CommandAnalyzeRouter:
		reply = m.handleAnalyzeRouter()

	case core.CommandRegisterProject:
		if cmd.Arg == "" {
			reply = m.address + ", you asked me to register a project but didn’t give me a path."
		} else {
			fake := "register project " + cmd.Arg
			reply = m.handleRegisterProject(fake, "register project ")
		}

	case core.CommandMapProject:
		if cmd.Arg == "" {
			reply = m.address + ", you asked me to map a project but didn’t give me a name."
		} else {
			fake := "map project " + cmd.Arg
			reply = m.handleMapProject(fake, "map project ")
		}

	case core.CommandSuggestProject:
		if cmd.Arg == "" {
			reply = m.address + ", you asked for suggestions but didn’t give me a project name."
		} else {
			fake := "suggest project " + cmd.Arg
			reply = m.handleSuggestProject(fake, "suggest project ")
		}

	case core.CommandAnalyze:
		if cmd.Arg == "" {
			reply = m.handleAnalyzeDefault()
		} else {
			fake := "analyze project " + cmd.Arg
			reply = m.handleAnalyzeProject(fake, "analyze project ")
		}

	case core.CommandPatch:
		if cmd.Arg == "" {
			reply = m.handlePatchDefault()
		} else {
			fake := "patch " + cmd.Arg
			reply = m.handlePatchFile(fake, "patch ")
		}

	case core.CommandApply:
		if cmd.Arg == "" {
			reply = m.handleApplyDefault()
		} else {
			fake := "apply " + cmd.Arg
			reply = m.handleApplyFile(fake, "apply ")
		}

	default:
		reply = m.defaultReply()
	}

	// Record outbound message.
	m.convo.Append("daemon", reply)
	m.brain.Record("chat", "daemon", reply)

	return reply
}

// --- Status / Law -----------------------------------------------------------

func (m *Mind) statusReply() string {
	listen := m.core.Config.ListenAddr
	dataDir := m.core.Data

	lawState := "no lawbook detected"
	if m.law != nil && m.law.Exists() {
		lawState = "lawbook loaded and in effect"
	}

	var b strings.Builder

	b.WriteString(m.address + ", here’s where I stand right now.\n\n")
	b.WriteString("I’m listening on " + listen + " and using \"" + dataDir + "\" for my data.\n")
	b.WriteString("Law status: " + lawState + ".\n")
	b.WriteString("Operating in Phase " + strconv.Itoa(m.phase) + " – insight, analysis, patch proposals, and apply under your approval.\n")

	return b.String()
}

func (m *Mind) lawStatusReply() string {
	if m.law == nil || !m.law.Exists() {
		return m.address + ", I don’t see a lawbook yet. I expected it under conf/lawbook.md."
	}

	return m.address + ", your lawbook is present and loaded. I’m following the rules defined in conf/lawbook.md."
}

// --- Router -----------------------------------------------------------------

func (m *Mind) handleRouter() string {
	if m.lastProject == "" {
		return m.address + ", I don’t have a current project yet. For example: analyze chaos-mvc."
	}

	if m.projects == nil {
		m.projects = core.NewProjectRegistry(m.core.Data)
	}

	proj, ok := m.projects.FindByName(m.lastProject)
	if !ok {
		return m.address + ", I don’t see a registered project named \"" + m.lastProject + "\"."
	}

	// If cached, verify it still exists and return.
	if rel, ok := m.routerCache[proj.Name]; ok {
		full := filepath.Join(proj.Path, rel)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			bootstrap := routerHasBootstrap(full)

			var b strings.Builder
			b.WriteString(m.address + ", for \"" + proj.Name + "\" I’m treating \"" + rel + "\" as the router.\n")
			if bootstrap {
				b.WriteString("It references bootstrap.php, so the bootstrap pattern is in place.\n")
			} else {
				b.WriteString("I don’t clearly see a bootstrap.php reference in that file yet.\n")
			}
			b.WriteString("This path is cached; I’ll reuse it unless the file moves or disappears.")

			return b.String()
		}
		// If cached path is invalid now, drop it and re-discover.
		delete(m.routerCache, proj.Name)
	}

	// Discover a router candidate.
	candidates := []string{
		"public/index.php",
		"index.php",
		"public/router.php",
		"app/router.php",
	}

	var found string
	for _, rel := range candidates {
		full := filepath.Join(proj.Path, rel)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			found = rel
			break
		}
	}

	if found == "" {
		return m.address + ", I don’t see an obvious router file for \"" + proj.Name + "\" yet. I checked public/index.php, index.php, public/router.php, and app/router.php."
	}

	m.routerCache[proj.Name] = found
	full := filepath.Join(proj.Path, found)
	bootstrap := routerHasBootstrap(full)

	var b strings.Builder
	b.WriteString(m.address + ", for \"" + proj.Name + "\" I’ve identified \"" + found + "\" as the router.\n")
	if bootstrap {
		b.WriteString("It appears to use bootstrap.php, which matches your preferred pattern.\n")
	} else {
		b.WriteString("I don’t clearly see a bootstrap.php reference there yet.\n")
	}
	b.WriteString("I’ll cache this as the router path for this project.")

	return b.String()
}

func (m *Mind) handleAnalyzeRouter() string {
	if m.lastProject == "" {
		return m.address + ", I don’t have a current project yet. For example: analyze chaos-mvc, then analyze router."
	}

	if m.projects == nil {
		m.projects = core.NewProjectRegistry(m.core.Data)
	}

	proj, ok := m.projects.FindByName(m.lastProject)
	if !ok {
		return m.address + ", I don’t see a registered project named \"" + m.lastProject + "\"."
	}

	// Resolve router file, preferring cache.
	var rel string

	if cached, ok := m.routerCache[proj.Name]; ok {
		full := filepath.Join(proj.Path, cached)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			rel = cached
		} else {
			delete(m.routerCache, proj.Name)
		}
	}

	if rel == "" {
		candidates := []string{
			"public/index.php",
			"index.php",
			"public/router.php",
			"app/router.php",
		}
		for _, c := range candidates {
			full := filepath.Join(proj.Path, c)
			if info, err := os.Stat(full); err == nil && !info.IsDir() {
				rel = c
				break
			}
		}
	}

	if rel == "" {
		return m.address + ", I tried to analyze the router, but I couldn’t find a likely router file. I checked public/index.php, index.php, public/router.php, and app/router.php."
	}

	m.routerCache[proj.Name] = rel
	full := filepath.Join(proj.Path, rel)

	data, err := os.ReadFile(full)
	if err != nil {
		m.core.Log.Errorf("analyze router: read failed: %v", err)
		return m.address + ", I found a router candidate but couldn’t read it: " + err.Error()
	}

	content := string(data)
	bootstrap := routerHasBootstrap(full)

	hasDB := strings.Contains(content, "mysqli_") ||
		strings.Contains(content, "PDO") ||
		strings.Contains(content, "SELECT ")
	hasHTML := strings.Contains(content, "<html") ||
		strings.Contains(strings.ToLower(content), "<!doctype html") ||
		strings.Contains(content, "echo \"<") ||
		strings.Contains(content, "echo '<")
	hasSideEffects := strings.Contains(content, "file_put_contents(") ||
		strings.Contains(content, "fopen(") ||
		strings.Contains(content, "unlink(")

	var b strings.Builder

	b.WriteString(m.address + ", here’s what I see in the router for \"" + proj.Name + "\".\n\n")
	b.WriteString("I’m looking at \"" + rel + "\" as the router entry.\n")
	if bootstrap {
		b.WriteString("It references bootstrap.php, so your bootstrap pattern is in play.\n")
	} else {
		b.WriteString("I don’t clearly see a bootstrap.php reference in this file.\n")
	}

	b.WriteString("\nFrom a Router Law perspective:\n")

	if hasDB {
		b.WriteString("- I see signs of DB or query logic inside the router, which is something you normally want out of that layer.\n")
	} else {
		b.WriteString("- I don’t see obvious DB or query logic in the router.\n")
	}

	if hasHTML {
		b.WriteString("- I see signs of inline HTML or view rendering in the router.\n")
	} else {
		b.WriteString("- I don’t see obvious inline HTML or view rendering in the router.\n")
	}

	if hasSideEffects {
		b.WriteString("- I see file or side-effect style operations inside the router.\n")
	} else {
		b.WriteString("- I don’t see obvious file I/O side-effects in the router.\n")
	}

	if !hasDB && !hasHTML && !hasSideEffects {
		b.WriteString("\nOverall, this router looks clean under your Router Law from this quick scan.")
	}

	return b.String()
}

func routerHasBootstrap(fullPath string) bool {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "bootstrap.php")
}

// --- Project registration / map / suggest -----------------------------------

func (m *Mind) handleRegisterProject(msg string, prefix string) string {
	if m.projects == nil {
		m.projects = core.NewProjectRegistry(m.core.Data)
	}

	if len(msg) <= len(prefix) {
		return m.address + ", you asked me to register a project but didn’t give me a path."
	}

	rawPath := strings.TrimSpace(msg[len(prefix):])
	if rawPath == "" {
		return m.address + ", you asked me to register a project, but the path was empty."
	}

	proj, err := m.projects.Register(rawPath)
	if err != nil {
		m.core.Log.Errorf("project register failed: %v", err)
		return m.address + ", registering that project failed: " + err.Error()
	}

	m.lastProject = proj.Name

	return m.address + ", I’ve registered that as project \"" + proj.Name + "\" at \"" + proj.Path + "\"."
}

func (m *Mind) handleMapProject(msg string, prefix string) string {
	if m.projects == nil {
		m.projects = core.NewProjectRegistry(m.core.Data)
	}
	if m.mapper == nil {
		m.mapper = brain.NewMapper(m.core)
	}
	if m.suggest == nil {
		m.suggest = brain.NewSuggestEngine(m.core)
	}

	if len(msg) <= len(prefix) {
		return m.address + ", you asked me to map a project but didn’t give me a name."
	}

	raw := strings.TrimSpace(msg[len(prefix):])
	if raw == "" {
		return m.address + ", you asked me to map a project but the name was empty."
	}

	proj, ok := m.projects.FindByName(raw)
	if !ok {
		return m.address + ", I don’t see a registered project named \"" + raw + "\"."
	}

	m.lastProject = proj.Name

	pm, err := m.mapper.MapProject(proj)
	if err != nil {
		m.core.Log.Errorf("map project failed: %v", err)
		return m.address + ", mapping that project failed: " + err.Error()
	}

	suggestions := m.suggest.SuggestionsForProject(pm)

	var b strings.Builder

	b.WriteString(m.address + ", here’s the high-level map for \"" + proj.Name + "\".\n\n")
	b.WriteString("Path: " + pm.Path + ".\n")
	b.WriteString("Files: " + strconv.Itoa(pm.TotalFiles) + ", directories: " + strconv.Itoa(pm.TotalDirs) + ", max depth: " + strconv.Itoa(pm.MaxDepth) + ".\n")

	if len(pm.Languages) > 0 {
		b.WriteString("Languages I see: " + strings.Join(pm.Languages, ", ") + ".\n")
	}

	if len(suggestions) > 0 {
		b.WriteString("\nA few structural observations:\n")
		for _, s := range suggestions {
			b.WriteString("- " + s + "\n")
		}
	}

	return b.String()
}

func (m *Mind) handleSuggestProject(msg string, prefix string) string {
	if m.projects == nil {
		m.projects = core.NewProjectRegistry(m.core.Data)
	}
	if m.mapper == nil {
		m.mapper = brain.NewMapper(m.core)
	}
	if m.suggest == nil {
		m.suggest = brain.NewSuggestEngine(m.core)
	}

	if len(msg) <= len(prefix) {
		return m.address + ", you asked for suggestions but didn’t give me a project name."
	}

	raw := strings.TrimSpace(msg[len(prefix):])
	if raw == "" {
		return m.address + ", you asked for suggestions but the name was empty."
	}

	proj, ok := m.projects.FindByName(raw)
	if !ok {
		return m.address + ", I don’t see a registered project named \"" + raw + "\"."
	}

	m.lastProject = proj.Name

	pm, err := m.mapper.MapProject(proj)
	if err != nil {
		m.core.Log.Errorf("map for suggestions failed: %v", err)
		return m.address + ", analyzing that project failed: " + err.Error()
	}

	suggestions := m.suggest.SuggestionsForProject(pm)
	if len(suggestions) == 0 {
		return m.address + ", nothing urgent jumps out structurally yet. It looks calm from this distance."
	}

	var b strings.Builder
	b.WriteString(m.address + ", here are some structural suggestions for \"" + proj.Name + "\":\n")
	for _, s := range suggestions {
		b.WriteString("- " + s + "\n")
	}

	return b.String()
}

// --- Analyze ---------------------------------------------------------------

func (m *Mind) handleAnalyzeDefault() string {
	if m.lastProject == "" {
		return m.address + ", you said \"analyze\" but didn’t tell me which project. For example: analyze chaos-mvc."
	}

	fake := "analyze project " + m.lastProject
	return m.handleAnalyzeProject(fake, "analyze project ")
}

func (m *Mind) handleAnalyzeProject(msg string, prefix string) string {
	if m.projects == nil {
		m.projects = core.NewProjectRegistry(m.core.Data)
	}
	if m.mapper == nil {
		m.mapper = brain.NewMapper(m.core)
	}
	if m.init == nil {
		m.init = brain.NewInitializer(m.core)
	}
	if m.phpScan == nil {
		m.phpScan = brain.NewPHPScanner(m.core)
	}

	if len(msg) <= len(prefix) {
		return m.address + ", you asked me to analyze a project but didn’t give me a name."
	}

	raw := strings.TrimSpace(msg[len(prefix):])
	if raw == "" {
		return m.address + ", you asked me to analyze a project but the name was empty."
	}

	proj, ok := m.projects.FindByName(raw)
	if !ok {
		return m.address + ", I don’t see a registered project named \"" + raw + "\"."
	}

	m.lastProject = proj.Name

	created, readmePath, err := m.init.EnsureReadme(proj)
	if err != nil {
		m.core.Log.Errorf("analyze: ensure README failed: %v", err)
		return m.address + ", I tried to ensure a README exists, but that failed: " + err.Error()
	}

	pm, err := m.mapper.MapProject(proj)
	if err != nil {
		m.core.Log.Errorf("analyze: map failed: %v", err)
		return m.address + ", mapping that project failed: " + err.Error()
	}

	phpReport, err := m.phpScan.AnalyzeProject(proj)
	if err != nil {
		m.core.Log.Errorf("analyze: PHP scan failed: %v", err)
		return m.address + ", the PHP scan failed: " + err.Error()
	}

	if len(phpReport.SampleNoDoc) > 0 {
		m.lastPHPExample = phpReport.SampleNoDoc[0]
	} else if phpReport.TotalFiles > 0 && m.lastPHPExample == "" {
		m.lastPHPExample = "index.php"
	}

	var b strings.Builder

	b.WriteString(m.address + ", here’s what I see in \"" + proj.Name + "\".\n\n")

	// README
	if created {
		b.WriteString("I created a minimal README for you at " + readmePath + " so the project has a starting point.\n")
	} else {
		b.WriteString("There’s already a README in place (for example at " + readmePath + ").\n")
	}

	// Structure
	b.WriteString("Structurally, I see " + strconv.Itoa(pm.TotalFiles) + " files across " +
		strconv.Itoa(pm.TotalDirs) + " directories, with a maximum depth of " + strconv.Itoa(pm.MaxDepth) + ".\n")
	if len(pm.Languages) > 0 {
		b.WriteString("Language-wise I’m seeing: " + strings.Join(pm.Languages, ", ") + ".\n")
	}

	// PHP
	if phpReport.TotalFiles == 0 {
		b.WriteString("\nI don’t see any PHP files yet, so there’s nothing PHP-specific to critique.")
		return b.String()
	}

	b.WriteString("\nOn the PHP side, I see " + strconv.Itoa(phpReport.TotalFiles) + " file")
	if phpReport.TotalFiles != 1 {
		b.WriteString("s")
	}
	b.WriteString(".\n")

	if phpReport.MissingStrict > 0 {
		b.WriteString(strconv.Itoa(phpReport.MissingStrict) + " file")
		if phpReport.MissingStrict != 1 {
			b.WriteString("s")
		}
		b.WriteString(" appear to be missing declare(strict_types=1).\n")
	} else {
		b.WriteString("All inspected PHP files appear to be using declare(strict_types=1).\n")
	}

	if phpReport.MissingDocHint > 0 {
		b.WriteString(strconv.Itoa(phpReport.MissingDocHint) + " file")
		if phpReport.MissingDocHint != 1 {
			b.WriteString("s")
		}
		b.WriteString(" don’t seem to have a top-level docblock.\n")
		if len(phpReport.SampleNoDoc) > 0 {
			b.WriteString("Examples include: " + strings.Join(phpReport.SampleNoDoc, ", ") + ".\n")
		}
	} else {
		b.WriteString("From the sample I saw, top-level docblocks look present.\n")
	}

	if phpReport.MissingRequireCount > 0 {
		b.WriteString(strconv.Itoa(phpReport.MissingRequireCount) + " require/include target")
		if phpReport.MissingRequireCount != 1 {
			b.WriteString("s")
		}
		b.WriteString(" look unresolved.\n")

		limit := len(phpReport.MissingRequires)
		if limit > 5 {
			limit = 5
		}
		if limit > 0 {
			b.WriteString("Examples:\n")
			for i := 0; i < limit; i++ {
				mr := phpReport.MissingRequires[i]
				b.WriteString("- " + mr.File + " → " + mr.Target + "\n")
			}
		}
	} else {
		b.WriteString("I don’t see unresolved require/include targets from this scan.\n")
	}

	return b.String()
}

// --- Patch ------------------------------------------------------------------

func (m *Mind) handlePatchDefault() string {
	if m.lastProject == "" {
		return m.address + ", you said \"patch\" but I don’t know which project you want. For example: analyze chaos-mvc first."
	}
	if m.lastPHPExample == "" {
		return m.address + ", I don’t have a specific PHP file in mind to patch yet. For example: patch index.php."
	}

	fake := "patch " + m.lastPHPExample
	return m.handlePatchFile(fake, "patch ")
}

func (m *Mind) handlePatchFile(msg string, prefix string) string {
	if m.projects == nil {
		m.projects = core.NewProjectRegistry(m.core.Data)
	}
	if m.patchEng == nil {
		m.patchEng = patch.NewEngine(m.core)
	}
	if m.phpScan == nil {
		m.phpScan = brain.NewPHPScanner(m.core)
	}

	if m.lastProject == "" {
		return m.address + ", you asked me to patch a file, but I don’t know which project yet. For example: analyze chaos-mvc first."
	}

	if len(msg) <= len(prefix) {
		return m.handlePatchDefault()
	}

	file := strings.TrimSpace(msg[len(prefix):])
	if file == "" {
		return m.handlePatchDefault()
	}

	proj, ok := m.projects.FindByName(m.lastProject)
	if !ok {
		return m.address + ", I’ve lost track of the last project. Tell me which one to use again."
	}

	rel := strings.TrimPrefix(file, "./")
	rel = strings.TrimPrefix(rel, "/")

	phpReport, err := m.phpScan.AnalyzeProject(proj)
	if err != nil {
		m.core.Log.Errorf("patch: PHP scan failed: %v", err)
		return m.address + ", I tried to scan the PHP for that project, but the scan failed: " + err.Error()
	}

	var unresolved []brain.MissingRequire
	if phpReport.MissingRequireCount > 0 {
		for _, mr := range phpReport.MissingRequires {
			if mr.File == rel {
				unresolved = append(unresolved, mr)
			}
		}
	}

	patched, err := m.patchEng.PatchPHPFile(proj, rel)
	if err != nil {
		m.core.Log.Errorf("patch: PatchPHPFile failed: %v", err)
		return m.address + ", I couldn’t prepare a patch for " + rel + ": " + err.Error()
	}

	// Remember last patch.
	key := proj.Name + ":" + rel
	m.lastPatch[key] = patched
	m.lastPatchKey = key
	m.lastPHPExample = rel

	var b strings.Builder

	b.WriteString(m.address + ", here’s the cleaned version of \"" + rel + "\" for \"" + proj.Name + "\".\n\n")
	b.WriteString("I’ve prepared this as a proposal only; I have not written it to disk. You’re still in control.\n\n")

	if len(unresolved) > 0 {
		b.WriteString("I also noticed unresolved require/include targets in this file:\n")
		limit := len(unresolved)
		if limit > 3 {
			limit = 3
		}
		for i := 0; i < limit; i++ {
			mr := unresolved[i]
			b.WriteString("- " + mr.File + " → " + mr.Target + "\n")
		}
		b.WriteString("You can tell me whether to fix those paths, generate stubs, or leave them alone.\n\n")
	}

	b.WriteString("Proposed file (preview only):\n\n```php\n")
	b.WriteString(patched)
	b.WriteString("\n```\\n")

	return b.String()
}

// --- Apply ------------------------------------------------------------------

func (m *Mind) handleApplyDefault() string {
	if m.lastProject == "" {
		return m.address + ", you said \"apply\" but I don’t have a project context yet. For example: analyze chaos-mvc."
	}
	if m.lastPatchKey == "" {
		return m.address + ", I don’t have a prepared patch queued to apply. Patch a file first, then I can apply it."
	}

	parts := strings.SplitN(m.lastPatchKey, ":", 2)
	if len(parts) != 2 {
		return m.address + ", my last patch reference looks wrong internally. Ask me to patch the file again and I’ll refresh it."
	}

	file := parts[1]
	return m.applyPatchForFile(file)
}

func (m *Mind) handleApplyFile(msg string, prefix string) string {
	if len(msg) <= len(prefix) {
		return m.handleApplyDefault()
	}

	file := strings.TrimSpace(msg[len(prefix):])
	if file == "" {
		return m.handleApplyDefault()
	}

	return m.applyPatchForFile(file)
}

func (m *Mind) applyPatchForFile(file string) string {
	if m.projects == nil {
		m.projects = core.NewProjectRegistry(m.core.Data)
	}
	if m.patchEng == nil {
		m.patchEng = patch.NewEngine(m.core)
	}

	if m.lastProject == "" {
		return m.address + ", I don’t know which project you want this patch applied to. For example: analyze chaos-mvc."
	}

	proj, ok := m.projects.FindByName(m.lastProject)
	if !ok {
		return m.address + ", I’ve lost track of the last project. Tell me which one to use again."
	}

	rel := strings.TrimPrefix(file, "./")
	rel = strings.TrimPrefix(rel, "/")

	key := proj.Name + ":" + rel
	content, ok := m.lastPatch[key]
	if !ok {
		return m.address + ", I don’t have a prepared patch stored for \"" + rel + "\" yet. Ask me to patch that file first."
	}

	if err := m.patchEng.ApplyFile(proj, rel, content); err != nil {
		m.core.Log.Errorf("apply: ApplyFile failed: %v", err)
		return m.address + ", applying that patch failed: " + err.Error()
	}

	return m.address + ", done. I’ve applied the patch to \"" + rel + "\" in \"" + proj.Name + "\" and saved a .bak backup of the previous version."
}

// --- Tasks ------------------------------------------------------------------

func (m *Mind) handleTaskAdd(text string) string {
	if text == "" {
		return m.address + ", you asked me to create a task but didn’t say what it is."
	}
	if m.tasks == nil {
		return m.address + ", my task store isn’t available."
	}

	t, err := m.tasks.Add(text)
	if err != nil {
		m.core.Log.Errorf("tasks: add failed: %v", err)
		return m.address + ", I tried to add that task but something went wrong."
	}

	return m.address + ", I’ve added that as task #" + strconv.Itoa(t.ID) + "."
}

func (m *Mind) handleTaskList() string {
	if m.tasks == nil {
		return m.address + ", my task store isn’t available."
	}

	items := m.tasks.List()
	if len(items) == 0 {
		return m.address + ", you don’t have any tasks stored with me yet."
	}

	var b strings.Builder
	b.WriteString(m.address + ", here are the tasks I’m tracking:\n")
	for _, t := range items {
		status := "open"
		if t.Done {
			status = "done"
		}
		b.WriteString("- #" + strconv.Itoa(t.ID) + " [" + status + "] " + t.Text + "\n")
	}

	return b.String()
}

func (m *Mind) handleTaskDone(idText string) string {
	if m.tasks == nil {
		return m.address + ", my task store isn’t available."
	}

	id, err := strconv.Atoi(idText)
	if err != nil {
		return m.address + ", I need a numeric task id after \"done\". For example: done 1."
	}

	err = m.tasks.Complete(id)
	if err != nil {
		if err == tasks.ErrNotFound {
			return m.address + ", I don’t see a task with id #" + strconv.Itoa(id) + "."
		}
		m.core.Log.Errorf("tasks: complete failed: %v", err)
		return m.address + ", I tried to mark that task done but something went wrong."
	}

	return m.address + ", task #" + strconv.Itoa(id) + " is now marked as done."
}

// --- Default / config ------------------------------------------------------

func (m *Mind) defaultReply() string {
	return m.address + ", I’ve heard you and recorded the message.\nI can map, analyze, patch, and apply within the current project when you ask, and I won’t write any changes unless you explicitly tell me to apply them."
}

func (m *Mind) loadLanguageConfig() languageConfig {
	confPath := filepath.Join(m.core.Conf, "language.json")
	var cfg languageConfig

	f, err := os.Open(confPath)
	if err != nil {
		return cfg
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		m.core.Log.Errorf("mind: decode language.json: %v", err)
		return languageConfig{}
	}

	return cfg
}
