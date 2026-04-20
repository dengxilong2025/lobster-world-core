package adoption

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

func TestAdoption_MinimalAntiReplay_NonceAndTimeWindow(t *testing.T) {
	t.Parallel()

	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))

	clk := &testClock{now: time.Unix(1_000_000, 0)}
	svc := NewService(Options{
		Clock:    clk,
		Cooldown: 0,
		NonceTTL: 10 * time.Minute,
		MaxSkew:  5 * time.Minute,
	})

	lobsterID := "lobster_aaaa"
	nonce := "n1"
	clientTs := clk.now.Unix()

	// confirm success
	msg := confirmMessage(pubB64, lobsterID, clientTs, nonce)
	sig := ed25519.Sign(priv, msg)
	sigB64 := base64.StdEncoding.EncodeToString(sig)
	if _, err := svc.ConfirmByHumanSig(pubB64, lobsterID, sigB64, clientTs, nonce); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	// replay with same nonce should fail
	msg2 := confirmMessage(pubB64, lobsterID, clientTs, nonce)
	sig2 := ed25519.Sign(priv, msg2)
	sig2B64 := base64.StdEncoding.EncodeToString(sig2)
	if _, err := svc.ConfirmByHumanSig(pubB64, lobsterID, sig2B64, clientTs, nonce); err == nil {
		t.Fatalf("expected replay to be rejected")
	}

	// outside time window should fail (even with fresh nonce)
	oldTs := clk.now.Add(-10 * time.Minute).Unix()
	msg3 := confirmMessage(pubB64, lobsterID, oldTs, "n2")
	sig3 := ed25519.Sign(priv, msg3)
	sig3B64 := base64.StdEncoding.EncodeToString(sig3)
	if _, err := svc.ConfirmByHumanSig(pubB64, lobsterID, sig3B64, oldTs, "n2"); err == nil {
		t.Fatalf("expected old client_ts to be rejected")
	}

	// after nonce TTL expires, reuse is allowed.
	clk.now = clk.now.Add(11 * time.Minute)
	msg4 := confirmMessage(pubB64, lobsterID, clk.now.Unix(), nonce)
	sig4 := ed25519.Sign(priv, msg4)
	sig4B64 := base64.StdEncoding.EncodeToString(sig4)
	if _, err := svc.ConfirmByHumanSig(pubB64, lobsterID, sig4B64, clk.now.Unix(), nonce); err != nil {
		t.Fatalf("expected nonce reuse after ttl ok, got %v", err)
	}
}

