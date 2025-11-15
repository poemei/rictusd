// cmd/rictusd/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"rictusd/internal/api"
	"rictusd/internal/config"
	"rictusd/internal/core"
	"rictusd/internal/logx"
	"rictusd/internal/version"
	"rictusd/internal/watch"
)

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "data/rictusd.json", "path to rictusd config json")
	flag.Parse()

	// Load config first so we know data_dir
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Println("config load failed:", err)
		os.Exit(1)
	}

	// Init logs: <data_dir>/logs/rictus.log
	logDir := filepath.Join(cfg.DataDir, "logs")
	logx.InitWithDir(logDir)
	logx.Info("rictus", "starting", "version", version.String(), "log_dir", logDir)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	ctrl, err := core.NewController(cfg)
	if err != nil {
		logx.Error("core", "init_failed", "err", err)
		os.Exit(1)
	}

	srv := api.NewServer(cfg, ctrl, cfgPath)
	go func() {
		if err := srv.Start(); err != nil {
			logx.Error("api", "start_failed", "err", err)
			cancel()
		}
	}()

	go ctrl.Run(ctx)
	go watch.NewManager(cfgPath, cfg, ctrl.ReloadFromFile).Start(ctx)

	<-ctx.Done()
	logx.Info("rictus", "shutdown_begin")
	_ = srv.Stop(context.Background())
	_ = ctrl.Stop()
	logx.Info("rictus", "shutdown_complete")
	fmt.Println("bye.")
}

