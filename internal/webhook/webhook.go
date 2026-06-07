// Package webhook sends Slack-compatible notifications on health check events.
package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	bolt "go.klarlabs.de/bolt"

	"github.com/felixgeelhaar/heartbeat/internal/events"
)

// Config holds webhook configuration.
type Config struct {
	URL string `yaml:"url"`
}

// Notifier sends webhook notifications for health check events.
type Notifier struct {
	url    string
	logger *bolt.Logger
	client *http.Client
}

// New creates a new webhook notifier.
func New(url string, logger *bolt.Logger) *Notifier {
	return &Notifier{
		url:    url,
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// OnEvent implements events.Listener.
func (n *Notifier) OnEvent(e events.Event) {
	var text string
	switch e.Type {
	case events.HealthCheckCreated:
		text = fmt.Sprintf(":clipboard: New health check created (ID: %s)", e.HealthCheckID[:8])
	case events.HealthCheckStatusChanged:
		text = fmt.Sprintf(":white_check_mark: Health check status changed (ID: %s)", e.HealthCheckID[:8])
	case events.VoteSubmitted:
		text = fmt.Sprintf(":ballot_box: %s voted on *%s*", e.Participant, e.MetricName)
	case events.HealthCheckDeleted:
		text = fmt.Sprintf(":wastebasket: Health check deleted (ID: %s)", e.HealthCheckID[:8])
	default:
		return
	}

	payload := map[string]string{"text": text}
	body, _ := json.Marshal(payload)

	resp, err := n.client.Post(n.url, "application/json", bytes.NewReader(body))
	if err != nil {
		n.logger.Error().Err(err).Str("url", n.url).Msg("webhook failed")
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		n.logger.Warn().Int("status", resp.StatusCode).Str("url", n.url).Msg("webhook non-OK response")
	}
}
