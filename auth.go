package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	codexOAuthClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexOAuthRedirectURI = "http://localhost:1455/auth/callback"
)

type oauthSession struct {
	Verifier  string
	State     string
	CreatedAt time.Time
}

var oauthSessions = struct {
	sync.Mutex
	byState map[string]oauthSession
}{byState: map[string]oauthSession{}}

var loopbackCallback = struct {
	sync.Mutex
	started bool
}{}

func (s *server) checkAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.AdminToken == "" {
		http.Error(w, "admin token is not configured", http.StatusForbidden)
		return false
	}
	token := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("admin_token"))
	}
	if token != s.cfg.AdminToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *server) checkOperator(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.AdminToken != "" {
		return s.checkAdmin(w, r)
	}
	if !isLoopbackRequest(r) {
		http.Error(w, "loopback access required; set CODEX_POOL_ADMIN_TOKEN for remote operator use", http.StatusForbidden)
		return false
	}
	return true
}

func (s *server) checkProxyAccess(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.SharedProxyToken == "" {
		return true
	}
	if strings.TrimSpace(r.Header.Get("X-Codex-Pool-Token")) == s.cfg.SharedProxyToken {
		return true
	}
	http.Error(w, "missing X-Codex-Pool-Token", http.StatusUnauthorized)
	return false
}

func isLoopbackRequest(r *http.Request) bool {
	if r == nil || hasForwardingHeader(r) {
		return false
	}
	return isLoopbackHost(r.Host) && isLoopbackAddr(r.RemoteAddr)
}

func hasForwardingHeader(r *http.Request) bool {
	for _, key := range []string{"Forwarded", "X-Forwarded-For", "X-Real-IP", "CF-Connecting-IP"} {
		if strings.TrimSpace(r.Header.Get(key)) != "" {
			return true
		}
	}
	return false
}

func isLoopbackHost(hostport string) bool {
	host := strings.TrimSpace(hostport)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isLoopbackAddr(addr string) bool {
	host := strings.TrimSpace(addr)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *server) startCodexOAuth(w http.ResponseWriter, r *http.Request) {
	if !s.checkOperator(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.ensureLoopbackCallbackServer(); err != nil {
		respondJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	verifier := randomURLSafe(32)
	challengeRaw := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeRaw[:])
	state := randomURLSafe(32)

	u := *s.cfg.AuthBase
	u.Path = singleJoin(s.cfg.AuthBase.Path, "/oauth/authorize")
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", codexOAuthClientID)
	q.Set("redirect_uri", codexOAuthRedirectURI)
	q.Set("scope", "openid profile email offline_access")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("originator", "codex_cli_rs")
	q.Set("state", state)
	u.RawQuery = q.Encode()

	oauthSessions.Lock()
	oauthSessions.byState[state] = oauthSession{Verifier: verifier, State: state, CreatedAt: time.Now()}
	oauthSessions.Unlock()

	respondJSON(w, map[string]any{
		"status":       "ok",
		"oauth_url":    u.String(),
		"callback_url": codexOAuthRedirectURI,
		"state":        state,
		"note":         "Complete sign-in in the browser. The localhost:1455 callback will save the account into pool/codex.",
	})
}

func (s *server) ensureLoopbackCallbackServer() error {
	loopbackCallback.Lock()
	defer loopbackCallback.Unlock()
	if loopbackCallback.started {
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		accountID, err := s.completeOAuthCallback(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "<!doctype html><title>Codex Pool OAuth</title><body><h1>Sign-in failed</h1><p>%s</p></body>", htmlEscape(err.Error()))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><title>Codex Pool OAuth</title><body><h1>Codex account saved</h1><p>Account %s is now in the pool. You can close this tab.</p></body>", htmlEscape(accountID))
	})
	srv := &http.Server{Addr: "127.0.0.1:1455", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("could not listen on %s for OAuth callback: %w", srv.Addr, err)
	}
	loopbackCallback.started = true
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("OAuth callback server stopped: %v", err)
		}
	}()
	return nil
}

func (s *server) completeOAuthCallback(ctx context.Context, code, state string) (string, error) {
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" || state == "" {
		return "", fmt.Errorf("missing code or state")
	}
	oauthSessions.Lock()
	session, ok := oauthSessions.byState[state]
	if ok {
		delete(oauthSessions.byState, state)
	}
	oauthSessions.Unlock()
	if !ok {
		return "", fmt.Errorf("unknown OAuth state")
	}
	if time.Since(session.CreatedAt) > 15*time.Minute {
		return "", fmt.Errorf("OAuth session expired")
	}

	tokens, err := s.exchangeOAuthCode(ctx, code, session.Verifier)
	if err != nil {
		return "", err
	}
	acc, err := s.saveOAuthAccount(tokens)
	if err != nil {
		return "", err
	}
	if err := s.reloadPool(); err != nil {
		log.Printf("warning: account saved but reload failed: %v", err)
	}
	return acc.ID, nil
}

func (s *server) exchangeOAuthCode(ctx context.Context, code, verifier string) (*codexTokens, error) {
	body := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  codexOAuthRedirectURI,
		"client_id":     codexOAuthClientID,
		"code_verifier": verifier,
	}
	bodyJSON, _ := json.Marshal(body)
	u := *s.cfg.AuthBase
	u.Path = singleJoin(s.cfg.AuthBase.Path, "/oauth/token")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange failed: %s: %s", resp.Status, safeText(raw))
	}
	var out codexTokens
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return nil, fmt.Errorf("token exchange returned no access token")
	}
	return &out, nil
}

func (s *server) saveOAuthAccount(tokens *codexTokens) (*account, error) {
	claims := parseCodexClaims(tokens.IDToken)
	accountID := claims.ChatGPTAccountID
	if tokens.AccountID != nil && strings.TrimSpace(*tokens.AccountID) != "" {
		accountID = strings.TrimSpace(*tokens.AccountID)
	}
	id := firstNonEmpty(accountID, claims.Subject, accountIDFromOpaque(tokens.AccessToken, "codex"))
	id = sanitizeFileStem(id)
	path := filepath.Join(s.cfg.PoolDir, "codex", id+".json")
	now := time.Now().UTC()
	root := map[string]any{
		"tokens": map[string]any{
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"id_token":      tokens.IDToken,
		},
		"auth_mode":     "chatgpt",
		"plan_type":     firstNonEmpty(claims.PlanType, "chatgpt"),
		"last_refresh":  now.Format(time.RFC3339),
		"health_status": "healthy",
	}
	if accountID != "" {
		root["tokens"].(map[string]any)["account_id"] = accountID
	}
	if err := atomicWriteJSON(path, root); err != nil {
		return nil, err
	}
	return loadAccountFile(path)
}

func (s *server) saveAPIKey(w http.ResponseWriter, r *http.Request) {
	if !s.checkOperator(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	key := strings.TrimSpace(payload.APIKey)
	if key == "" {
		respondJSONError(w, http.StatusBadRequest, "api_key is required")
		return
	}
	if !strings.HasPrefix(key, "sk-") {
		respondJSONError(w, http.StatusBadRequest, "api_key must start with sk-")
		return
	}
	id := accountIDFromOpaque(key, "openai_api")
	path := filepath.Join(s.cfg.PoolDir, "openai_api", sanitizeFileStem(id)+".json")
	root := map[string]any{
		"OPENAI_API_KEY": key,
		"auth_mode":      "api_key",
		"plan_type":      "api",
		"health_status":  "loaded",
	}
	if err := atomicWriteJSON(path, root); err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.reloadPool(); err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, map[string]any{"status": "ok", "account_id": id})
}

func (s *server) refreshIfNeeded(ctx context.Context, acc *account) error {
	if acc == nil || acc.Kind != accountKindCodexOAuth || s.cfg.DisableRefresh {
		return nil
	}
	acc.mu.Lock()
	needsRefresh := acc.RefreshToken != "" && (acc.ExpiresAt.IsZero() || time.Until(acc.ExpiresAt) < 2*time.Minute)
	refreshToken := acc.RefreshToken
	acc.mu.Unlock()
	if !needsRefresh {
		return nil
	}
	body := map[string]string{
		"client_id":     codexOAuthClientID,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"scope":         "openid profile email",
	}
	bodyJSON, _ := json.Marshal(body)
	u := *s.cfg.AuthBase
	u.Path = singleJoin(s.cfg.AuthBase.Path, "/oauth/token")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		acc.mu.Lock()
		acc.HealthStatus = "refresh_failed"
		acc.HealthError = safeText(raw)
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			acc.Dead = true
		}
		acc.mu.Unlock()
		_ = saveAccount(acc)
		return fmt.Errorf("refresh failed for %s: %s", acc.ID, resp.Status)
	}
	var tokens codexTokens
	if err := json.Unmarshal(raw, &tokens); err != nil {
		return err
	}
	claims := parseCodexClaims(tokens.IDToken)
	now := time.Now().UTC()
	acc.mu.Lock()
	acc.AccessToken = firstNonEmpty(tokens.AccessToken, acc.AccessToken)
	acc.RefreshToken = firstNonEmpty(tokens.RefreshToken, acc.RefreshToken)
	acc.IDToken = firstNonEmpty(tokens.IDToken, acc.IDToken)
	if tokens.AccountID != nil && strings.TrimSpace(*tokens.AccountID) != "" {
		acc.AccountID = strings.TrimSpace(*tokens.AccountID)
	}
	if acc.AccountID == "" {
		acc.AccountID = claims.ChatGPTAccountID
	}
	if !claims.ExpiresAt.IsZero() {
		acc.ExpiresAt = claims.ExpiresAt
	}
	acc.Email = firstNonEmpty(claims.Email, acc.Email)
	acc.PlanType = firstNonEmpty(claims.PlanType, acc.PlanType)
	acc.LastRefresh = now
	acc.HealthStatus = "healthy"
	acc.HealthError = ""
	acc.Dead = false
	acc.mu.Unlock()
	return saveAccount(acc)
}

func randomURLSafe(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func sanitizeFileStem(value string) string {
	value = strings.TrimSpace(value)
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			out.WriteRune(r)
		default:
			out.WriteByte('_')
		}
	}
	if out.Len() == 0 {
		return "account"
	}
	return out.String()
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")
	return replacer.Replace(value)
}

func importLocalCodexAuth(dstDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	src := filepath.Join(home, ".codex", "auth.json")
	raw, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if acc, err := loadAccountBytes("imported.json", src, raw); err != nil || acc == nil || acc.Kind != accountKindCodexOAuth {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("%s is not a Codex OAuth auth file", src)
	}
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return "", err
	}
	dst := filepath.Join(dstDir, "local-codex.json")
	if err := os.WriteFile(dst, raw, 0o600); err != nil {
		return "", err
	}
	return dst, nil
}

func loadAccountBytes(name, path string, data []byte) (*account, error) {
	tmp, err := os.CreateTemp("", "codex-pool-account-*.json")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	_, _ = tmp.Write(data)
	_ = tmp.Close()
	defer os.Remove(tmpPath)
	acc, err := loadAccountFile(tmpPath)
	if acc != nil {
		acc.ID = strings.TrimSuffix(name, filepath.Ext(name))
		acc.File = path
	}
	return acc, err
}
