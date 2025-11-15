package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"rictusd/internal/version"
)

type cliConfig struct {
	DataDir string `json:"data_dir"`
	Paths struct {
		QueueDir   string `json:"queue_dir"`
		ReportsDir string `json:"reports_dir"`
		LogsDir    string `json:"logs_dir"`
	} `json:"paths"`
	Queue struct {
		Dir string `json:"dir"`
	} `json:"queue"`
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func repoRoot() string {
	exe, _ := os.Executable()
	return filepath.Clean(filepath.Join(filepath.Dir(exe), ".."))
}

func rootJoin(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(repoRoot(), p)
}

func loadCfg(path string) cliConfig {
	if path == "" {
		path = "data/rictusd.json"
	}
	b, err := os.ReadFile(path)
	must(err)
	var c cliConfig
	_ = json.Unmarshal(b, &c)

	// resolve effective queue
	q := c.Paths.QueueDir
	if q == "" {
		q = c.Queue.Dir
	}
	if q == "" {
		q = "./data/queue"
	}
	c.Paths.QueueDir = rootJoin(q)

	// resolve reports/logs
	r := c.Paths.ReportsDir
	if r == "" {
		r = "./data/reports"
	}
	c.Paths.ReportsDir = rootJoin(r)

	l := c.Paths.LogsDir
	if l == "" {
		l = "./data/log/tasks"
	}
	c.Paths.LogsDir = rootJoin(l)

	return c
}

func ensureDirWritable(dir string) error {
	// create if missing (parents too)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s failed: %w", dir, err)
	}
	// probe writability by creating/removing a temp file
	probe := filepath.Join(dir, ".rictuscli.probe")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		// try to enrich EPERM/EACCES with stat details
		st, stErr := os.Stat(dir)
		if stErr == nil {
			if s, ok := st.Sys().(*syscall.Stat_t); ok {
				return fmt.Errorf("queue not writable: %s (uid=%d gid=%d mode=%o): %v",
					dir, s.Uid, s.Gid, st.Mode().Perm(), err)
			}
		}
		return fmt.Errorf("queue not writable: %s: %v", dir, err)
	}
	f.Close()
	_ = os.Remove(probe)
	return nil
}

func readAllStdin() ([]byte, error) {
	info, _ := os.Stdin.Stat()
	if (info.Mode() & os.ModeCharDevice) != 0 {
		return nil, fmt.Errorf("stdin is empty (pipe JSON or use --file)")
	}
	return bufio.NewReader(os.Stdin).ReadBytes(0)
}

func cmdVersion() {
	fmt.Println("rictuscli", version.String())
}

func cmdSubmit(cfgPath, jsonFile string) {
	cfg := loadCfg(cfgPath)
	if err := ensureDirWritable(cfg.Paths.QueueDir); err != nil {
		must(err)
	}

	var payload []byte
	var err error
	if jsonFile != "" {
		payload, err = os.ReadFile(jsonFile)
		must(err)
	} else {
		payload, err = readAllStdin()
		if payload == nil || len(payload) == 0 {
			sc := bufio.NewScanner(os.Stdin)
			var b strings.Builder
			for sc.Scan() {
				b.WriteString(sc.Text())
			}
			payload = []byte(b.String())
		}
	}

	var tmp map[string]any
	_ = json.Unmarshal(payload, &tmp)
	if _, ok := tmp["id"]; !ok {
		tmp["id"] = fmt.Sprintf("task-%d", time.Now().UnixNano())
		b, _ := json.Marshal(tmp)
		payload = b
	}
	id, _ := tmp["id"].(string)
	if id == "" {
		must(fmt.Errorf("task id resolution failed"))
	}

	out := filepath.Join(cfg.Paths.QueueDir, id+".json")
	if err := os.WriteFile(out, payload, 0o644); err != nil {
		// If write still fails, report precise cause (not a fake ENOENT)
		st, stErr := os.Stat(cfg.Paths.QueueDir)
		if stErr == nil {
			if s, ok := st.Sys().(*syscall.Stat_t); ok {
				must(fmt.Errorf("write failed: %s (uid=%d gid=%d mode=%o): %v",
					cfg.Paths.QueueDir, s.Uid, s.Gid, st.Mode().Perm(), err))
			}
		}
		must(fmt.Errorf("write failed: %s: %v", cfg.Paths.QueueDir, err))
	}

	fmt.Println(id)
	fmt.Fprintf(os.Stderr, "[queue] %s\n", cfg.Paths.QueueDir)
}

func cmdStatus(cfgPath, id string) {
	if id == "" {
		fmt.Fprintln(os.Stderr, "missing --id")
		os.Exit(2)
	}
	cfg := loadCfg(cfgPath)
	p := filepath.Join(cfg.Paths.ReportsDir, id+".json")
	b, err := os.ReadFile(p)
	if err != nil {
		fmt.Println("UNKNOWN")
		return
	}
	var rep map[string]any
	_ = json.Unmarshal(b, &rep)
	st, _ := rep["status"].(string)
	if st == "" {
		st = "UNKNOWN"
	}
	fmt.Println(st)
}

func cmdReport(cfgPath, id string) {
	if id == "" {
		fmt.Fprintln(os.Stderr, "missing --id")
		os.Exit(2)
	}
	cfg := loadCfg(cfgPath)
	p := filepath.Join(cfg.Paths.ReportsDir, id+".json")
	b, err := os.ReadFile(p)
	must(err)
	os.Stdout.Write(b)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <submit|status|report|version> [flags]\n", os.Args[0])
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "-v", "--version":
		cmdVersion()
	case "submit":
		fs := flag.NewFlagSet("submit", flag.ExitOnError)
		cfg := fs.String("config", "data/rictusd.json", "path to rictusd config")
		file := fs.String("file", "", "task JSON file (or pipe JSON via stdin)")
		_ = fs.Parse(os.Args[2:])
		cmdSubmit(*cfg, *file)
	case "status":
		fs := flag.NewFlagSet("status", flag.ExitOnError)
		cfg := fs.String("config", "data/rictusd.json", "path to rictusd config")
		id := fs.String("id", "", "task id")
		_ = fs.Parse(os.Args[2:])
		cmdStatus(*cfg, *id)
	case "report":
		fs := flag.NewFlagSet("report", flag.ExitOnError)
		cfg := fs.String("config", "data/rictusd.json", "path to rictusd config")
		id := fs.String("id", "", "task id")
		_ = fs.Parse(os.Args[2:])
		cmdReport(*cfg, *id)
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", os.Args[1])
		os.Exit(2)
	}
}
