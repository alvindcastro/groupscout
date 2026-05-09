package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const adminSessionCookieName = "groupscout_session"

type adminAuthConfig struct {
	Enabled        bool
	SetupToken     string
	SetupTokenFile string
	SessionTTL     time.Duration
	Logger         *slog.Logger
}

type adminAuthenticator struct {
	enabled        bool
	setupTokenHash [32]byte
	setupTokenFile string
	setupTokenEnv  bool
	sessionTTL     time.Duration
	now            func() time.Time
	log            *slog.Logger
	mu             sync.Mutex
	sessions       map[string]time.Time
}

type adminContextKey struct{}

func newAdminAuthenticator(cfg adminAuthConfig) (*adminAuthenticator, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 24 * time.Hour
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	setupToken := strings.TrimSpace(cfg.SetupToken)
	setupTokenEnv := setupToken != ""
	if setupToken == "" {
		token, err := readOrCreateSetupToken(cfg.SetupTokenFile)
		if err != nil {
			return nil, err
		}
		setupToken = token
		cfg.Logger.Warn("initial admin setup token is required for first login", "setup_token_file", cfg.SetupTokenFile)
	} else {
		cfg.Logger.Info("initial admin setup token loaded from ADMIN_SETUP_TOKEN")
	}

	if setupToken == "" {
		return nil, fmt.Errorf("admin setup token is empty")
	}

	return &adminAuthenticator{
		enabled:        true,
		setupTokenHash: sha256.Sum256([]byte(setupToken)),
		setupTokenFile: cfg.SetupTokenFile,
		setupTokenEnv:  setupTokenEnv,
		sessionTTL:     cfg.SessionTTL,
		now:            time.Now,
		log:            cfg.Logger,
		sessions:       map[string]time.Time{},
	}, nil
}

func readOrCreateSetupToken(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("ADMIN_SETUP_TOKEN_FILE must not be empty")
	}
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data)), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read admin setup token: %w", err)
	}

	token, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create admin setup token directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write admin setup token: %w", err)
	}
	return token, nil
}

func (a *adminAuthenticator) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"auth_required":    a != nil && a.enabled,
		"authenticated":    a != nil && a.validRequestSession(r),
		"setup_token_file": a.setupTokenFile,
	})
}

func (a *adminAuthenticator) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !a.validSetupToken(req.Token) {
		writeJSONError(w, http.StatusUnauthorized, "invalid setup token")
		return
	}
	setupTokenRotated, err := a.rotateSetupTokenAfterLogin()
	if err != nil {
		a.log.Error("failed to rotate admin setup token", "error", err, "setup_token_file", a.setupTokenFile)
		writeJSONError(w, http.StatusInternalServerError, "rotate setup token failed")
		return
	}
	sessionToken, expiresAt, err := a.createSession()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "create admin session failed")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"session_token":       sessionToken,
		"token_type":          "Bearer",
		"expires_at":          expiresAt,
		"setup_token_rotated": setupTokenRotated,
		"user":                map[string]string{"role": "admin"},
	})
}

func (a *adminAuthenticator) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := a.requestSessionToken(r)
	if token != "" {
		a.revokeSession(token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"revoked": token != ""})
}

func (a *adminAuthenticator) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.validRequestSession(r) {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": map[string]string{"role": "admin"}})
}

func (a *adminAuthenticator) requireSession(next http.Handler) http.Handler {
	if a == nil || !a.enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.validRequestSession(r) {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminContextKey{}, true)))
	})
}

func (a *adminAuthenticator) validSetupToken(token string) bool {
	candidate := sha256.Sum256([]byte(strings.TrimSpace(token)))
	a.mu.Lock()
	defer a.mu.Unlock()
	return subtle.ConstantTimeCompare(candidate[:], a.setupTokenHash[:]) == 1
}

func (a *adminAuthenticator) rotateSetupTokenAfterLogin() (bool, error) {
	if a.setupTokenEnv {
		a.log.Warn("admin setup token loaded from ADMIN_SETUP_TOKEN cannot be rotated automatically")
		return false, nil
	}
	if strings.TrimSpace(a.setupTokenFile) == "" {
		return false, nil
	}
	token, err := randomToken()
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(a.setupTokenFile, []byte(token+"\n"), 0o600); err != nil {
		return false, fmt.Errorf("write rotated admin setup token: %w", err)
	}
	a.mu.Lock()
	a.setupTokenHash = sha256.Sum256([]byte(token))
	a.mu.Unlock()
	a.log.Info("admin setup token rotated after successful login", "setup_token_file", a.setupTokenFile)
	return true, nil
}

func (a *adminAuthenticator) createSession() (string, time.Time, error) {
	sessionToken, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := a.now().Add(a.sessionTTL)
	a.mu.Lock()
	a.sessions[sessionToken] = expiresAt
	a.mu.Unlock()
	return sessionToken, expiresAt, nil
}

func (a *adminAuthenticator) validRequestSession(r *http.Request) bool {
	if a == nil || !a.enabled {
		return true
	}
	return a.validSession(a.requestSessionToken(r))
}

func (a *adminAuthenticator) requestSessionToken(r *http.Request) string {
	token := bearerToken(r.Header.Get("Authorization"))
	if token != "" {
		return token
	}
	if cookie, err := r.Cookie(adminSessionCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

func (a *adminAuthenticator) validSession(token string) bool {
	if token == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	expiresAt, ok := a.sessions[token]
	if !ok {
		return false
	}
	if !a.now().Before(expiresAt) {
		delete(a.sessions, token)
		return false
	}
	return true
}

func (a *adminAuthenticator) revokeSession(token string) {
	if token == "" {
		return
	}
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

func bearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func requestHasAdminSession(r *http.Request) bool {
	value, ok := r.Context().Value(adminContextKey{}).(bool)
	return ok && value
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
