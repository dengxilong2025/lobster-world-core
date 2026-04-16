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

	return &Service{
		clock:    c,
		cooldown: cd,
		byLobster: map[string]binding{},
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

