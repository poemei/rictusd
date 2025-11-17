package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rictusd/modules/core"
	"rictusd/modules/mind"
)

type Server struct {
	core   *core.Core
	server *http.Server
	mind   *mind.Mind
}

type chatRequest struct {
	Message string `json:"message"`
}

type chatResponse struct {
	Reply string `json:"reply"`
}

// New constructs the HTTP server but does not start listening yet.
func New(c *core.Core) (*Server, error) {
	mux := http.NewServeMux()

	s := &Server{
		core: c,
		mind: mind.New(c),
		server: &http.Server{
			Addr:              c.Config.ListenAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}

	// Web UI at /
	mux.HandleFunc("/", s.handleRoot)

	// Health + chat API
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/chat", s.handleChat)

	return s, nil
}

// Start begins listening and serving HTTP requests.
func (s *Server) Start() error {
	s.core.Log.Infof("HTTP server starting on %s", s.core.Config.ListenAddr)
	return s.server.ListenAndServe()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleRoot serves web/index.html if path is "/".
// For anything under /assets, we serve static files from web/.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Static assets: /assets/* -> web/assets/*
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		root := filepath.Join(s.core.Root, "web")
		fs := http.FileServer(http.Dir(root))
		http.StripPrefix("/", fs).ServeHTTP(w, r)
		return
	}

	// Only "/" gets index.html; anything else falls through to 404.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	indexPath := filepath.Join(s.core.Root, "web", "index.html")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		s.core.Log.Errorf("serve index.html: %v", err)
		http.Error(w, "web UI unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.core.Log.Errorf("chat decode error: %v", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	reply := s.mind.Chat(msg)

	resp := chatResponse{
		Reply: reply,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		s.core.Log.Errorf("chat encode error: %v", err)
	}
}
