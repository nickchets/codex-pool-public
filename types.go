package main

import (
	"sync"
	"sync/atomic"
	"time"
)

type accountKind string

const (
	accountKindCodexOAuth accountKind = "codex_oauth"
	accountKindOpenAIAPI  accountKind = "openai_api"
)

type account struct {
	ID              string      `json:"id"`
	Kind            accountKind `json:"kind"`
	File            string      `json:"file"`
	AccessToken     string      `json:"-"`
	RefreshToken    string      `json:"-"`
	IDToken         string      `json:"-"`
	AccountID       string      `json:"account_id,omitempty"`
	Email           string      `json:"email,omitempty"`
	PlanType        string      `json:"plan_type,omitempty"`
	Disabled        bool        `json:"disabled"`
	Dead            bool        `json:"dead"`
	HealthStatus    string      `json:"health_status,omitempty"`
	HealthError     string      `json:"health_error,omitempty"`
	ExpiresAt       time.Time   `json:"expires_at,omitempty"`
	LastRefresh     time.Time   `json:"last_refresh,omitempty"`
	LastUsed        time.Time   `json:"last_used,omitempty"`
	RateLimitUntil  time.Time   `json:"rate_limit_until,omitempty"`
	RequestCount    int64       `json:"request_count"`
	ErrorCount      int64       `json:"error_count"`
	LastStatusCode  int         `json:"last_status_code,omitempty"`
	LastStatusAt    time.Time   `json:"last_status_at,omitempty"`
	LastUsedPercent float64     `json:"last_used_percent,omitempty"`
	Inflight        int64       `json:"inflight"`
	mu              sync.Mutex
}

func (a *account) inflight() int64 {
	if a == nil {
		return 0
	}
	return atomic.LoadInt64(&a.Inflight)
}

func (a *account) addInflight(delta int64) {
	if a != nil {
		atomic.AddInt64(&a.Inflight, delta)
	}
}

func (a *account) recordStatus(code int) {
	if a == nil {
		return
	}
	now := time.Now().UTC()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.LastStatusCode = code
	a.LastStatusAt = now
	a.LastUsed = now
	a.RequestCount++
	if code >= 400 {
		a.ErrorCount++
	}
}

type statusPayload struct {
	Version         string           `json:"version"`
	GeneratedAt     time.Time        `json:"generated_at"`
	UptimeSeconds   int64            `json:"uptime_seconds"`
	ListenAddr      string           `json:"listen_addr"`
	PoolDir         string           `json:"pool_dir"`
	AccountCount    int              `json:"account_count"`
	OAuthCount      int              `json:"oauth_count"`
	APIKeyCount     int              `json:"api_key_count"`
	EligibleCount   int              `json:"eligible_count"`
	LocalOperator   bool             `json:"local_operator"`
	SharedProxyAuth bool             `json:"shared_proxy_auth"`
	Accounts        []accountSummary `json:"accounts"`
	Setup           setupLinks       `json:"setup"`
}

type accountSummary struct {
	ID             string      `json:"id"`
	Kind           accountKind `json:"kind"`
	AccountID      string      `json:"account_id,omitempty"`
	Email          string      `json:"email,omitempty"`
	PlanType       string      `json:"plan_type,omitempty"`
	Disabled       bool        `json:"disabled"`
	Dead           bool        `json:"dead"`
	Eligible       bool        `json:"eligible"`
	BlockReason    string      `json:"block_reason,omitempty"`
	ExpiresAt      string      `json:"expires_at,omitempty"`
	LastRefresh    string      `json:"last_refresh,omitempty"`
	LastUsed       string      `json:"last_used,omitempty"`
	RateLimitUntil string      `json:"rate_limit_until,omitempty"`
	Inflight       int64       `json:"inflight"`
	RequestCount   int64       `json:"request_count"`
	ErrorCount     int64       `json:"error_count"`
	LastStatusCode int         `json:"last_status_code,omitempty"`
	HealthStatus   string      `json:"health_status,omitempty"`
	HealthError    string      `json:"health_error,omitempty"`
}

type setupLinks struct {
	CodexConfig string `json:"codex_config"`
	ShellScript string `json:"shell_script"`
	PowerShell  string `json:"powershell"`
}
