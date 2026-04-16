package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type Service struct {
	clock Clock

	challengeTTL time.Duration
	sessionTTL   time.Duration

	challenges map[string]challengeRecord
	sessions   map[string]sessionRecord
}

type challengeRecord struct {
	pubkey    string
	expiresAt time.Time
	used      bool
}

type sessionRecord struct {
	lobsterID string
	pubkey    string
	expiresAt time.Time
}

type Options struct {
	Clock        Clock
	ChallengeTTL time.Duration
	SessionTTL   time.Duration
}

func NewService(opts Options) *Service {
	c := opts.Clock
	if c == nil {
		c = realClock{}
	}

	ttl := opts.ChallengeTTL
	if ttl <= 0 {
		ttl = 60 * time.Second
	}

	sttl := opts.SessionTTL
	if sttl <= 0 {
		sttl = 24 * time.Hour
	}

	return &Service{
		clock:        c,
		challengeTTL: ttl,
		sessionTTL:   sttl,
		challenges:   map[string]challengeRecord{},
		sessions:     map[string]sessionRecord{},
	}
}

func (s *Service) CreateChallenge(pubkeyBase64 string) (challenge string, ttlSec int, err error) {
	if _, err := decodeEd25519PubKey(pubkeyBase64); err != nil {
		return "", 0, err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", 0, fmt.Errorf("rand: %w", err)
	}
	challenge = base64.StdEncoding.EncodeToString(raw)

	s.challenges[challenge] = challengeRecord{
		pubkey:    pubkeyBase64,
		expiresAt: s.clock.Now().Add(s.challengeTTL),
		used:      false,
	}

	return challenge, int(s.challengeTTL.Seconds()), nil
}

func (s *Service) Prove(pubkeyBase64, challenge, sigBase64 string) (sessionToken string, expiresAt int64, lobsterID string, err error) {
	pub, err := decodeEd25519PubKey(pubkeyBase64)
	if err != nil {
		return "", 0, "", err
	}

	rec, ok := s.challenges[challenge]
	if !ok {
		return "", 0, "", fmt.Errorf("unknown challenge")
	}
	if rec.used {
		return "", 0, "", fmt.Errorf("challenge already used")
	}
	if s.clock.Now().After(rec.expiresAt) {
		return "", 0, "", fmt.Errorf("challenge expired")
	}
	if rec.pubkey != pubkeyBase64 {
		return "", 0, "", fmt.Errorf("challenge pubkey mismatch")
	}

	sig, err := base64.StdEncoding.DecodeString(sigBase64)
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid sig encoding")
	}
	if len(sig) != ed25519.SignatureSize {
		return "", 0, "", fmt.Errorf("invalid signature size")
	}
	if !ed25519.Verify(pub, []byte(challenge), sig) {
		return "", 0, "", fmt.Errorf("invalid signature")
	}

	s.challenges[challenge] = challengeRecord{
		pubkey:    rec.pubkey,
		expiresAt: rec.expiresAt,
		used:      true,
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", 0, "", fmt.Errorf("rand: %w", err)
	}
	sessionToken = base64.StdEncoding.EncodeToString(raw)

	lobsterID = deriveLobsterID(pub)
	exp := s.clock.Now().Add(s.sessionTTL)
	s.sessions[sessionToken] = sessionRecord{
		lobsterID: lobsterID,
		pubkey:    pubkeyBase64,
		expiresAt: exp,
	}

	return sessionToken, exp.Unix(), lobsterID, nil
}

func (s *Service) GetSession(token string) (lobsterID string, pubkey string, ok bool) {
	rec, ok := s.sessions[token]
	if !ok {
		return "", "", false
	}
	if s.clock.Now().After(rec.expiresAt) {
		delete(s.sessions, token)
		return "", "", false
	}
	return rec.lobsterID, rec.pubkey, true
}

func decodeEd25519PubKey(pubkeyBase64 string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(pubkeyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid pubkey encoding")
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid pubkey size")
	}
	return ed25519.PublicKey(b), nil
}

func deriveLobsterID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	// Compact, stable, human-visible.
	return fmt.Sprintf("lobster_%x", sum[:6]) // 12 hex chars
}

