package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	cmd := os.Args[1]

	var (
		bin = flagString(cmd, "bin", "./bin/rictusd", "path to rictusd binary")
		cfg = flagString(cmd, "config", "data/rictusd.json", "path to rictusd config")
		pid = flagString(cmd, "pid", "data/rictusd.pid", "path to pid file")
		out = flagString(cmd, "out", "data/logs/rictusd.out", "stdout/stderr log file")
		n   = flagInt(cmd, "n", 200, "lines for tail (tail subcommand)")
	)
	parseFlags(cmd)

	switch cmd {
	case "start":
		dieIf(start(*bin, *cfg, *pid, *out))
	case "stop":
		dieIf(stop(*pid))
	case "restart":
		_ = stop(*pid)
		dieIf(start(*bin, *cfg, *pid, *out))
	case "status":
		msg, _ := status(*pid)
		fmt.Println(msg)
	case "tail", "logs":
		dieIf(tail(*out, *n))
	default:
		usage()
	}
}

func usage() {
	fmt.Println(`usage: rictusctl <start|stop|restart|status|tail> [flags]

Common flags:
  -bin     path to rictusd binary (default ./bin/rictusd)
  -config  path to rictus config  (default data/rictusd.json)
  -pid     path to pid file       (default data/rictusd.pid)
  -out     stdout/stderr log      (default data/logs/rictusd.out)

Tail flags:
  -n       lines for tail (default 200)
`)
}

func flagString(cmd, name, value, usage string) *string {
	fs := ensureFS(cmd)
	if fs.Lookup(name) == nil {
		fs.String(name, value, usage)
	}
	return strPtr(name, fs)
}

func flagInt(cmd, name string, value int, usage string) *int {
	fs := ensureFS(cmd)
	if fs.Lookup(name) == nil {
		fs.Int(name, value, usage)
	}
	return intPtr(name, fs)
}

var flagSets = map[string]*flag.FlagSet{}

func ensureFS(cmd string) *flag.FlagSet {
	if fs, ok := flagSets[cmd]; ok {
		return fs
	}
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	flagSets[cmd] = fs
	return fs
}

func parseFlags(cmd string) {
	fs := ensureFS(cmd)
	_ = fs.Parse(os.Args[2:])
}

func strPtr(name string, fs *flag.FlagSet) *string {
	var v string
	fs.VisitAll(func(f *flag.Flag) {
		if f.Name == name {
			// no-op: value filled by Parse
		}
	})
	p := fs.Lookup(name)
	if p != nil {
		v = p.Value.String()
	} else {
		v = ""
	}
	return &v
}

func intPtr(name string, fs *flag.FlagSet) *int {
	p := fs.Lookup(name)
	if p == nil {
		z := 0
		return &z
	}
	i, _ := strconv.Atoi(p.Value.String())
	return &i
}

func start(bin, cfg, pidPath, outPath string) error {
	// Already running?
	if alive, _ := isRunning(pidPath); alive {
		msg, _ := status(pidPath)
		fmt.Println(msg)
		return nil
	}

	// Validate inputs
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("binary not found: %s", bin)
	}
	if _, err := os.Stat(cfg); err != nil {
		return fmt.Errorf("config not found: %s", cfg)
	}

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	// Open/append log file
	logFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	// We won't close logFile here; the child process inherits the fd.

	// Prepare command: detach session, no stdin, stdout/stderr to file
	cmd := exec.Command(bin, "-config", cfg)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}

	// Write PID
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("write pid: %w", err)
	}

	// Give it a moment, confirm it's alive
	time.Sleep(600 * time.Millisecond)
	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		_ = os.Remove(pidPath)
		return fmt.Errorf("process not alive after start: %v", err)
	}

	fmt.Printf("rictus started (PID %d)\n", cmd.Process.Pid)
	return nil
}

func stop(pidPath string) error {
	pid, err := readPID(pidPath)
	if err != nil {
		return fmt.Errorf("read pid: %w", err)
	}
	// Hard kill (per your requirement)
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("kill -9 %d: %w", pid, err)
	}
	_ = os.Remove(pidPath)
	fmt.Println("rictus stopped")
	return nil
}

func status(pidPath string) (string, error) {
	pid, err := readPID(pidPath)
	if err != nil {
		return "rictus not running", nil
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return fmt.Sprintf("rictus not running (stale PID %d)", pid), nil
	}
	return fmt.Sprintf("rictus running (PID %d)", pid), nil
}

func isRunning(pidPath string) (bool, int) {
	pid, err := readPID(pidPath)
	if err != nil {
		return false, 0
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return false, pid
	}
	return true, pid
}

func readPID(pidPath string) (int, error) {
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return 0, fmt.Errorf("empty pid file")
	}
	return strconv.Atoi(s)
}

func tail(path string, n int) error {
	// Try to exec system tail for simplicity and proper follow
	tcmd := exec.Command("sh", "-c", fmt.Sprintf("tail -n %d -f %s", n, shellQuote(path)))
	tcmd.Stdout = os.Stdout
	tcmd.Stderr = os.Stderr
	tcmd.Stdin = os.Stdin
	return tcmd.Run()
}

// --- small helpers ---

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.ContainsAny(s, " \t\"'`$\\") {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}
	return s
}

func dieIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// (optional) copyFile is unused but handy if you later want to rotate logs here.
func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil { return err }
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil { return err }
	return out.Sync()
}

