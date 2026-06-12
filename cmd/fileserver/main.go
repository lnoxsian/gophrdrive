package main

import (
	"fmt"
	"os"

	"github.com/lnoxsian/gophrdrv/internal/config"
	"github.com/lnoxsian/gophrdrv/internal/server"
)

func main() {
	cfg, err := config.ParseConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	srv := server.NewServer(cfg)
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
