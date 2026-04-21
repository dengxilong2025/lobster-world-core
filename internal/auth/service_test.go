package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

func TestAuth_ChallengeProve_SuccessAndReplayProtection(t *testing.T) {
	t.Parallel()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	clk := &testClock{now: time.Unix(1_000_000, 0)}
	svc := NewService(Options{
		Clock:        clk,
		ChallengeTTL: 60 * time.Second,
		SessionTTL:   24 * time.Hour,
	})

	ch, _, err := svc.CreateChallenge(pubB64)
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}

	sig := ed25519.Sign(priv, []byte(ch))
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	token, exp, lobsterID, err := svc.Prove(pubB64, ch, sigB64)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	if token == "" || exp == 0 || lobsterID == "" {
		t.Fatalf("expected non-empty token/exp/lobsterID, got token=%q exp=%d lobsterID=%q", token, exp, lobsterID)
	}

	// challenge replay should fail
	_, _, _, err = svc.Prove(pubB64, ch, sigB64)
	if err == nil {
		t.Fatalf("expected replayed challenge to be rejected")
	}
}

func TestAuth_Challenge_Expires(t *testing.T) {
	t.Parallel()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	clk := &testClock{now: time.Unix(1_000_000, 0)}
	svc := NewService(Options{
		Clock:        clk,
		ChallengeTTL: 2 * time.Second,
		SessionTTL:   24 * time.Hour,
	})

	ch, _, err := svc.CreateChallenge(pubB64)
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	clk.now = clk.now.Add(3 * time.Second)

	sig := ed25519.Sign(priv, []byte(ch))
	sigB64 := base64.StdEncoding.EncodeToString(sig)
	_, _, _, err = svc.Prove(pubB64, ch, sigB64)
	if err == nil {
		t.Fatalf("expected expired challenge to fail")
	}
}

func TestAuth_Session_ExpiresAndIsRejected(t *testing.T) {
	t.Parallel()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	clk := &testClock{now: time.Unix(1_000_000, 0)}
	svc := NewService(Options{
		Clock:        clk,
		ChallengeTTL: 60 * time.Second,
		SessionTTL:   2 * time.Second,
	})

	ch, _, err := svc.CreateChallenge(pubB64)
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	sig := ed25519.Sign(priv, []byte(ch))
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	token, _, _, err := svc.Prove(pubB64, ch, sigB64)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	if _, _, ok := svc.GetSession(token); !ok {
		t.Fatalf("expected session ok before expiry")
	}

	clk.now = clk.now.Add(3 * time.Second)
	if _, _, ok := svc.GetSession(token); ok {
		t.Fatalf("expected session to be expired")
	}
}

