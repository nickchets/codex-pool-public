package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "none"})
	payload, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestLoadPoolAcceptsCodexOAuthAndOpenAIAPIOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "openai_api"), 0o700); err != nil {
		t.Fatal(err)
	}
	idToken := testIDToken(t, map[string]any{
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"email": "dev@example.com",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct_test",
			"chatgpt_plan_type":  "pro",
		},
	})
	if err := os.WriteFile(filepath.Join(dir, "codex", "work.json"), []byte(`{"tokens":{"access_token":"oa-token","refresh_token":"refresh","id_token":"`+idToken+`"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openai_api", "ci.json"), []byte(`{"OPENAI_API_KEY":"sk-test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.json"), []byte(`{"otherProviderOauth":{"accessToken":"nope"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	pool, err := loadPool(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := pool.count(); got != 2 {
		t.Fatalf("count=%d, want 2", got)
	}
	if got := pool.countKind(accountKindCodexOAuth); got != 1 {
		t.Fatalf("oauth count=%d, want 1", got)
	}
	if got := pool.countKind(accountKindOpenAIAPI); got != 1 {
		t.Fatalf("api count=%d, want 1", got)
	}
}

func TestOpenAIAPIKeyDoesNotSupportBackendPaths(t *testing.T) {
	acc := &account{Kind: accountKindOpenAIAPI, AccessToken: "sk-test"}
	if accountSupportsPath(acc, "/backend-api/codex/models") {
		t.Fatal("api key account should not support ChatGPT backend paths")
	}
	if !accountSupportsPath(acc, "/v1/responses") {
		t.Fatal("api key account should support /v1/responses")
	}
}

func TestGeneratedConfigIsCodexOnly(t *testing.T) {
	s := &server{cfg: config{ListenAddr: "127.0.0.1:8989"}, pool: &poolState{}, startTime: time.Now()}
	req := httptest.NewRequest(http.MethodGet, "http://pool.local/config/codex.toml", nil)
	rr := httptest.NewRecorder()
	s.serveCodexConfig(rr, req)
	body := rr.Body.String()
	for _, want := range []string{`model_provider = "codex-pool"`, `wire_api = "responses"`, `requires_openai_auth = true`} {
		if !strings.Contains(body, want) {
			t.Fatalf("config missing %q in:\n%s", want, body)
		}
	}
	for _, forbidden := range []string{"third-party-provider-a", "third-party-provider-b"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("config contains forbidden provider %q", forbidden)
		}
	}
}
