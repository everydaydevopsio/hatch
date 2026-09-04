package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	mobyclient "github.com/moby/moby/client"
)

const defaultRemoteServiceURL = "https://hatch.orchael.com"

type launchResult struct {
	SessionID string
	Hostname  string
	StartURL  string
	BrowserURL string
}

type remoteSessionRequest struct {
	SessionID string `json:"session_id"`
	Server    string `json:"server"`
	Target    string `json:"target"`
	LaunchURL string `json:"launch_url"`
	ExpiresAt string `json:"expires_at"`
}

// init handles open-remote before the legacy command dispatcher. Keeping this
// isolated lets the remote feature evolve without changing the existing open
// command until the CLI is split into command packages.
func init() {
	if len(os.Args) < 2 || os.Args[1] != "open-remote" {
		return
	}
	if err := runOpenRemote(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "hatch: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runOpenRemote(args []string) error {
	opts, err := parseOpenArgs(args)
	if err != nil {
		return fmt.Errorf("usage: hatch open-remote [--port port] <url>: %w", err)
	}
	if err := requireDocker(); err != nil {
		return err
	}
	before := time.Now().Add(-2 * time.Second).Unix()
	if err := launch(opts); err != nil {
		return err
	}
	session, err := newestRemoteSession(opts.URL, before)
	if err != nil {
		return err
	}
	if err := notifyRemoteSession(session); err != nil {
		_ = stopSession(session.SessionID)
		return err
	}
	fmt.Printf("Remote:       notified paired devices\n")
	return nil
}

func newestRemoteSession(startURL string, createdAfter int64) (launchResult, error) {
	cfg, err := loadConfig()
	if err != nil {
		return launchResult{}, err
	}
	ctx := context.Background()
	cli, err := dockerClient()
	if err != nil {
		return launchResult{}, err
	}
	defer cli.Close()

	result, err := cli.ContainerList(ctx, mobyclient.ContainerListOptions{All: true})
	if err != nil {
		return launchResult{}, fmt.Errorf("find remote session: %w", err)
	}
	containers := hatchContainers(result.Items)
	sort.Slice(containers, func(i, j int) bool { return containers[i].Created > containers[j].Created })
	for _, ctr := range containers {
		if ctr.Created < createdAfter || ctr.Labels["io.everydaydevops.hatch.start-url"] != redactedStartURL(startURL) {
			continue
		}
		accessPath, err := waitForAccessPath(ctx, cli, ctr.ID, 2*time.Second)
		if err != nil {
			continue
		}
		port := ctr.Labels["io.everydaydevops.hatch.port"]
		return launchResult{
			SessionID: ctr.Labels["io.everydaydevops.hatch.session"],
			Hostname: cfg.Hostname,
			StartURL: startURL,
			BrowserURL: fmt.Sprintf("https://%s:%s%s", cfg.Hostname, port, accessPath),
		}, nil
	}
	return launchResult{}, fmt.Errorf("remote session was created but could not be discovered")
}

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
		Server: session.Hostname,
		Target: remoteTarget(session.StartURL),
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
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
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
