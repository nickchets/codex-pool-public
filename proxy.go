package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const maxBufferedRequest = 16 * 1024 * 1024

func (s *server) proxyCodex(w http.ResponseWriter, r *http.Request) {
	if !s.checkProxyAccess(w, r) {
		return
	}

	body, conversationKey, err := readProxyBodyAndKey(r)
	if err != nil {
		respondJSONError(w, http.StatusRequestEntityTooLarge, err.Error())
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	acc, err := s.pool.choose(r.URL.Path, conversationKey, time.Now())
	if err != nil {
		respondJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if err := s.refreshIfNeeded(r.Context(), acc); err != nil {
		log.Printf("warning: refresh before request failed for %s: %v", acc.ID, err)
	}

	target, upstreamPath, err := s.upstreamForAccount(acc, r.URL.Path)
	if err != nil {
		respondJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	acc.addInflight(1)
	defer acc.addInflight(-1)

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			out := pr.Out
			out.URL.Scheme = target.Scheme
			out.URL.Host = target.Host
			out.URL.Path = singleJoin(target.Path, upstreamPath)
			out.URL.RawPath = ""
			out.Host = target.Host
			out.Body = io.NopCloser(bytes.NewReader(body))
			out.ContentLength = int64(len(body))
			if len(body) == 0 {
				out.Body = http.NoBody
				out.ContentLength = 0
			}
			pr.SetXForwarded()
			stripHopHeaders(out.Header)
			setAccountAuth(out.Header, acc)
			rewriteWebSocketBearer(out.Header, acc.AccessToken)
		},
		ModifyResponse: func(resp *http.Response) error {
			acc.recordStatus(resp.StatusCode)
			s.applyResponseHealth(acc, resp)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			acc.recordStatus(http.StatusBadGateway)
			log.Printf("proxy error for %s via %s: %v", r.URL.Path, acc.ID, err)
			respondJSONError(w, http.StatusBadGateway, "upstream proxy error")
		},
		FlushInterval: 100 * time.Millisecond,
	}
	if isUpgrade(r) {
		proxy.FlushInterval = -1
	}
	proxy.ServeHTTP(w, r)
}

func readProxyBodyAndKey(r *http.Request) ([]byte, string, error) {
	key := conversationKeyFromRequest(r, nil)
	if r.Body == nil || r.Body == http.NoBody {
		return nil, key, nil
	}
	limited := io.LimitReader(r.Body, maxBufferedRequest+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if len(body) > maxBufferedRequest {
		return nil, "", fmt.Errorf("request body exceeds %d bytes", maxBufferedRequest)
	}
	if key == "" {
		key = conversationKeyFromRequest(r, body)
	}
	return body, key, nil
}

func conversationKeyFromRequest(r *http.Request, body []byte) string {
	if r == nil {
		return ""
	}
	for _, value := range []string{
		r.URL.Query().Get("session_id"),
		r.URL.Query().Get("conversation_id"),
		r.Header.Get("X-Codex-Session"),
		r.Header.Get("OpenAI-Conversation-ID"),
		r.Header.Get("X-OpenAI-Conversation-ID"),
	} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if len(body) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"conversation_id", "thread_id", "session_id", "previous_response_id", "prompt_cache_key"} {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *server) upstreamForAccount(acc *account, reqPath string) (*url.URL, string, error) {
	if acc == nil {
		return nil, "", fmt.Errorf("missing account")
	}
	if acc.Kind == accountKindOpenAIAPI {
		if strings.HasPrefix(reqPath, "/responses") {
			return cloneURL(s.cfg.APIBase), "/v1" + reqPath, nil
		}
		if strings.HasPrefix(reqPath, "/v1/") {
			return cloneURL(s.cfg.APIBase), reqPath, nil
		}
		return nil, "", fmt.Errorf("OpenAI API key accounts only support /v1 and /responses paths")
	}

	switch {
	case strings.HasPrefix(reqPath, "/backend-api/"):
		return cloneURL(s.cfg.BackendBase), strings.TrimPrefix(reqPath, "/backend-api"), nil
	case strings.HasPrefix(reqPath, "/v1/responses/compact"):
		return cloneURL(s.cfg.CodexBase), "/responses/compact", nil
	case strings.HasPrefix(reqPath, "/v1/responses"):
		return cloneURL(s.cfg.CodexBase), "/responses", nil
	case strings.HasPrefix(reqPath, "/responses/compact"):
		return cloneURL(s.cfg.CodexBase), "/responses/compact", nil
	case strings.HasPrefix(reqPath, "/responses"):
		return cloneURL(s.cfg.CodexBase), "/responses", nil
	case strings.HasPrefix(reqPath, "/ws"):
		return cloneURL(s.cfg.CodexBase), reqPath, nil
	case strings.HasPrefix(reqPath, "/v1/"):
		return cloneURL(s.cfg.APIBase), reqPath, nil
	default:
		return cloneURL(s.cfg.CodexBase), reqPath, nil
	}
}

func setAccountAuth(headers http.Header, acc *account) {
	headers.Del("Authorization")
	headers.Del("ChatGPT-Account-ID")
	headers.Del("X-Api-Key")
	headers.Set("Authorization", "Bearer "+acc.AccessToken)
	if acc.Kind == accountKindCodexOAuth && acc.AccountID != "" {
		headers.Set("ChatGPT-Account-ID", acc.AccountID)
	}
}

func stripHopHeaders(headers http.Header) {
	for _, key := range []string{
		"Proxy-Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
	} {
		headers.Del(key)
	}
}

func (s *server) applyResponseHealth(acc *account, resp *http.Response) {
	if acc == nil || resp == nil {
		return
	}
	now := time.Now().UTC()
	shouldSave := false
	acc.mu.Lock()
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		wait := parseRetryAfter(resp.Header.Get("Retry-After"))
		if wait <= 0 {
			wait = 90 * time.Second
		}
		acc.RateLimitUntil = now.Add(wait)
		acc.HealthStatus = "rate_limited"
		acc.HealthError = "upstream returned 429"
		shouldSave = true
	case http.StatusUnauthorized, http.StatusForbidden:
		acc.HealthStatus = "auth_failed"
		acc.HealthError = resp.Status
		if acc.Kind == accountKindCodexOAuth {
			acc.Dead = true
		}
		shouldSave = true
	default:
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			acc.HealthStatus = "healthy"
			acc.HealthError = ""
		}
	}
	if value := resp.Header.Get("OpenAI-Processing-Ms"); value != "" {
		_ = value
	}
	acc.mu.Unlock()
	if shouldSave {
		if err := saveAccount(acc); err != nil {
			log.Printf("warning: failed to persist account health for %s: %v", acc.ID, err)
		}
	}
}

func (s *server) fakeOAuthToken(w http.ResponseWriter, r *http.Request) {
	if !s.checkProxyAccess(w, r) {
		return
	}
	acc, err := s.pool.choose("/backend-api/codex/models", "", time.Now())
	if err != nil {
		respondJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if acc.Kind != accountKindCodexOAuth {
		respondJSONError(w, http.StatusServiceUnavailable, "no ChatGPT OAuth account available")
		return
	}
	if err := s.refreshIfNeeded(context.Background(), acc); err != nil {
		respondJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	acc.mu.Lock()
	expiresIn := int64(3600)
	if !acc.ExpiresAt.IsZero() {
		expiresIn = int64(time.Until(acc.ExpiresAt).Seconds())
	}
	payload := map[string]any{
		"access_token":  acc.AccessToken,
		"refresh_token": acc.RefreshToken,
		"id_token":      acc.IDToken,
		"token_type":    "Bearer",
		"expires_in":    expiresIn,
	}
	acc.mu.Unlock()
	respondJSON(w, payload)
}
