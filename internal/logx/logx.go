// internal/logx/logx.go
package logx

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var level = "info"

func InitWithDir(dir string) {
	lv := strings.ToLower(strings.TrimSpace(os.Getenv("RICTUS_LOG_LEVEL")))
	if lv == "" { lv = "info" }
	level = lv

	_ = os.MkdirAll(dir, 0o755)
	fp := filepath.Join(dir, "rictus.log")
	fh, err := os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		// if file open fails, fall back to stdout
		log.SetFlags(0)
		return
	}
	log.SetOutput(fh)
	log.SetFlags(0)
}

func ts() string { return time.Now().Format(time.RFC3339) }

func logKV(sev, comp, msg string, kv ...any) {
	parts := []string{ts(), "lvl=" + sev, "comp=" + comp, "msg=" + msg}
	for i := 0; i < len(kv)-1; i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", kv[i], kv[i+1]))
	}
	log.Println(strings.Join(parts, " "))
}

func allowed(sev string) bool {
	order := map[string]int{"debug": 10, "info": 20, "warn": 30, "error": 40}
	return order[sev] >= order[level]
}

func Debug(comp, msg string, kv ...any) { if allowed("debug") { logKV("debug", comp, msg, kv...) } }
func Info(comp, msg string, kv ...any)  { if allowed("info")  { logKV("info", comp, msg, kv...) } }
func Warn(comp, msg string, kv ...any)  { if allowed("warn")  { logKV("warn", comp, msg, kv...) } }
func Error(comp, msg string, kv ...any) { if allowed("error") { logKV("error", comp, msg, kv...) } }

