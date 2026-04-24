package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type configFile struct {
	ListenAddr          string `toml:"listen_addr"`
	PoolDir             string `toml:"pool_dir"`
	PublicURL           string `toml:"public_url"`
	AdminToken          string `toml:"admin_token"`
	SharedProxyToken    string `toml:"shared_proxy_token"`
	DisableRefresh      bool   `toml:"disable_refresh"`
	MaxAttempts         int    `toml:"max_attempts"`
	RequestTimeoutSecs  int    `toml:"request_timeout_seconds"`
	StreamIdleSecs      int    `toml:"stream_idle_timeout_seconds"`
	UpstreamCodexBase   string `toml:"upstream_codex_base"`
	UpstreamBackendBase string `toml:"upstream_backend_base"`
	UpstreamAPIBase     string `toml:"upstream_openai_api_base"`
	UpstreamAuthBase    string `toml:"upstream_auth_base"`
}

type config struct {
	ListenAddr       string
	PoolDir          string
	PublicURL        string
	AdminToken       string
	SharedProxyToken string
	DisableRefresh   bool
	MaxAttempts      int
	RequestTimeout   time.Duration
	StreamIdle       time.Duration
	CodexBase        *url.URL
	BackendBase      *url.URL
	APIBase          *url.URL
	AuthBase         *url.URL
}

func loadConfig() config {
	var file configFile
	configPath := getString("CODEX_POOL_CONFIG", "", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		if _, err := toml.DecodeFile(configPath, &file); err != nil {
			log.Printf("warning: could not parse %s: %v", configPath, err)
		}
	}

	cfg := config{
		ListenAddr:       getString("CODEX_POOL_LISTEN_ADDR", file.ListenAddr, "127.0.0.1:8989"),
		PoolDir:          getString("CODEX_POOL_DIR", file.PoolDir, "pool"),
		PublicURL:        strings.TrimRight(getString("CODEX_POOL_PUBLIC_URL", file.PublicURL, ""), "/"),
		AdminToken:       getString("CODEX_POOL_ADMIN_TOKEN", file.AdminToken, ""),
		SharedProxyToken: getString("CODEX_POOL_SHARED_PROXY_TOKEN", file.SharedProxyToken, ""),
		DisableRefresh:   getBool("CODEX_POOL_DISABLE_REFRESH", file.DisableRefresh),
		MaxAttempts:      getInt("CODEX_POOL_MAX_ATTEMPTS", file.MaxAttempts, 3),
		RequestTimeout:   time.Duration(getInt("CODEX_POOL_REQUEST_TIMEOUT_SECONDS", file.RequestTimeoutSecs, 300)) * time.Second,
		StreamIdle:       time.Duration(getInt("CODEX_POOL_STREAM_IDLE_TIMEOUT_SECONDS", file.StreamIdleSecs, 600)) * time.Second,
		CodexBase:        mustURL(getString("CODEX_POOL_UPSTREAM_CODEX_BASE", file.UpstreamCodexBase, "https://chatgpt.com/backend-api/codex")),
		BackendBase:      mustURL(getString("CODEX_POOL_UPSTREAM_BACKEND_BASE", file.UpstreamBackendBase, "https://chatgpt.com/backend-api")),
		APIBase:          mustURL(getString("CODEX_POOL_UPSTREAM_OPENAI_API_BASE", file.UpstreamAPIBase, "https://api.openai.com")),
		AuthBase:         mustURL(getString("CODEX_POOL_UPSTREAM_AUTH_BASE", file.UpstreamAuthBase, "https://auth.openai.com")),
	}

	flag.StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr, "listen address")
	flag.StringVar(&cfg.PoolDir, "pool-dir", cfg.PoolDir, "directory containing Codex/OpenAI account JSON files")
	flag.StringVar(&cfg.PublicURL, "public-url", cfg.PublicURL, "public base URL used in generated config")
	flag.Parse()
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	return cfg
}

func getString(envKey, fileValue, defaultValue string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	if strings.TrimSpace(fileValue) != "" {
		return strings.TrimSpace(fileValue)
	}
	return defaultValue
}

func getInt(envKey string, fileValue, defaultValue int) int {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	if fileValue > 0 {
		return fileValue
	}
	return defaultValue
}

func getBool(envKey string, fileValue bool) bool {
	if v := strings.TrimSpace(strings.ToLower(os.Getenv(envKey))); v != "" {
		return v == "1" || v == "true" || v == "yes" || v == "on"
	}
	return fileValue
}

func mustURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		log.Fatalf("invalid URL %q: %v", raw, err)
	}
	return u
}
