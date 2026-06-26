package server

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lnoxsian/gophrdrv/internal/config"
	"github.com/lnoxsian/gophrdrv/internal/handlers"
	"github.com/lnoxsian/gophrdrv/internal/templates"
	"rsc.io/qr"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip compression for binary streaming and file transfer endpoints
		path := r.URL.Path
		if path == "/download" || path == "/download-zip" || path == "/upload" {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Initialize gzip writer at BestSpeed level to optimize transfer speed with minimal CPU overhead
		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")

		grw := gzipResponseWriter{
			ResponseWriter: w,
			Writer:         gz,
		}

		next.ServeHTTP(grw, r)
	})
}

func AuthMiddleware(ctx *handlers.HandlerContext, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ctx.Cfg.Private {
			next.ServeHTTP(w, r)
			return
		}

		// Allow access to /login endpoint
		if r.URL.Path == "/login" {
			next.ServeHTTP(w, r)
			return
		}

		// Check authentication
		cookie, err := r.Cookie("gophrdrv_session")
		if err == nil && cookie.Value == ctx.SessionToken {
			next.ServeHTTP(w, r)
			return
		}

		// If not authenticated, determine action
		// For page loads (GET / or /browse, /view, /edit), show the lock screen
		if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/browse" || r.URL.Path == "/view" || r.URL.Path == "/edit") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			data := map[string]interface{}{
				"Redirect": r.URL.RequestURI(),
				"Error":    "",
			}
			if err := templates.ExecuteTemplate(w, "lock.html", data); err != nil {
				ctx.LogError("Failed to render lock screen: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		// For other requests (POSTs, downloads, APIs), return 401 Unauthorized
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

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
	mux.HandleFunc("/login", ctx.LoginHandler)
	mux.HandleFunc("/logout", ctx.LogoutHandler)
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
		Handler:      GzipMiddleware(AuthMiddleware(ctx, mux)),
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
	}

	// Channel to capture errors during startup
	errChan := make(chan error, 1)

	// Start server in background goroutine
	go func() {
		displayHost := s.cfg.Host
		if displayHost == "" || displayHost == "0.0.0.0" || displayHost == "[::]" || displayHost == "127.0.0.1" || displayHost == "localhost" || displayHost == "::1" {
			displayHost = getPreferredIP()
		}
		ctx.LogInfo("starting file server on http://%s:%d serving directory %s", displayHost, s.cfg.Port, s.cfg.Root)
		if s.cfg.ShowQR {
			s.printQRCode(displayHost)
		}
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

func getPreferredIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Fallback to loopback or basic interface lookup
		addrs, err := net.InterfaceAddrs()
		if err == nil {
			for _, address := range addrs {
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						return ipnet.IP.String()
					}
				}
			}
		}
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func (s *Server) printQRCode(displayHost string) {
	url := fmt.Sprintf("http://%s:%d", displayHost, s.cfg.Port)

	code, err := qr.Encode(url, qr.L)
	if err != nil {
		fmt.Printf("Failed to generate QR code: %v\n", err)
		return
	}

	fmt.Println("\nScan the QR code below to open the server in your browser:")

	const (
		ansiReset = "\033[0m"
		ansiBlack = "\033[40m  "
		ansiWhite = "\033[47m  "
		quietZone = 2
	)

	for y := -quietZone; y < code.Size+quietZone; y++ {
		for x := -quietZone; x < code.Size+quietZone; x++ {
			if y < 0 || y >= code.Size || x < 0 || x >= code.Size {
				fmt.Print(ansiWhite)
			} else {
				if code.Black(x, y) {
					fmt.Print(ansiBlack)
				} else {
					fmt.Print(ansiWhite)
				}
			}
		}
		fmt.Print(ansiReset + "\n")
	}
	fmt.Println()
}
