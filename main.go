package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

const version = "0.11.1-codex-only"

func main() {
	cfg := loadConfig()
	pool, err := loadPool(cfg.PoolDir)
	if err != nil {
		log.Fatalf("load pool: %v", err)
	}
	if pool.count() == 0 {
		log.Printf("warning: no Codex/OpenAI accounts found under %s", cfg.PoolDir)
	}

	handler := &server{
		cfg:       cfg,
		pool:      pool,
		client:    newHTTPClient(cfg),
		startTime: time.Now(),
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       5 * time.Minute,
	}

	log.Printf("codex-pool listening on %s (accounts=%d, oauth=%d, api_keys=%d)", cfg.ListenAddr, pool.count(), pool.countKind(accountKindCodexOAuth), pool.countKind(accountKindOpenAIAPI))
	if cfg.AdminToken == "" {
		log.Printf("local operator endpoints require loopback access; set CODEX_POOL_ADMIN_TOKEN for remote administration")
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

type server struct {
	cfg       config
	pool      *poolState
	client    *http.Client
	startTime time.Time
}

func newHTTPClient(cfg config) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   50,
	}
	return &http.Client{Transport: transport}
}

func (s *server) reloadPool() error {
	pool, err := loadPool(s.cfg.PoolDir)
	if err != nil {
		return err
	}
	s.pool = pool
	return nil
}

func shutdownWithTimeout(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func envOrEmpty(key string) string {
	return os.Getenv(key)
}
