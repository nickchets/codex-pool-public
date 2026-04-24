package main

import (
	"net/http"
	"strings"
	"time"
)

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/":
		s.serveHome(w, r)
	case r.URL.Path == "/healthz":
		respondJSON(w, map[string]any{"status": "ok", "version": version})
	case r.URL.Path == "/status":
		s.serveStatus(w, r)
	case r.URL.Path == "/config/codex.toml":
		s.serveCodexConfig(w, r)
	case r.URL.Path == "/setup/codex.sh":
		s.serveShellSetup(w, r)
	case r.URL.Path == "/setup/codex.ps1":
		s.servePowerShellSetup(w, r)
	case r.URL.Path == "/admin/reload":
		s.handleReload(w, r)
	case r.URL.Path == "/admin/accounts":
		s.handleAccounts(w, r)
	case r.URL.Path == "/operator/codex/oauth-start":
		s.startCodexOAuth(w, r)
	case r.URL.Path == "/operator/codex/api-key-add":
		s.saveAPIKey(w, r)
	case r.URL.Path == "/oauth/token":
		s.fakeOAuthToken(w, r)
	case isProxyPath(r.URL.Path):
		s.proxyCodex(w, r)
	default:
		http.NotFound(w, r)
	}
}

func isProxyPath(path string) bool {
	return strings.HasPrefix(path, "/v1/") ||
		strings.HasPrefix(path, "/responses") ||
		strings.HasPrefix(path, "/backend-api/") ||
		strings.HasPrefix(path, "/ws") ||
		strings.HasPrefix(path, "/api/codex/")
}

func (s *server) handleReload(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.reloadPool(); err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, map[string]any{"status": "ok", "accounts": s.pool.count()})
}

func (s *server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	respondJSON(w, s.pool.summaries(time.Now().UTC()))
}
