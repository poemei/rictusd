package api

import (
  "context"
  "encoding/json"
  "fmt"
  "net/http"
  "time"

  "rictusd/internal/config"
  "rictusd/internal/core"
  "rictusd/internal/tasks"
  "rictusd/internal/version"
)

type Server struct {
  cfg     *config.Config
  core    *core.Controller
  cfgPath string
  srv     *http.Server
}

func NewServer(cfg *config.Config, ctrl *core.Controller, cfgPath string) *Server {
  mux := http.NewServeMux()
  s := &Server{
    cfg:     cfg,
    core:    ctrl,
    cfgPath: cfgPath,
    srv: &http.Server{
      Addr:         cfg.HTTP.Addr,
      Handler:      withCommon(mux),
      ReadTimeout:  cfg.ReadTimeout(),
      WriteTimeout: cfg.WriteTimeout(),
    },
  }
  mux.HandleFunc("/healthz", s.handleHealth)
  mux.HandleFunc("/version", s.handleVersion)
  mux.HandleFunc("/agents", s.handleAgents)
  mux.HandleFunc("/tasks", s.handleEnqueue)
  mux.HandleFunc("/tasks/", s.handleResult) // /tasks/{id}
  mux.HandleFunc("/reload", s.handleReload) // POST
  return s
}

func (s *Server) Start() error { return s.srv.ListenAndServe() }
func (s *Server) Stop(ctx context.Context) error { return s.srv.Shutdown(ctx) }

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
  writeJSON(w, http.StatusOK, map[string]any{
    "ok": true,
    "time": time.Now().UTC().Format(time.RFC3339Nano),
  })
}
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
  writeJSON(w, http.StatusOK, map[string]any{"name": "rictusd", "version": version.String()})
}
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodGet { writeErr(w, http.StatusMethodNotAllowed, "method not allowed"); return }
  writeJSON(w, http.StatusOK, map[string]any{"agents": s.core.AgentNames()})
}
func (s *Server) handleEnqueue(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodPost { writeErr(w, http.StatusMethodNotAllowed, "method not allowed"); return }
  var req struct {
    ID string `json:"id"`
    Agent string `json:"agent"`
    Op string `json:"op"`
    Payload map[string]any `json:"payload"`
  }
  if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeErr(w, http.StatusBadRequest, "invalid json"); return }
  if req.ID == "" || req.Agent == "" || req.Op == "" { writeErr(w, http.StatusBadRequest, "id, agent, and op required"); return }
  t := tasks.Task{ ID: req.ID, Agent: req.Agent, Op: req.Op, Payload: req.Payload, CreatedAt: time.Now() }
  if err := s.core.Submit(t); err != nil { writeErr(w, http.StatusServiceUnavailable, err.Error()); return }
  writeJSON(w, http.StatusAccepted, map[string]any{"queued": true, "id": req.ID})
}
func (s *Server) handleResult(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodGet { writeErr(w, http.StatusMethodNotAllowed, "method not allowed"); return }
  id := r.URL.Path[len("/tasks/"):]
  if id == "" { writeErr(w, http.StatusBadRequest, "missing task id"); return }
  res, ok := s.core.Result(id)
  if !ok { writeErr(w, http.StatusNotFound, fmt.Sprintf("no result for %q yet", id)); return }
  writeJSON(w, http.StatusOK, res)
}
func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodPost { writeErr(w, http.StatusMethodNotAllowed, "method not allowed"); return }
  if err := s.core.ReloadFromFile(s.cfgPath); err != nil { writeErr(w, http.StatusBadRequest, err.Error()); return }
  writeJSON(w, http.StatusOK, map[string]any{"reloaded": true})
}

func withCommon(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    next.ServeHTTP(w, r)
  })
}
func writeJSON(w http.ResponseWriter, code int, v any) { w.WriteHeader(code); _ = json.NewEncoder(w).Encode(v) }
func writeErr(w http.ResponseWriter, code int, msg string) { writeJSON(w, code, map[string]any{"error": msg}) }
