package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type codexAuthFile struct {
	Tokens          *codexTokens `json:"tokens"`
	OpenAIAPIKey    string       `json:"OPENAI_API_KEY"`
	APIKey          string       `json:"api_key"`
	AuthMode        string       `json:"auth_mode"`
	PlanType        string       `json:"plan_type"`
	Disabled        bool         `json:"disabled"`
	Dead            bool         `json:"dead"`
	HealthStatus    string       `json:"health_status"`
	HealthError     string       `json:"health_error"`
	LastRefresh     *time.Time   `json:"last_refresh"`
	RateLimitUntil  *time.Time   `json:"rate_limit_until"`
	LastUsedPercent float64      `json:"last_used_percent"`
}

type codexTokens struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	IDToken      string  `json:"id_token"`
	AccountID    *string `json:"account_id,omitempty"`
}

type jwtClaims struct {
	ExpiresAt        time.Time
	ChatGPTAccountID string
	UserID           string
	PlanType         string
	Email            string
	Subject          string
}

type poolState struct {
	mu       sync.RWMutex
	accounts []*account
	sticky   map[string]string
	rr       int
}

func loadPool(poolDir string) (*poolState, error) {
	if err := os.MkdirAll(filepath.Join(poolDir, "codex"), 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(poolDir, "openai_api"), 0o700); err != nil {
		return nil, err
	}

	var accounts []*account
	err := filepath.WalkDir(poolDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", "data", "tmp", "dist", "bin":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		acc, err := loadAccountFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if acc != nil {
			accounts = append(accounts, acc)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })
	return &poolState{accounts: accounts, sticky: map[string]string{}}, nil
}

func loadAccountFile(path string) (*account, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw codexAuthFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	if raw.Tokens != nil && strings.TrimSpace(raw.Tokens.AccessToken) != "" {
		claims := parseCodexClaims(raw.Tokens.IDToken)
		accID := ""
		if raw.Tokens.AccountID != nil {
			accID = strings.TrimSpace(*raw.Tokens.AccountID)
		}
		if accID == "" {
			accID = claims.ChatGPTAccountID
		}
		plan := firstNonEmpty(strings.TrimSpace(raw.PlanType), claims.PlanType, "chatgpt")
		acc := &account{
			ID:              firstNonEmpty(name, accountIDFromOpaque(raw.Tokens.AccessToken, "codex")),
			Kind:            accountKindCodexOAuth,
			File:            path,
			AccessToken:     strings.TrimSpace(raw.Tokens.AccessToken),
			RefreshToken:    strings.TrimSpace(raw.Tokens.RefreshToken),
			IDToken:         strings.TrimSpace(raw.Tokens.IDToken),
			AccountID:       accID,
			Email:           claims.Email,
			PlanType:        plan,
			ExpiresAt:       claims.ExpiresAt,
			Disabled:        raw.Disabled,
			Dead:            raw.Dead,
			HealthStatus:    strings.TrimSpace(raw.HealthStatus),
			HealthError:     strings.TrimSpace(raw.HealthError),
			LastUsedPercent: raw.LastUsedPercent,
		}
		if raw.LastRefresh != nil {
			acc.LastRefresh = raw.LastRefresh.UTC()
		}
		if raw.RateLimitUntil != nil {
			acc.RateLimitUntil = raw.RateLimitUntil.UTC()
		}
		if acc.HealthStatus == "" {
			acc.HealthStatus = "loaded"
		}
		return acc, nil
	}

	key := strings.TrimSpace(firstNonEmpty(raw.OpenAIAPIKey, raw.APIKey))
	if key != "" {
		acc := &account{
			ID:              firstNonEmpty(name, accountIDFromOpaque(key, "openai_api")),
			Kind:            accountKindOpenAIAPI,
			File:            path,
			AccessToken:     key,
			PlanType:        firstNonEmpty(strings.TrimSpace(raw.PlanType), "api"),
			Disabled:        raw.Disabled,
			Dead:            raw.Dead,
			HealthStatus:    firstNonEmpty(strings.TrimSpace(raw.HealthStatus), "loaded"),
			HealthError:     strings.TrimSpace(raw.HealthError),
			LastUsedPercent: raw.LastUsedPercent,
		}
		if raw.RateLimitUntil != nil {
			acc.RateLimitUntil = raw.RateLimitUntil.UTC()
		}
		return acc, nil
	}
	return nil, nil
}

func (p *poolState) count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.accounts)
}

func (p *poolState) countKind(kind accountKind) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total := 0
	for _, acc := range p.accounts {
		if acc.Kind == kind {
			total++
		}
	}
	return total
}

func (p *poolState) summaries(now time.Time) []accountSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]accountSummary, 0, len(p.accounts))
	for _, acc := range p.accounts {
		acc.mu.Lock()
		eligible, reason := accountEligibleLocked(acc, now, "")
		row := accountSummary{
			ID:             acc.ID,
			Kind:           acc.Kind,
			AccountID:      acc.AccountID,
			Email:          acc.Email,
			PlanType:       acc.PlanType,
			Disabled:       acc.Disabled,
			Dead:           acc.Dead,
			Eligible:       eligible,
			BlockReason:    reason,
			Inflight:       acc.inflight(),
			RequestCount:   acc.RequestCount,
			ErrorCount:     acc.ErrorCount,
			LastStatusCode: acc.LastStatusCode,
			HealthStatus:   acc.HealthStatus,
			HealthError:    acc.HealthError,
		}
		row.ExpiresAt = formatTime(acc.ExpiresAt)
		row.LastRefresh = formatTime(acc.LastRefresh)
		row.LastUsed = formatTime(acc.LastUsed)
		row.RateLimitUntil = formatTime(acc.RateLimitUntil)
		acc.mu.Unlock()
		out = append(out, row)
	}
	return out
}

func (p *poolState) choose(path, conversationKey string, now time.Time) (*account, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if conversationKey != "" {
		if id := p.sticky[conversationKey]; id != "" {
			for _, acc := range p.accounts {
				if acc.ID == id && accountSupportsPath(acc, path) {
					acc.mu.Lock()
					ok, _ := accountEligibleLocked(acc, now, path)
					acc.mu.Unlock()
					if ok {
						return acc, nil
					}
				}
			}
		}
	}

	var candidates []*account
	for _, acc := range p.accounts {
		if !accountSupportsPath(acc, path) {
			continue
		}
		acc.mu.Lock()
		ok, _ := accountEligibleLocked(acc, now, path)
		acc.mu.Unlock()
		if ok {
			candidates = append(candidates, acc)
		}
	}
	if len(candidates) == 0 {
		return nil, errors.New("no eligible Codex/OpenAI account for request")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		ai, aj := candidates[i], candidates[j]
		if ai.inflight() != aj.inflight() {
			return ai.inflight() < aj.inflight()
		}
		ai.mu.Lock()
		iLast := ai.LastUsed
		iReq := ai.RequestCount
		ai.mu.Unlock()
		aj.mu.Lock()
		jLast := aj.LastUsed
		jReq := aj.RequestCount
		aj.mu.Unlock()
		if !iLast.Equal(jLast) {
			return iLast.Before(jLast)
		}
		return iReq < jReq
	})
	chosen := candidates[p.rr%len(candidates)]
	p.rr++
	if conversationKey != "" {
		p.sticky[conversationKey] = chosen.ID
	}
	return chosen, nil
}

func accountEligibleLocked(acc *account, now time.Time, path string) (bool, string) {
	switch {
	case acc == nil:
		return false, "missing"
	case acc.Disabled:
		return false, "disabled"
	case acc.Dead:
		return false, "dead"
	case !acc.RateLimitUntil.IsZero() && acc.RateLimitUntil.After(now):
		return false, "rate_limited"
	case acc.AccessToken == "":
		return false, "missing_access_token"
	case path != "" && !accountSupportsPath(acc, path):
		return false, "unsupported_path"
	default:
		return true, ""
	}
}

func accountSupportsPath(acc *account, path string) bool {
	if acc == nil {
		return false
	}
	if acc.Kind == accountKindOpenAIAPI {
		return strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/responses")
	}
	return true
}

func parseCodexClaims(idToken string) jwtClaims {
	var out jwtClaims
	parts := strings.Split(strings.TrimSpace(idToken), ".")
	if len(parts) < 2 {
		return out
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return out
	}
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return out
	}
	if exp, ok := obj["exp"].(float64); ok && exp > 0 {
		out.ExpiresAt = time.Unix(int64(exp), 0).UTC()
	}
	if sub, ok := obj["sub"].(string); ok {
		out.Subject = sub
	}
	if email, ok := obj["email"].(string); ok {
		out.Email = email
	}
	if auth, ok := obj["https://api.openai.com/auth"].(map[string]any); ok {
		if v, ok := auth["chatgpt_account_id"].(string); ok {
			out.ChatGPTAccountID = v
		}
		if v, ok := auth["chatgpt_user_id"].(string); ok {
			out.UserID = v
		}
		if v, ok := auth["chatgpt_plan_type"].(string); ok {
			out.PlanType = v
		}
	}
	if profile, ok := obj["https://api.openai.com/profile"].(map[string]any); ok && out.Email == "" {
		if v, ok := profile["email"].(string); ok {
			out.Email = v
		}
	}
	if out.ChatGPTAccountID == "" {
		if v, ok := obj["chatgpt_account_id"].(string); ok {
			out.ChatGPTAccountID = v
		}
	}
	return out
}

func saveAccount(acc *account) error {
	if acc == nil || acc.File == "" {
		return errors.New("missing account file")
	}
	if err := os.MkdirAll(filepath.Dir(acc.File), 0o700); err != nil {
		return err
	}

	root := map[string]any{}
	if existing, err := os.ReadFile(acc.File); err == nil && len(existing) > 0 {
		_ = json.Unmarshal(existing, &root)
	}
	acc.mu.Lock()
	defer acc.mu.Unlock()

	switch acc.Kind {
	case accountKindCodexOAuth:
		tokens, _ := root["tokens"].(map[string]any)
		if tokens == nil {
			tokens = map[string]any{}
		}
		tokens["access_token"] = acc.AccessToken
		tokens["refresh_token"] = acc.RefreshToken
		tokens["id_token"] = acc.IDToken
		if acc.AccountID != "" {
			tokens["account_id"] = acc.AccountID
		}
		root["tokens"] = tokens
	case accountKindOpenAIAPI:
		root["OPENAI_API_KEY"] = acc.AccessToken
		root["auth_mode"] = "api_key"
		root["plan_type"] = "api"
	}
	root["disabled"] = acc.Disabled
	root["dead"] = acc.Dead
	root["health_status"] = acc.HealthStatus
	root["health_error"] = acc.HealthError
	if !acc.LastRefresh.IsZero() {
		root["last_refresh"] = acc.LastRefresh.UTC().Format(time.RFC3339)
	}
	if !acc.RateLimitUntil.IsZero() {
		root["rate_limit_until"] = acc.RateLimitUntil.UTC().Format(time.RFC3339)
	}
	if acc.LastUsedPercent > 0 {
		root["last_used_percent"] = acc.LastUsedPercent
	}
	return atomicWriteJSON(acc.File, root)
}

func atomicWriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func accountIDFromOpaque(value, prefix string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return prefix + "_" + hex.EncodeToString(sum[:6])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
