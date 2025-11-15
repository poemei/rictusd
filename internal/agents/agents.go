package agents

import (
  "context"
  "errors"
  "time"
)

type Agent interface {
  Name() string
  Ping(ctx context.Context) error
  Run(ctx context.Context, op string, payload map[string]any) (map[string]any, error)
}

type EchoAgent struct{}

func (EchoAgent) Name() string { return "echo" }
func (EchoAgent) Ping(ctx context.Context) error {
  select {
  case <-ctx.Done():
    return ctx.Err()
  case <-time.After(50 * time.Millisecond):
    return nil
  }
}
func (EchoAgent) Run(ctx context.Context, op string, payload map[string]any) (map[string]any, error) {
  if op == "" { return nil, errors.New("op required") }
  out := map[string]any{
    "op":      op,
    "payload": payload,
    "ts":      time.Now().UTC().Format(time.RFC3339Nano),
  }
  return out, nil
}
