package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/lnoxsian/gophrdrv/internal/templates"
)

// LoginHandler handles password authentication for private mode
func (h *HandlerContext) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if !h.Cfg.Private {
		// If not in private mode, redirect to home
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		// Render lock screen
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := map[string]interface{}{
			"Redirect": r.URL.Query().Get("redirect"),
			"Error":    "",
		}
		if err := templates.ExecuteTemplate(w, "lock.html", data); err != nil {
			h.LogError("Failed to render lock screen: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if r.Method != http.MethodPost {
		h.RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Only GET and POST requests are allowed for this endpoint.")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.RenderError(w, http.StatusBadRequest, "Bad Request", "Failed to parse login form.")
		return
	}

	password := r.FormValue("password")
	redirect := r.FormValue("redirect")
	if redirect == "" || !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
		redirect = "/"
	}

	passwdHash := sha256.Sum256([]byte(password))
	cfgPasswdHash := sha256.Sum256([]byte(h.Cfg.Password))

	if subtle.ConstantTimeCompare(passwdHash[:], cfgPasswdHash[:]) == 1 {
		// Correct password - set session cookie
		cookie := &http.Cookie{
			Name:     "gophrdrv_session",
			Value:    h.SessionToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(24 * time.Hour),
		}
		http.SetCookie(w, cookie)
		h.LogInfo("Successful login from %s", r.RemoteAddr)
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	// Incorrect password - render lock screen with error
	h.LogError("Failed login attempt from %s", r.RemoteAddr)
	time.Sleep(1 * time.Second) // Mitigate brute force attacks
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"Redirect": redirect,
		"Error":    "Incorrect password. Please try again.",
	}
	w.WriteHeader(http.StatusUnauthorized)
	if err := templates.ExecuteTemplate(w, "lock.html", data); err != nil {
		h.LogError("Failed to render lock screen: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

// LogoutHandler logs the user out by clearing the session cookie
func (h *HandlerContext) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     "gophrdrv_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	}
	http.SetCookie(w, cookie)
	h.LogInfo("User logged out")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
