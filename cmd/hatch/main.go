package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	mobyclient "github.com/moby/moby/client"
	"gopkg.in/yaml.v3"
)

const (
	defaultImage   = "hatch:local"
	defaultPortMin = 18000
	defaultPortMax = 18999
	defaultTTL     = 3600
)

var accessPathRE = regexp.MustCompile(`Hatch access path:\s+(/hatch/\?token=[^\s]+)`)

type config struct {
	Hostname string `yaml:"hostname"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "hatch: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "init":
		if len(args) != 2 {
			return errors.New("usage: hatch init <hostname>")
		}
		return initConfig(args[1])
	case "list":
		return listSessions()
	case "stop":
		if len(args) != 2 {
			return errors.New("usage: hatch stop <session>")
		}
		return stopSession(args[1])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		if len(args) != 1 {
			return errors.New("usage: hatch <url>")
		}
		return launch(args[0])
	}
}

func printUsage() {
	fmt.Println(`Hatch launches an ephemeral browser desktop for an OAuth URL.

Usage:
  hatch init <hostname>
  hatch <url>
  hatch list
  hatch stop <session>

Examples:
  hatch init devbox.tailnet.ts.net
  hatch 'https://example.com/oauth/authorize?...'
  hatch list
  hatch stop 8ac4d911`)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "hatch", "hatch.yaml"), nil
}

func initConfig(rawHostname string) error {
	hostname, err := normalizeHostname(rawHostname)
	if err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(config{Hostname: hostname})
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Printf("Wrote %s\n", path)
	fmt.Printf("Hostname: %s\n", hostname)
	return nil
}

func normalizeHostname(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("hostname cannot be empty")
	}

	if strings.Contains(input, "://") {
		u, err := url.Parse(input)
		if err != nil || u.Hostname() == "" {
			return "", fmt.Errorf("invalid hostname %q", input)
		}
		input = u.Hostname()
	}

	if strings.ContainsAny(input, "/?#") {
		return "", fmt.Errorf("hostname must not include a path, query, or fragment: %q", input)
	}
	if host, _, err := net.SplitHostPort(input); err == nil {
		input = host
	}
	if input == "" {
		return "", errors.New("hostname cannot be empty")
	}
	return input, nil
}

func loadConfig() (config, error) {
	path, err := configPath()
	if err != nil {
		return config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config{}, fmt.Errorf("configuration not found; run hatch init <hostname> first")
		}
		return config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Hostname == "" {
		return config{}, errors.New("configuration has no hostname; rerun hatch init <hostname>")
	}
	return cfg, nil
}

func dockerClient() (*mobyclient.Client, error) {
	cli, err := mobyclient.New(mobyclient.FromEnv, mobyclient.WithUserAgent("hatch-cli"))
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}
	return cli, nil
}

func launch(rawURL string) error {
	startURL, err := validateStartURL(rawURL)
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx := context.Background()
	cli, err := dockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	port, err := findFreePort(defaultPortMin, defaultPortMax)
	if err != nil {
		return err
	}
	sessionID, err := newSessionID()
	if err != nil {
		return err
	}
	name := "hatch-" + sessionID

	labels := map[string]string{
		"io.everydaydevops.hatch.managed":   "true",
		"io.everydaydevops.hatch.session":   sessionID,
		"io.everydaydevops.hatch.start-url": startURL,
		"io.everydaydevops.hatch.port":      strconv.Itoa(port),
	}

	created, err := cli.ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Name:  name,
		Image: defaultImage,
		Config: &containertypes.Config{
			Image: defaultImage,
			Env: []string{
				"HATCH_START_URL=" + startURL,
				"HATCH_HTTPS_PORT=" + strconv.Itoa(port),
				"HATCH_GUAC_LAUNCH_TTL_SECONDS=" + strconv.Itoa(defaultTTL),
			},
			Labels: labels,
		},
		HostConfig: &containertypes.HostConfig{
			NetworkMode: "host",
			ShmSize:     1 << 30,
			SecurityOpt: []string{"no-new-privileges:true"},
		},
	})
	if err != nil {
		return fmt.Errorf("create container from %s: %w", defaultImage, err)
	}

	cleanup := true
	defer func() {
		if cleanup {
			_, _ = cli.ContainerRemove(context.Background(), created.ID, mobyclient.ContainerRemoveOptions{Force: true})
		}
	}()

	if _, err := cli.ContainerStart(ctx, created.ID, mobyclient.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	accessPath, err := waitForAccessPath(ctx, cli, created.ID, 20*time.Second)
	if err != nil {
		return err
	}
	cleanup = false

	fmt.Printf("Session:      %s\n", sessionID)
	fmt.Printf("Container:    %s\n", name)
	fmt.Printf("Start URL:    %s\n", startURL)
	fmt.Printf("Browser URL:  https://%s:%d%s\n", cfg.Hostname, port, accessPath)
	fmt.Printf("Stop with:    hatch stop %s\n", sessionID)
	return nil
}

func validateStartURL(raw string) (string, error) {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("URL scheme must be http or https")
	}
	if u.Host == "" {
		return "", errors.New("URL must include a host")
	}
	return raw, nil
}

func findFreePort(min, max int) (int, error) {
	for port := min; port <= max; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no free TCP port found in range %d-%d", min, max)
}

func newSessionID() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session ID: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func waitForAccessPath(ctx context.Context, cli *mobyclient.Client, containerID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		logs, err := cli.ContainerLogs(ctx, containerID, mobyclient.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "200",
		})
		if err == nil {
			data, readErr := io.ReadAll(logs)
			_ = logs.Close()
			if readErr == nil {
				if match := accessPathRE.FindSubmatch(data); len(match) == 2 {
					return string(match[1]), nil
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for Hatch access URL; inspect container logs")
}

func listSessions() error {
	ctx := context.Background()
	cli, err := dockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	result, err := cli.ContainerList(ctx, mobyclient.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	type row struct {
		session string
		status  string
		port    string
		url     string
	}
	var rows []row
	for _, ctr := range result.Items {
		if ctr.Labels["io.everydaydevops.hatch.managed"] != "true" {
			continue
		}
		rows = append(rows, row{
			session: ctr.Labels["io.everydaydevops.hatch.session"],
			status:  ctr.Status,
			port:    ctr.Labels["io.everydaydevops.hatch.port"],
			url:     ctr.Labels["io.everydaydevops.hatch.start-url"],
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].session < rows[j].session })

	if len(rows) == 0 {
		fmt.Println("No Hatch sessions.")
		return nil
	}
	fmt.Printf("%-10s %-8s %-24s %s\n", "SESSION", "PORT", "STATUS", "START URL")
	for _, r := range rows {
		fmt.Printf("%-10s %-8s %-24s %s\n", r.session, r.port, truncate(r.status, 24), r.url)
	}
	return nil
}

func stopSession(session string) error {
	ctx := context.Background()
	cli, err := dockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	result, err := cli.ContainerList(ctx, mobyclient.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	var matches []containertypes.Summary
	for _, ctr := range result.Items {
		if ctr.Labels["io.everydaydevops.hatch.managed"] != "true" {
			continue
		}
		id := ctr.Labels["io.everydaydevops.hatch.session"]
		if id == session || strings.HasPrefix(id, session) {
			matches = append(matches, ctr)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("session %q not found", session)
	}
	if len(matches) > 1 {
		return fmt.Errorf("session prefix %q matches multiple sessions", session)
	}

	if _, err := cli.ContainerRemove(ctx, matches[0].ID, mobyclient.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("remove session: %w", err)
	}
	fmt.Printf("Stopped %s\n", matches[0].Labels["io.everydaydevops.hatch.session"])
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 2 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
