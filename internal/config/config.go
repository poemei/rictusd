package config

import (
  "encoding/json"
  "fmt"
  "os"
  "time"
)

type HTTPConfig struct {
  Addr           string `json:"addr"`
  ReadTimeoutMs  int    `json:"read_timeout_ms"`
  WriteTimeoutMs int    `json:"write_timeout_ms"`
}

type STNConfig struct {
  Enabled bool   `json:"enabled"`
  URL     string `json:"url"`
  Token   string `json:"token"` // deprecated when stn_ingest_secret is set
}

type DigitConfig struct {
  Enabled    bool   `json:"enabled"`
  Socket     string `json:"socket"`
  Namespace  string `json:"namespace"`
  AuthHeader string `json:"auth_header"` // reserved
  AuthValue  string `json:"auth_value"`  // reserved
}

type AutoConfig struct {
  Enabled       bool     `json:"enabled"`
  PollMs        int      `json:"poll_ms"`
  WatchConfig   bool     `json:"watch_config"`
  WatchPaths    []string `json:"watch_paths"`   // additional paths/globs to watch
  RebuildCmd    string   `json:"rebuild_cmd"`   // if non-empty, run on any watch_paths change
}

type Config struct {
  NodeID          string      `json:"node_id"`
  DataDir         string      `json:"data_dir"`
  LogLevel        string      `json:"log_level"`
  HTTP            HTTPConfig  `json:"http"`
  STN             STNConfig   `json:"stn_api"`
  Digit           DigitConfig `json:"digit"`
  STNIngestSecret string      `json:"stn_ingest_secret"`
  MaxWorkers      int         `json:"max_workers"`
  QueueCapacity   int         `json:"queue_capacity"`
  Auto            AutoConfig  `json:"auto"`
}

func (c *Config) ReadTimeout() time.Duration {
  if c.HTTP.ReadTimeoutMs <= 0 { return 10 * time.Second }
  return time.Duration(c.HTTP.ReadTimeoutMs) * time.Millisecond
}
func (c *Config) WriteTimeout() time.Duration {
  if c.HTTP.WriteTimeoutMs <= 0 { return 10 * time.Second }
  return time.Duration(c.HTTP.WriteTimeoutMs) * time.Millisecond
}

func Load(path string) (*Config, error) {
  raw, err := os.ReadFile(path)
  if err != nil { return nil, fmt.Errorf("read %s: %w", path, err) }
  var cfg Config
  if err := json.Unmarshal(raw, &cfg); err != nil { return nil, fmt.Errorf("parse %s: %w", path, err) }
  if cfg.MaxWorkers <= 0 { cfg.MaxWorkers = 2 }
  if cfg.QueueCapacity <= 0 { cfg.QueueCapacity = 128 }
  if cfg.HTTP.Addr == "" { cfg.HTTP.Addr = "127.0.0.1:7979" }
  if cfg.Auto.PollMs <= 0 { cfg.Auto.PollMs = 1500 }
  return &cfg, nil
}
