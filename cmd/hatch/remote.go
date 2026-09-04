package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultRemoteServiceURL = "https://hatch.orchael.com"

type remoteSessionRequest struct {
	SessionID string `json:"session_id"`
	Server    string `json:"server"`
	Target    string `json:"target"`
	LaunchURL string `json:"launch_url"`
	ExpiresAt string `json:"expires_at"`
}

// notifyRemoteSession registers a newly-created Hatch session with the remote
// rendezvous service. The service is responsible for notifying devices paired
// with this Hatch server. HATCH_REMOTE_TOKEN identifies the server; it is
// created by the pairing/registration flow and deliberately kept out of CLI
// arguments so it does not leak through process listings.
func notifyRemoteSession(session launchResult) error {
	token := strings.TrimSpace(os.Getenv("HATCH_REMOTE_TOKEN"))
	if token == "" {
		return fmt.Errorf("remote access is not configured; pair a Hatch app first or set HATCH_REMOTE_TOKEN")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("HATCH_REMOTE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultRemoteServiceURL
	}

	payload := remoteSessionRequest{
		SessionID: session.SessionID,
		Server:    session.Hostname,
		Target:    remoteTarget(session.StartURL),
		LaunchURL: session.BrowserURL,
		ExpiresAt: time.Now().Add(time.Duration(defaultTTL) * time.Second).UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode remote session: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/sessions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create remote request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("notify remote service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("remote service returned %s", resp.Status)
	}
	return nil
}

func remoteTarget(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "remote authentication"
	}
	return u.Hostname()
}
