package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lnoxsian/gophrdrv/internal/config"
	"github.com/lnoxsian/gophrdrv/internal/handlers"
)

type Server struct {
	cfg *config.Config
}

func NewServer(cfg *config.Config) *Server {
	return &Server{cfg: cfg}
}

// Start starts the HTTP server and handles graceful shutdown
func (s *Server) Start() error {
	ctx := handlers.NewHandlerContext(s.cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/", ctx.BrowseHandler)
	mux.HandleFunc("/browse", ctx.BrowseHandler)
	mux.HandleFunc("/download", ctx.DownloadHandler)
	mux.HandleFunc("/download-zip", ctx.DownloadZipHandler)
	mux.HandleFunc("/view", ctx.ViewHandler)
	mux.HandleFunc("/edit", ctx.EditHandler)
	mux.HandleFunc("/save", ctx.SaveHandler)
	mux.HandleFunc("/upload", ctx.UploadHandler)
	mux.HandleFunc("/mkdir", ctx.MkdirHandler)
	mux.HandleFunc("/rename", ctx.RenameHandler)
	mux.HandleFunc("/delete", ctx.DeleteHandler)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
	}

	// Channel to capture errors during startup
	errChan := make(chan error, 1)

	// Start server in background goroutine
	go func() {
		ctx.LogInfo("starting file server on http://%s serving directory %s", addr, s.cfg.Root)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Channel to listen for OS signals (SIGINT, SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal is received or startup error occurs
	select {
	case err := <-errChan:
		return fmt.Errorf("server startup failed: %w", err)
	case sig := <-sigChan:
		ctx.LogInfo("received signal %v, shutting down server...", sig)

		// Create a shutdown context with 10-second timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		ctx.LogInfo("server stopped gracefully")
	}

	return nil
}
