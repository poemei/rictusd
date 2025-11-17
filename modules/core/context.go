package core

import (
	"log"
	"os"
	"path/filepath"
)

// Logger is a minimal logging interface used by RictusD internals.
// It supports both plain and formatted messages so existing code can call
// Info(...) or Infof(...), etc.
type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)

	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// stdLogger is a simple logger that writes to the standard library log package.
type stdLogger struct{}

// Info logs an informational message.
func (l *stdLogger) Info(msg string) {
	log.Printf("[INFO] %s", msg)
}

// Warn logs a warning message.
func (l *stdLogger) Warn(msg string) {
	log.Printf("[WARN] %s", msg)
}

// Error logs an error message.
func (l *stdLogger) Error(msg string) {
	log.Printf("[ERROR] %s", msg)
}

// Infof logs a formatted informational message.
func (l *stdLogger) Infof(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

// Warnf logs a formatted warning message.
func (l *stdLogger) Warnf(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}

// Errorf logs a formatted error message.
func (l *stdLogger) Errorf(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

// Config holds daemon configuration that other modules care about.
type Config struct {
	ListenAddr string `json:"listen_addr"`
}

// Core is the central context passed around to subsystems.
// Keep it lean: paths, config, and a logger.
type Core struct {
	Root   string // root directory of the daemon
	Data   string // data directory path
	Conf   string // conf directory path
	Config Config // runtime config
	Log    Logger // logger
}

// New constructs a Core context using the current working directory as the root.
// Data and Conf are derived as ./data and ./conf by default. ListenAddr defaults
// to :8080 unless overridden by higher-level configuration.
func New() (*Core, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	root := cwd
	dataDir := filepath.Join(root, "data")
	confDir := filepath.Join(root, "conf")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		return nil, err
	}

	c := &Core{
		Root: root,
		Data: dataDir,
		Conf: confDir,
		Config: Config{
			ListenAddr: ":8080",
		},
		Log: &stdLogger{},
	}

	return c, nil
}
