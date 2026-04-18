package gateway

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
)

type failingEventStore struct{}

func (f failingEventStore) Append(e spec.Event) error { return errors.New("append failed") }
func (f failingEventStore) Query(q store.Query) ([]spec.Event, error) {
	return []spec.Event{}, nil
}
func (f failingEventStore) GetByID(worldID, eventID string) (spec.Event, bool, error) {
	return spec.Event{}, false, nil
}

func TestIntents_Returns500WhenEventPersistFails(t *testing.T) {
	t.Parallel()

	es := failingEventStore{}
	hub := stream.NewHub()
	h := NewHandler(Options{EventStore: es, Hub: hub})
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)

	body, _ := json.Marshal(map[string]any{"world_id": "w1", "goal": "启动世界"})
	resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestAdoptionConfirm_Returns500AndDoesNotPublishWhenPersistFails(t *testing.T) {
	t.Parallel()

	es := failingEventStore{}
	hub := stream.NewHub()
	ch, unsub := hub.Subscribe(8)
	defer unsub()

	h := NewHandler(Options{EventStore: es, Hub: hub})
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	lobsterID := "lob_1"
	clientTs := int64(123)
	nonce := "n1"
	msg := []byte("adopt_confirm|" + lobsterID + "|" + pubB64 + "|123|n1")
	sig := ed25519.Sign(priv, msg)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	reqBody, _ := json.Marshal(map[string]any{
		"human_pubkey": pubB64,
		"lobster_id":   lobsterID,
		"sig":         sigB64,
		"client_ts":   clientTs,
		"nonce":       nonce,
	})
	resp, err := http.Post(s.URL+"/api/v0/adoptions/confirm", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	select {
	case e := <-ch:
		t.Fatalf("expected no publish on persist failure, got %#v", e)
	case <-time.After(50 * time.Millisecond):
		// ok
	}
}
