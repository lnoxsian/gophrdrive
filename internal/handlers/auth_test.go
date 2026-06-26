package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lnoxsian/gophrdrv/internal/config"
)

func setupPrivateTestContext(t *testing.T) (*HandlerContext, string) {
	tmpDir, err := os.MkdirTemp("", "handlers-private-test-root")
	if err != nil {
		t.Fatalf("failed to create temp root: %v", err)
	}

	cfg := &config.Config{
		Root:         tmpDir,
		Port:         8080,
		Host:         "0.0.0.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		MaxUpload:    10 * 1024 * 1024,
		Private:      true,
		Password:     "testpass",
	}

	ctx := NewHandlerContext(cfg)
	return ctx, tmpDir
}

func TestLoginHandler_NotPrivate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "handlers-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Root:    tmpDir,
		Private: false,
	}
	ctx := NewHandlerContext(cfg)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	ctx.LoginHandler(w, req)

	if w.Result().StatusCode != http.StatusSeeOther {
		t.Errorf("expected status SeeOther (303), got %v", w.Result().StatusCode)
	}
	if loc := w.Result().Header.Get("Location"); loc != "/" {
		t.Errorf("expected redirect to '/', got %q", loc)
	}
}

func TestLoginHandler_Get(t *testing.T) {
	ctx, tmpDir := setupPrivateTestContext(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	ctx.LoginHandler(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "GOPHRDRV Private") {
		t.Errorf("expected body to contain 'GOPHRDRV Private'")
	}
}

func TestLoginHandler_Post_CorrectPassword(t *testing.T) {
	ctx, tmpDir := setupPrivateTestContext(t)
	defer os.RemoveAll(tmpDir)

	form := url.Values{}
	form.Add("password", "testpass")
	form.Add("redirect", "/browse?path=foo")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	ctx.LoginHandler(w, req)

	if w.Result().StatusCode != http.StatusSeeOther {
		t.Errorf("expected status SeeOther (303), got %v", w.Result().StatusCode)
	}
	if loc := w.Result().Header.Get("Location"); loc != "/browse?path=foo" {
		t.Errorf("expected redirect to '/browse?path=foo', got %q", loc)
	}

	// Verify cookie
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "gophrdrv_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}
	if sessionCookie.Value != ctx.SessionToken {
		t.Errorf("expected cookie value to match session token %q, got %q", ctx.SessionToken, sessionCookie.Value)
	}
}

func TestLoginHandler_Post_IncorrectPassword(t *testing.T) {
	ctx, tmpDir := setupPrivateTestContext(t)
	defer os.RemoveAll(tmpDir)

	form := url.Values{}
	form.Add("password", "wrongpass")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	ctx.LoginHandler(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status Unauthorized (401), got %v", w.Result().StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Incorrect password. Please try again.") {
		t.Errorf("expected body to contain error message")
	}
}

func TestLogoutHandler(t *testing.T) {
	ctx, tmpDir := setupPrivateTestContext(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	w := httptest.NewRecorder()
	ctx.LogoutHandler(w, req)

	if w.Result().StatusCode != http.StatusSeeOther {
		t.Errorf("expected status SeeOther (303), got %v", w.Result().StatusCode)
	}
	if loc := w.Result().Header.Get("Location"); loc != "/" {
		t.Errorf("expected redirect to '/', got %q", loc)
	}

	// Verify cookie is cleared
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "gophrdrv_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}
	if sessionCookie.MaxAge != -1 {
		t.Errorf("expected cookie MaxAge -1 (cleared), got %v", sessionCookie.MaxAge)
	}
}
