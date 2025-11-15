package watch

import (
  "context"
  "crypto/sha1"
  "fmt"
  "io"
  "os"
  "path/filepath"
  "strings"
  "time"

  "rictusd/internal/config"
  "rictusd/internal/logx"
)

type Manager struct {
  cfgPath string
  cfg     *config.Config
  reload  func(string) error
  running bool
}

func NewManager(cfgPath string, cfg *config.Config, reload func(string) error) *Manager {
  return &Manager{cfgPath: cfgPath, cfg: cfg, reload: reload}
}

func (m *Manager) Start(ctx context.Context) {
  if !m.cfg.Auto.Enabled { return }
  m.running = true
  poll := time.Duration(m.cfg.Auto.PollMs) * time.Millisecond
  if poll < 300*time.Millisecond { poll = 300 * time.Millisecond }
  logx.Info("watch", "start", "poll_ms", int(poll/time.Millisecond))

  // baselines
  lastCfgSig := fileSig(m.cfgPath)
  lastPathsSig := m.pathsSig()

  ticker := time.NewTicker(poll)
  defer ticker.Stop()
  for {
    select {
    case <-ctx.Done():
      logx.Info("watch", "stop")
      return
    case <-ticker.C:
      // config changes
      if m.cfg.Auto.WatchConfig {
        cur := fileSig(m.cfgPath)
        if cur != lastCfgSig {
          logx.Info("watch", "cfg_change_detected")
          if err := m.reload(m.cfgPath); err != nil {
            logx.Error("watch", "cfg_reload_failed", "err", err)
          } else {
            logx.Info("watch", "cfg_reloaded")
            lastCfgSig = cur
          }
        }
      }
      // path changes -> rebuild
      if cmd := strings.TrimSpace(m.cfg.Auto.RebuildCmd); cmd != "" {
        cur := m.pathsSig()
        if cur != lastPathsSig {
          logx.Info("watch", "paths_changed", "paths", strings.Join(m.cfg.Auto.WatchPaths, ","))
          // best-effort shell
          if err := runShell(ctx, cmd); err != nil {
            logx.Error("watch", "rebuild_failed", "err", err)
          } else {
            logx.Info("watch", "rebuild_ok")
          }
          lastPathsSig = cur
        }
      }
    }
  }
}

func (m *Manager) pathsSig() string {
  if len(m.cfg.Auto.WatchPaths) == 0 { return "" }
  h := sha1.New()
  for _, p := range m.cfg.Auto.WatchPaths {
    matches, _ := filepath.Glob(p)
    for _, f := range matches {
      fi, err := os.Stat(f)
      if err != nil { continue }
      io.WriteString(h, f)
      io.WriteString(h, fi.ModTime().UTC().Format(time.RFC3339Nano))
      io.WriteString(h, fmt.Sprintf("%d", fi.Size()))
    }
  }
  return fmt.Sprintf("%x", h.Sum(nil))
}

func fileSig(p string) string {
  fi, err := os.Stat(p)
  if err != nil { return "" }
  return fmt.Sprintf("%s|%d|%s", p, fi.Size(), fi.ModTime().UTC().Format(time.RFC3339Nano))
}

// very small shell runner (bash -lc) so user can do: "make build && systemctl restart rictus"
func runShell(ctx context.Context, cmd string) error {
  // lazy import to avoid pulling os/exec here; define here to keep file self-contained
  return runBash(ctx, cmd)
}
