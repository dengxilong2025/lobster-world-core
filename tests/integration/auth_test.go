package integration

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lobster-world-core/internal/gateway"
)

func TestAuth_ChallengeProveAndWhoami(t *testing.T) {
	t.Parallel()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	app := gateway.NewApp()
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	// 1) challenge
	chReq := map[string]any{
		"lobster_pubkey": pubB64,
		"client_ts":      1710000000,
	}
	chResp := postJSON(t, s.URL+"/api/v0/auth/challenge", chReq)

	challenge, ok := chResp["challenge"].(string)
	if !ok || challenge == "" {
		t.Fatalf("expected challenge string, got %#v", chResp)
	}

	// 2) prove
	sig := ed25519.Sign(priv, []byte(challenge))
	prReq := map[string]any{
		"lobster_pubkey": pubB64,
		"challenge":      challenge,
		"sig":            base64.StdEncoding.EncodeToString(sig),
		"client_ts":      1710000001,
	}
	prResp := postJSON(t, s.URL+"/api/v0/auth/prove", prReq)

	token, ok := prResp["session_token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected session_token, got %#v", prResp)
	}

	// 3) whoami
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v0/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /me: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var me map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		t.Fatalf("decode /me: %v", err)
	}
	if me["lobster_id"] == "" {
		t.Fatalf("expected lobster_id, got %#v", me)
	}
}

func postJSON(t *testing.T, url string, body any) map[string]any {
	t.Helper()

	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var raw bytes.Buffer
		_, _ = raw.ReadFrom(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, raw.String())
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}
