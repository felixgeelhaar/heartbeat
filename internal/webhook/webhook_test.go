package webhook_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	bolt "github.com/felixgeelhaar/bolt"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/events"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/webhook"
)

// captureServer returns an httptest.Server that records all POST bodies.
type captureServer struct {
	mu     sync.Mutex
	bodies []map[string]string
	status int
}

func newCaptureServer(statusCode int) (*httptest.Server, *captureServer) {
	cs := &captureServer{status: statusCode}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		json.Unmarshal(body, &payload)
		cs.mu.Lock()
		cs.bodies = append(cs.bodies, payload)
		cs.mu.Unlock()
		w.WriteHeader(cs.status)
	}))
	return srv, cs
}

func newNotifier(t *testing.T, url string) *webhook.Notifier {
	t.Helper()
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	return webhook.New(url, logger)
}

func waitForRequests(cs *captureServer, count int) bool {
	for i := 0; i < 100; i++ {
		cs.mu.Lock()
		n := len(cs.bodies)
		cs.mu.Unlock()
		if n >= count {
			return true
		}
		// Small busy-wait — acceptable in tests
		for j := 0; j < 1000; j++ {
		}
	}
	return false
}

func TestOnEvent_HealthCheckCreated(t *testing.T) {
	srv, cs := newCaptureServer(http.StatusOK)
	defer srv.Close()

	n := newNotifier(t, srv.URL)
	n.OnEvent(events.Event{
		Type:          events.HealthCheckCreated,
		HealthCheckID: "abcdefgh-1234-5678-9012-abcdef012345",
	})

	if !waitForRequests(cs, 1) {
		t.Fatal("expected 1 webhook call, timed out")
	}

	cs.mu.Lock()
	body := cs.bodies[0]
	cs.mu.Unlock()

	if !strings.Contains(body["text"], "abcdefgh") {
		t.Errorf("expected HC ID prefix in body, got: %q", body["text"])
	}
	if !strings.Contains(body["text"], "health check created") {
		t.Errorf("expected 'health check created' in body, got: %q", body["text"])
	}
}

func TestOnEvent_HealthCheckStatusChanged(t *testing.T) {
	srv, cs := newCaptureServer(http.StatusOK)
	defer srv.Close()

	n := newNotifier(t, srv.URL)
	n.OnEvent(events.Event{
		Type:          events.HealthCheckStatusChanged,
		HealthCheckID: "12345678-abcd-1234-abcd-123456789012",
	})

	if !waitForRequests(cs, 1) {
		t.Fatal("expected 1 webhook call, timed out")
	}

	cs.mu.Lock()
	body := cs.bodies[0]
	cs.mu.Unlock()

	if !strings.Contains(body["text"], "status changed") {
		t.Errorf("expected 'status changed' in body, got: %q", body["text"])
	}
}

func TestOnEvent_VoteSubmitted(t *testing.T) {
	srv, cs := newCaptureServer(http.StatusOK)
	defer srv.Close()

	n := newNotifier(t, srv.URL)
	n.OnEvent(events.Event{
		Type:          events.VoteSubmitted,
		HealthCheckID: "00000000-0000-0000-0000-000000000001",
		Participant:   "Alice",
		MetricName:    "Fun",
	})

	if !waitForRequests(cs, 1) {
		t.Fatal("expected 1 webhook call, timed out")
	}

	cs.mu.Lock()
	body := cs.bodies[0]
	cs.mu.Unlock()

	if !strings.Contains(body["text"], "Alice") {
		t.Errorf("expected participant name in body, got: %q", body["text"])
	}
	if !strings.Contains(body["text"], "Fun") {
		t.Errorf("expected metric name in body, got: %q", body["text"])
	}
}

func TestOnEvent_HealthCheckDeleted(t *testing.T) {
	srv, cs := newCaptureServer(http.StatusOK)
	defer srv.Close()

	n := newNotifier(t, srv.URL)
	n.OnEvent(events.Event{
		Type:          events.HealthCheckDeleted,
		HealthCheckID: "deleteme-1234-5678-abcd-000000000001",
	})

	if !waitForRequests(cs, 1) {
		t.Fatal("expected 1 webhook call, timed out")
	}

	cs.mu.Lock()
	body := cs.bodies[0]
	cs.mu.Unlock()

	if !strings.Contains(body["text"], "deleted") {
		t.Errorf("expected 'deleted' in body, got: %q", body["text"])
	}
}

func TestOnEvent_UnknownEvent_NoRequest(t *testing.T) {
	srv, cs := newCaptureServer(http.StatusOK)
	defer srv.Close()

	n := newNotifier(t, srv.URL)
	// Unknown event type must be silently ignored — no HTTP call.
	n.OnEvent(events.Event{
		Type:          events.EventType("unknown_event"),
		HealthCheckID: "00000000-0000-0000-0000-000000000001",
	})

	// Give goroutine time to fire (if it did)
	for i := 0; i < 500; i++ {
	}

	cs.mu.Lock()
	n2 := len(cs.bodies)
	cs.mu.Unlock()

	if n2 != 0 {
		t.Errorf("expected 0 webhook calls for unknown event, got %d", n2)
	}
}

func TestOnEvent_ServerError_LogsWarn(t *testing.T) {
	// Non-2xx response must not panic; just log a warning.
	srv, cs := newCaptureServer(http.StatusInternalServerError)
	defer srv.Close()

	n := newNotifier(t, srv.URL)
	n.OnEvent(events.Event{
		Type:          events.HealthCheckCreated,
		HealthCheckID: "00000000-0000-0000-0000-000000000001",
	})

	if !waitForRequests(cs, 1) {
		t.Fatal("expected 1 webhook call even on server error")
	}
}

func TestOnEvent_UnreachableURL_NooPanic(t *testing.T) {
	// Point to a definitely closed port — the client must not panic.
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	n := webhook.New("http://127.0.0.1:1", logger)

	// Must not panic.
	n.OnEvent(events.Event{
		Type:          events.HealthCheckCreated,
		HealthCheckID: "00000000-0000-0000-0000-000000000001",
	})
}

func TestOnEvent_PayloadIsJSONObject(t *testing.T) {
	var mu sync.Mutex
	var rawBodies [][]byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		rawBodies = append(rawBodies, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := newNotifier(t, srv.URL)
	n.OnEvent(events.Event{
		Type:          events.VoteSubmitted,
		HealthCheckID: "00000000-0000-0000-0000-000000000001",
		Participant:   "Bob",
		MetricName:    "Speed",
	})

	// Wait for the request
	for i := 0; i < 100; i++ {
		mu.Lock()
		n2 := len(rawBodies)
		mu.Unlock()
		if n2 >= 1 {
			break
		}
		for j := 0; j < 10000; j++ {
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if len(rawBodies) == 0 {
		t.Fatal("no webhook request captured")
	}

	var payload map[string]any
	if err := json.Unmarshal(rawBodies[0], &payload); err != nil {
		t.Fatalf("webhook body is not valid JSON: %v — body: %s", err, rawBodies[0])
	}
	if _, ok := payload["text"]; !ok {
		t.Error("expected 'text' field in webhook JSON payload")
	}
}

func TestNew_ReturnsNotifier(t *testing.T) {
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	n := webhook.New("https://example.com/hook", logger)
	if n == nil {
		t.Fatal("expected non-nil Notifier")
	}
}
