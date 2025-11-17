package main

import (
	"fmt"
	"os"

	"rictusd/modules/core"
	"rictusd/modules/server"
)

func main() {
	// 1. Load core system (config, logger, dirs)
	c, err := core.New()
	if err != nil {
		fmt.Printf("Failed to initialize core: %v\n", err)
		os.Exit(1)
	}

	c.Log.Info("RictusD starting up…")

	// 2. Initialize HTTP server
	srv, err := server.New(c)
	if err != nil {
		c.Log.Errorf("Failed to initialize server: %v", err)
		os.Exit(1)
	}

	// 3. Start listening
	c.Log.Infof("RictusD listening on %s", c.Config.ListenAddr)

	if err := srv.Start(); err != nil {
		c.Log.Errorf("Server error: %v", err)
		os.Exit(1)
	}
}
