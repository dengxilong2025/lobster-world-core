package adoption

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type Service struct {
	mu sync.RWMutex

	clock Clock

	// cooldown between re-binds after revoke
	cooldown time.Duration

	byLobster map[string]binding

	// Minimal anti-replay: nonce cache + client_ts time window.
	nonceTTL time.Duration
	maxSkew  time.Duration
	nonces   map[string]time.Time // key -> expiresAt
}

type binding struct {
	humanID       string
	humanPubKey   string
	boundAt       time.Time
	cooldownUntil time.Time
}

type Options struct {
	Clock    Clock
	Cooldown time.Duration
	// NonceTTL defines how long a (humanID,lobsterID,nonce) is considered "used".
	// Defaults to 10 minutes.
	NonceTTL time.Duration
	// MaxSkew defines allowed time skew between client_ts (unix seconds) and server clock.
	// Defaults to 5 minutes.
	MaxSkew time.Duration
}

func NewService(opts Options) *Service {
	c := opts.Clock
	if c == nil {
		c = realClock{}
	}
	cd := opts.Cooldown
	if cd <= 0 {
		cd = 24 * time.Hour
	}
	nttl := opts.NonceTTL
	if nttl <= 0 {
		nttl = 10 * time.Minute
	}
	skew := opts.MaxSkew
	if skew <= 0 {
		skew = 5 * time.Minute
	}

	return &Service{
		clock:    c,
		cooldown: cd,
		byLobster: map[string]binding{},
		nonceTTL: nttl,
		maxSkew:  skew,
		nonces:   map[string]time.Time{},
	}
}

func (s *Service) CooldownSeconds() int {
	return int(s.cooldown.Seconds())
}

func (s *Service) ConfirmByHumanSig(humanPubKeyB64, lobsterID, sigB64 string, clientTs int64, nonce string) (humanID string, err error) {
	humanID, err = deriveHumanID(humanPubKeyB64)
	if err != nil {
		return "", err
	}

	msg := confirmMessage(humanPubKeyB64, lobsterID, clientTs, nonce)
	if err := verifySig(humanPubKeyB64, msg, sigB64); err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkReplayLocked(humanID, lobsterID, clientTs, nonce); err != nil {
		return "", err
	}

	now := s.clock.Now()
	if b, ok := s.byLobster[lobsterID]; ok && now.Before(b.cooldownUntil) {
		return "", fmt.Errorf("cooldown active")
	}

	s.byLobster[lobsterID] = binding{
		humanID:       humanID,
		humanPubKey:   humanPubKeyB64,
		boundAt:       now,
		cooldownUntil: time.Time{},
	}
	return humanID, nil
}

func (s *Service) RevokeByHumanSig(humanPubKeyB64, lobsterID, sigB64 string, clientTs int64, nonce string) (humanID string, cooldownSec int, err error) {
	humanID, err = deriveHumanID(humanPubKeyB64)
	if err != nil {
		return "", 0, err
	}

	msg := revokeMessage(humanPubKeyB64, lobsterID, clientTs, nonce)
	if err := verifySig(humanPubKeyB64, msg, sigB64); err != nil {
		return "", 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkReplayLocked(humanID, lobsterID, clientTs, nonce); err != nil {
		return "", 0, err
	}

	b, ok := s.byLobster[lobsterID]
	if !ok || b.humanID != humanID {
		return "", 0, fmt.Errorf("not bound")
	}

	now := s.clock.Now()
	b.cooldownUntil = now.Add(s.cooldown)
	s.byLobster[lobsterID] = b
	return humanID, int(s.cooldown.Seconds()), nil
}

// RevokeByLobster allows the lobster side to revoke without needing human signatures.
func (s *Service) RevokeByLobster(lobsterID string) (humanID string, cooldownSec int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.byLobster[lobsterID]
	if !ok {
		return "", 0, fmt.Errorf("not bound")
	}

	now := s.clock.Now()
	b.cooldownUntil = now.Add(s.cooldown)
	s.byLobster[lobsterID] = b
	return b.humanID, int(s.cooldown.Seconds()), nil
}

func (s *Service) GetBinding(lobsterID string) (humanID string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.byLobster[lobsterID]
	if !ok {
		return "", false
	}
	return b.humanID, true
}

func confirmMessage(humanPubKeyB64, lobsterID string, clientTs int64, nonce string) []byte {
	return []byte(fmt.Sprintf("adopt_confirm|%s|%s|%d|%s", lobsterID, humanPubKeyB64, clientTs, nonce))
}

func revokeMessage(humanPubKeyB64, lobsterID string, clientTs int64, nonce string) []byte {
	return []byte(fmt.Sprintf("adopt_revoke|%s|%s|%d|%s", lobsterID, humanPubKeyB64, clientTs, nonce))
}

func verifySig(pubKeyB64 string, msg []byte, sigB64 string) error {
	pubBytes, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid pubkey")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature encoding")
	}
	if !ed25519.Verify(ed25519.PublicKey(pubBytes), msg, sigBytes) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func deriveHumanID(humanPubKeyB64 string) (string, error) {
	pubBytes, err := base64.StdEncoding.DecodeString(humanPubKeyB64)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid pubkey")
	}
	sum := sha256.Sum256(pubBytes)
	return fmt.Sprintf("human_%x", sum[:6]), nil
}

func (s *Service) checkReplayLocked(humanID, lobsterID string, clientTs int64, nonce string) error {
	if clientTs <= 0 {
		return fmt.Errorf("invalid client_ts")
	}
	now := s.clock.Now()
	clientTime := time.Unix(clientTs, 0)
	delta := now.Sub(clientTime)
	if delta < 0 {
		delta = -delta
	}
	if delta > s.maxSkew {
		return fmt.Errorf("client_ts outside allowed window")
	}

	// opportunistic cleanup
	for k, exp := range s.nonces {
		if now.After(exp) {
			delete(s.nonces, k)
		}
	}

	key := fmt.Sprintf("%s|%s|%s", humanID, lobsterID, nonce)
	if exp, ok := s.nonces[key]; ok && now.Before(exp) {
		return fmt.Errorf("nonce already used")
	}
	s.nonces[key] = now.Add(s.nonceTTL)
	return nil
}
