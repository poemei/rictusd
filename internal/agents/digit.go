package agents

import (
  "bytes"
  "context"
  "encoding/json"
  "errors"
  "fmt"
  "io"
  "net"
  "net/http"
  "net/url"
  "strings"
  "sync"
  "time"
)

type DigitAgent struct {
  mu        sync.RWMutex
  baseURL   string
  namespace string
  httpc     *http.Client
  enabled   bool

  user          string
  pass          string
  session       string
  sessionHeader string
  extraHeaders  map[string]string
}

func NewDigitAgent(socket, namespace string, enabled bool) *DigitAgent {
  base, qs := normalizeBase(socket)
  d := &DigitAgent{
    baseURL:   strings.TrimRight(base, "/"),
    namespace: namespace,
    enabled:   enabled,
    httpc: &http.Client{ Timeout: 8 * time.Second, Transport: &http.Transport{ DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext } },
    sessionHeader: "X-Session",
    extraHeaders:  map[string]string{},
  }
  if v := strings.TrimSpace(qs.Get("user")); v != "" { d.user = v }
  if v := qs.Get("pass"); v != "" { d.pass = v }
  if v := qs.Get("session"); v != "" { d.session = v }
  if v := strings.TrimSpace(qs.Get("session_header")); v != "" { d.sessionHeader = v }
  for key, vals := range qs {
    if !strings.HasPrefix(key, "h_") { continue }
    hname := strings.ReplaceAll(strings.TrimPrefix(key, "h_"), "_", "-")
    if len(vals) > 0 && strings.TrimSpace(vals[0]) != "" { d.extraHeaders[hname] = vals[0] }
  }
  if v := strings.TrimSpace(qs.Get("timeout_ms")); v != "" {
    if ms, err := parseInt(v); err == nil && ms > 0 { d.httpc.Timeout = time.Duration(ms) * time.Millisecond }
  }
  return d
}

func (d *DigitAgent) Name() string { return "digit" }

func (d *DigitAgent) Ping(ctx context.Context) error {
  if !d.enabled { return errors.New("digit disabled") }
  req, _ := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+"/healthz", nil)
  d.applyHeaders(req, false)
  resp, err := d.httpc.Do(req); if err != nil { return err }
  defer resp.Body.Close()
  if resp.StatusCode >= 300 { return fmt.Errorf("digit ping: %s", resp.Status) }
  return nil
}

func (d *DigitAgent) Run(ctx context.Context, op string, payload map[string]any) (map[string]any, error) {
  if !d.enabled { return nil, errors.New("digit disabled") }
  if op == "" { return nil, errors.New("op required") }
  switch op {
  case "ping":
    if err := d.Ping(ctx); err != nil { return nil, err }
    return map[string]any{"ok": true}, nil
  case "chat":
    input, _ := str(payload["input"]); if input == "" { return nil, errors.New(`payload.input (string) required for "chat"`) }
    return d.doJSON(ctx, http.MethodPost, "/chat", map[string]any{"input": input})
  case "commands":
    return d.doJSON(ctx, http.MethodGet, "/commands", nil)
  case "exec":
    name, _ := str(payload["name"]); if name == "" { return nil, errors.New(`payload.name (string) required for "exec"`) }
    args, _ := asMap(payload["args"])
    return d.doJSON(ctx, http.MethodPost, "/exec", map[string]any{"name": name, "args": args})
  case "alerts":
    n := 20; if v, ok := asInt(payload["n"]); ok && v > 0 { n = v }
    return d.doJSON(ctx, http.MethodGet, fmt.Sprintf("/alerts?n=%d", n), nil)
  default:
    return nil, fmt.Errorf("digit: unknown op %q", op)
  }
}

func (d *DigitAgent) doJSON(ctx context.Context, method, path string, body map[string]any) (map[string]any, error) {
  u := d.baseURL + path
  var rdr *bytes.Reader
  if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
    raw, _ := json.Marshal(body); rdr = bytes.NewReader(raw)
  } else { rdr = bytes.NewReader(nil) }
  if d.session == "" && d.user != "" && d.pass != "" {
    if err := d.login(ctx); err != nil { return nil, fmt.Errorf("digit login failed: %w", err) }
  }
  req, _ := http.NewRequestWithContext(ctx, method, u, rdr)
  if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch { req.Header.Set("Content-Type", "application/json") }
  d.applyHeaders(req, true)
  resp, err := d.httpc.Do(req); if err != nil { return nil, err }
  defer resp.Body.Close()
  if resp.StatusCode == http.StatusUnauthorized && d.user != "" && d.pass != "" {
    _ = resp.Body.Close()
    if err := d.login(ctx); err != nil { return nil, fmt.Errorf("digit re-login failed: %w", err) }
    req2, _ := http.NewRequestWithContext(ctx, method, u, rdr)
    if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch { req2.Header.Set("Content-Type", "application/json") }
    d.applyHeaders(req2, true)
    resp, err = d.httpc.Do(req2); if err != nil { return nil, err }
    defer resp.Body.Close()
  }
  if resp.StatusCode >= 300 { return nil, fmt.Errorf("digit %s %s: %s", method, path, resp.Status) }
  if resp.StatusCode == http.StatusNoContent { return map[string]any{"ok": true, "status": resp.StatusCode}, nil }
  b, _ := io.ReadAll(resp.Body)
  if len(b) == 0 { return map[string]any{"ok": true, "status": resp.StatusCode}, nil }
  var anyOut any
  if err := json.Unmarshal(b, &anyOut); err != nil { return map[string]any{"ok": true, "status": resp.StatusCode, "text": string(b)}, nil }
  if m, ok := anyOut.(map[string]any); ok { return m, nil }
  return map[string]any{"ok": true, "status": resp.StatusCode, "result": anyOut}, nil
}

func (d *DigitAgent) login(ctx context.Context) error {
  u := d.baseURL + "/login"
  raw, _ := json.Marshal(map[string]string{"username": d.user, "password": d.pass})
  req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
  req.Header.Set("Content-Type", "application/json")
  d.applyHeaders(req, false)
  resp, err := d.httpc.Do(req); if err != nil { return err }
  defer resp.Body.Close()
  if resp.StatusCode >= 300 { return fmt.Errorf("login: %s", resp.Status) }
  var out struct{ OK bool `json:"ok"`; Token string `json:"token"` }
  if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return err }
  if !out.OK || strings.TrimSpace(out.Token) == "" { return errors.New("login: missing token") }
  d.mu.Lock(); d.session = out.Token; d.mu.Unlock()
  return nil
}

func (d *DigitAgent) applyHeaders(req *http.Request, withSession bool) {
  req.Header.Set("Accept", "application/json")
  req.Header.Set("User-Agent", "rictus-digit/0.3.0")
  for k, v := range d.extraHeaders { req.Header.Set(k, v) }
  if withSession {
    d.mu.RLock(); s := d.session; d.mu.RUnlock()
    if strings.TrimSpace(s) != "" && strings.TrimSpace(d.sessionHeader) != "" {
      req.Header.Set(d.sessionHeader, s)
    } else if strings.TrimSpace(s) != "" {
      req.Header.Set("X-Session", s)
    }
  }
}

func normalizeBase(socket string) (string, url.Values) {
  base := socket
  if strings.HasPrefix(base, "tcp://") { base = "http://" + strings.TrimPrefix(base, "tcp://") }
  if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") { base = "http://" + base }
  u, err := url.Parse(base); if err != nil || u == nil { return base, url.Values{} }
  q := u.Query(); u.RawQuery = ""; u.Fragment = ""
  return u.String(), q
}
func parseInt(s string) (int, error) { var n int; _, err := fmt.Sscanf(s, "%d", &n); return n, err }
func str(v any) (string, bool) { if s, ok := v.(string); ok { return s, true }; return "", false }
func asMap(v any) (map[string]any, bool) { if v == nil { return map[string]any{}, true }; m, ok := v.(map[string]any); return m, ok }
func asInt(v any) (int, bool) { switch t := v.(type) { case float64: return int(t), true; case int: return t, true; default: return 0, false } }
