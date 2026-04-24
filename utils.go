package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func respondJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func respondJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

func singleJoin(basePath, childPath string) string {
	basePath = strings.TrimRight(basePath, "/")
	childPath = "/" + strings.TrimLeft(childPath, "/")
	if basePath == "" {
		return childPath
	}
	if childPath == "/" {
		return basePath
	}
	return basePath + childPath
}

func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	cp := *u
	return &cp
}

func safeText(raw []byte) string {
	text := string(raw)
	text = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 500 {
		return text[:500] + "..."
	}
	return text
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if secs, err := strconv.Atoi(value); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(value); err == nil {
		return time.Until(t)
	}
	return 0
}

func bearerFromHeaders(h http.Header) string {
	auth := strings.TrimSpace(h.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}
	return ""
}

func rewriteWebSocketBearer(headers http.Header, token string) {
	if token == "" {
		return
	}
	const prefix = "openai-insecure-api-key."
	values := headers.Values("Sec-WebSocket-Protocol")
	if len(values) == 0 {
		return
	}
	headers.Del("Sec-WebSocket-Protocol")
	for _, value := range values {
		parts := strings.Split(value, ",")
		for i, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, prefix) {
				part = prefix + token
			}
			parts[i] = part
		}
		headers.Add("Sec-WebSocket-Protocol", strings.Join(parts, ", "))
	}
}

func isUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "Upgrade") || strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func publicBaseURL(cfg config, r *http.Request) string {
	if cfg.PublicURL != "" {
		return cfg.PublicURL
	}
	scheme := "http"
	if r != nil && r.TLS != nil {
		scheme = "https"
	}
	host := cfg.ListenAddr
	if r != nil && strings.TrimSpace(r.Host) != "" {
		host = r.Host
	}
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}
