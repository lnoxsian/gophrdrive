package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/lnoxsian/gophrdrv/internal/config"
	"github.com/lnoxsian/gophrdrv/internal/templates"
)

type HandlerContext struct {
	Cfg          *config.Config
	SessionToken string
}

type ErrorData struct {
	StatusCode int
	Message    string
	Detail     string
}

func NewHandlerContext(cfg *config.Config) *HandlerContext {
	var sessionToken string
	if cfg.Private {
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err == nil {
			sessionToken = hex.EncodeToString(bytes)
		} else {
			sessionToken = fmt.Sprintf("fallback_token_%d", time.Now().UnixNano())
		}
	}
	return &HandlerContext{
		Cfg:          cfg,
		SessionToken: sessionToken,
	}
}

// Log logs messages in the format: timestamp level message
func (h *HandlerContext) Log(level, format string, a ...interface{}) {
	t := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s %s %s\n", t, level, msg)
}

func (h *HandlerContext) LogInfo(format string, a ...interface{}) {
	h.Log("INFO", format, a...)
}

func (h *HandlerContext) LogError(format string, a ...interface{}) {
	h.Log("ERROR", format, a...)
}

// RenderError renders a friendly HTML error page
func (h *HandlerContext) RenderError(w http.ResponseWriter, status int, msg, detail string) {
	h.LogError("HTTP %d: %s (%s)", status, msg, detail)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	data := ErrorData{
		StatusCode: status,
		Message:    msg,
		Detail:     detail,
	}

	err := templates.ExecuteTemplate(w, "error.html", data)
	if err != nil {
		// Fallback to plain text if template execution fails
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, fmt.Sprintf("%d %s: %s", status, msg, detail), status)
	}
}
