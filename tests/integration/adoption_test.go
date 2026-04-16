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
	"time"

	"lobster-world-core/internal/gateway"
)

func TestAdoption_ConfirmRevokeCooldown(t *testing.T) {
	t.Parallel()

	// Human key for signing adoption actions.
	hPub, hPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen human key: %v", err)
	}
	hPubB64 := base64.StdEncoding.EncodeToString(hPub)

	app := gateway.NewApp()
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	lobsterID := "lobster_test_1"

	// confirm
	confirmNonce := "n1"
	confirmTs := time.Now().Unix()
	confirmMsg := []byte("adopt_confirm|" + lobsterID + "|" + hPubB64 + "|" + itoa(confirmTs) + "|" + confirmNonce)
	confirmSig := base64.StdEncoding.EncodeToString(ed25519.Sign(hPriv, confirmMsg))

	confirmResp := postJSON(t, s.URL+"/api/v0/adoptions/confirm", map[string]any{
		"human_pubkey": hPubB64,
		"lobster_id":   lobsterID,
		"sig":          confirmSig,
		"client_ts":    confirmTs,
		"nonce":        confirmNonce,
	})
	if confirmResp["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", confirmResp)
	}

	// revoke
	revokeNonce := "n2"
	revokeTs := time.Now().Unix()
	revokeMsg := []byte("adopt_revoke|" + lobsterID + "|" + hPubB64 + "|" + itoa(revokeTs) + "|" + revokeNonce)
	revokeSig := base64.StdEncoding.EncodeToString(ed25519.Sign(hPriv, revokeMsg))

	revokeResp := postJSON(t, s.URL+"/api/v0/adoptions/revoke", map[string]any{
		"human_pubkey": hPubB64,
		"lobster_id":   lobsterID,
		"sig":          revokeSig,
		"client_ts":    revokeTs,
		"nonce":        revokeNonce,
	})
	if revokeResp["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", revokeResp)
	}
	if revokeResp["cooldown_sec"] == nil {
		t.Fatalf("expected cooldown_sec, got %#v", revokeResp)
	}

	// confirm again during cooldown should fail with 400.
	confirmNonce2 := "n3"
	confirmTs2 := time.Now().Unix()
	confirmMsg2 := []byte("adopt_confirm|" + lobsterID + "|" + hPubB64 + "|" + itoa(confirmTs2) + "|" + confirmNonce2)
	confirmSig2 := base64.StdEncoding.EncodeToString(ed25519.Sign(hPriv, confirmMsg2))
	b, _ := json.Marshal(map[string]any{
		"human_pubkey": hPubB64,
		"lobster_id":   lobsterID,
		"sig":          confirmSig2,
		"client_ts":    confirmTs2,
		"nonce":        confirmNonce2,
	})
	resp, err := http.Post(s.URL+"/api/v0/adoptions/confirm", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST confirm again: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func itoa(n int64) string {
	// tiny local helper to avoid strconv import bloat in this test file.
	// n is always small in this context.
	buf := make([]byte, 0, 20)
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		d := n % 10
		buf = append(buf, byte('0'+d))
		n /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
