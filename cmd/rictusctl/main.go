package main

import (
	"fmt"
	"os"
	"os/exec"
	//"path/filepath"
	"syscall"
)

const (
	pidFile  = "/opt/rictusd/data/rictusd.pid"
	daemon   = "/opt/rictusd/bin/rictusd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: rictusctl <start|stop|restart|status>")
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "start":
		startDaemon()
	case "stop":
		stopDaemon()
	case "restart":
		restartDaemon()
	case "status":
		status()
	default:
		fmt.Println("Unknown command:", cmd)
		os.Exit(1)
	}
}

func startDaemon() {
	// Check if already running
	if pid, running := readPID(); running {
		fmt.Printf("RictusD is already running (PID %d)\n", pid)
		return
	}

	fmt.Println("Starting RictusD…")

	// Ensure the binary exists
	if _, err := os.Stat(daemon); err != nil {
		fmt.Printf("Daemon binary not found at %s\n", daemon)
		os.Exit(1)
	}

	// Launch the daemon
	cmd := exec.Command(daemon)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	// Write PID file
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		fmt.Printf("Failed to write PID file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("RictusD started (PID %d)\n", cmd.Process.Pid)
}

func stopDaemon() {
	pid, running := readPID()
	if !running {
		fmt.Println("RictusD is not running.")
		return
	}

	fmt.Printf("Stopping RictusD (PID %d)…\n", pid)

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		fmt.Printf("Failed to stop daemon: %v\n", err)
		os.Exit(1)
	}

	// Remove pidfile
	_ = os.Remove(pidFile)

	fmt.Println("RictusD stopped.")
}

func restartDaemon() {
	stopDaemon()
	startDaemon()
}

func status() {
	pid, running := readPID()
	if running {
		fmt.Printf("RictusD is running (PID %d)\n", pid)
	} else {
		fmt.Println("RictusD is not running.")
	}
}

func readPID() (int, bool) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}

	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil {
		return 0, false
	}

	// Check if PID is actually alive
	if err := syscall.Kill(pid, 0); err != nil {
		// Not running — remove stale PID file
		_ = os.Remove(pidFile)
		return 0, false
	}

	return pid, true
}
