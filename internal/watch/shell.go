package watch

import (
  "context"
  "errors"
  "os/exec"
  "strings"
  "time"
)

func runBash(ctx context.Context, cmd string) error {
  if strings.TrimSpace(cmd) == "" { return errors.New("empty cmd") }
  c := exec.CommandContext(ctx, "bash", "-lc", cmd)
  c.Env = append(c.Env, "RICTUS_AUTORELOAD=1")
  c.Stdout = nil
  c.Stderr = nil
  // give generous but finite time
  if _, ok := ctx.Deadline(); !ok {
    var cancel context.CancelFunc
    ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
  }
  return c.Run()
}
