package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	mobyclient "github.com/moby/moby/client"
	"gopkg.in/yaml.v3"
)

const (
	defaultImage     = "hatch:local"
	defaultPortMin   = 18000
	defaultPortMax   = 18999
	defaultTTL       = 3600
	defaultReadyWait = 90 * time.Second
)

var accessPathRE = regexp.MustCompile(`Hatch access path:\s+(/hatch/\?token=[^\s]+)`)

type config struct {
	Hostname string `yaml:"hostname"`
	Port     int    `yaml:"port,omitempty"`
}

type openOptions struct {
	URL  string
	Port int
}

type dockerState int

const (
	dockerReady dockerState = iota
	dockerMissing
	dockerStopped
)

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
		if len(args) > 3 {
			return errors.New("usage: hatch init [hostname[:port]|hostname port]")
		}
		state, err := checkDocker()
		if err != nil {
			return err
		}
		if state != dockerReady {
			printDockerGuidance(state)
			return nil
		}
		if len(args) < 2 {
			return errors.New("Docker is ready; finish setup with: hatch init <hostname[:port]>")
		}
		return initConfig(args[1:])
	case "list":
		if err := requireDocker(); err != nil {
			return err
		}
		return listSessions()
	case "open":
		opts, err := parseOpenArgs(args[1:])
		if err != nil {
			return err
		}
		if err := requireDocker(); err != nil {
			return err
		}
		return launch(opts)
	case "stop":
		if len(args) != 2 {
			return errors.New("usage: hatch stop <session>|--all")
		}
		if err := requireDocker(); err != nil {
			return err
		}
		if args[1] == "--all" {
			return stopAllSessions()
		}
		return stopSession(args[1])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q; run 'hatch help'", args[0])
	}
}

func printUsage() {
	fmt.Println(`Hatch launches an ephemeral browser desktop for an OAuth URL.

Usage:
  hatch init [hostname[:port]|hostname port]
  hatch open [--port port] <url>
  hatch list
  hatch stop <session>|--all

Examples:
  hatch init
  hatch init devbox.tailnet.ts.net 8443
  hatch init devbox.tailnet.ts.net:8443
  hatch open --port 8443 'https://example.com/oauth/authorize?...'
  hatch open 'https://example.com/oauth/authorize?...'
  hatch list
  hatch stop --all
  hatch stop 8ac4d911`)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "hatch", "hatch.yaml"), nil
}

func initConfig(args []string) error {
	endpoint, err := normalizeEndpoint(args)
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

	data, err := yaml.Marshal(endpoint)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	fmt.Printf("Docker is installed and running.\n")
	fmt.Printf("Wrote %s\n", path)
	fmt.Printf("Hostname: %s\n", endpoint.Hostname)
	if endpoint.Port != 0 {
		fmt.Printf("Port: %d\n", endpoint.Port)
	} else {
		fmt.Printf("Port: dynamic\n")
	}
	return nil
}

func normalizeHostname(input string) (string, error) {
	endpoint, err := normalizeEndpoint([]string{input})
	if err != nil {
		return "", err
	}
	return endpoint.Hostname, nil
}

func normalizeEndpoint(args []string) (config, error) {
	if len(args) == 0 || len(args) > 2 {
		return config{}, errors.New("usage: hatch init [hostname[:port]|hostname port]")
	}

	rawHost := args[0]
	rawPort := ""
	if len(args) == 2 {
		rawPort = args[1]
	}

	hostname, port, err := parseEndpoint(rawHost, rawPort)
	if err != nil {
		return config{}, err
	}
	return config{Hostname: hostname, Port: port}, nil
}

func parseEndpoint(input, rawPort string) (string, int, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", 0, errors.New("hostname cannot be empty")
	}
	rawPort = strings.TrimSpace(rawPort)
	port := 0

	if strings.Contains(input, "://") {
		u, err := url.Parse(input)
		if err != nil || u.Hostname() == "" {
			return "", 0, fmt.Errorf("invalid hostname %q", input)
		}
		if u.Port() != "" {
			if rawPort != "" {
				return "", 0, errors.New("provide port either in the hostname or as a separate argument, not both")
			}
			rawPort = u.Port()
		}
		input = u.Hostname()
	}

	if strings.ContainsAny(input, "/?#") {
		return "", 0, fmt.Errorf("hostname must not include a path, query, or fragment: %q", input)
	}
	if host, splitPort, err := net.SplitHostPort(input); err == nil {
		if rawPort != "" {
			return "", 0, errors.New("provide port either in the hostname or as a separate argument, not both")
		}
		input = host
		rawPort = splitPort
	} else if strings.Count(input, ":") == 1 {
		if rawPort != "" {
			return "", 0, errors.New("provide port either in the hostname or as a separate argument, not both")
		}
		host, splitPort, _ := strings.Cut(input, ":")
		input = host
		rawPort = splitPort
	} else if strings.Contains(input, ":") {
		return "", 0, fmt.Errorf("invalid hostname or host:port %q", input)
	}
	if input == "" {
		return "", 0, errors.New("hostname cannot be empty")
	}
	if rawPort != "" {
		parsed, err := strconv.Atoi(rawPort)
		if err != nil || parsed < 1 || parsed > 65535 {
			return "", 0, fmt.Errorf("invalid port %q", rawPort)
		}
		port = parsed
	}
	return input, port, nil
}

func parseOpenArgs(args []string) (openOptions, error) {
	if len(args) == 0 {
		return openOptions{}, errors.New("usage: hatch open [--port port] <url>")
	}

	var opts openOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--port":
			if opts.Port != 0 {
				return openOptions{}, errors.New("port specified more than once")
			}
			if i+1 >= len(args) {
				return openOptions{}, errors.New("usage: hatch open [--port port] <url>")
			}
			port, err := parsePort(args[i+1])
			if err != nil {
				return openOptions{}, err
			}
			opts.Port = port
			i++
		case strings.HasPrefix(arg, "--port="):
			if opts.Port != 0 {
				return openOptions{}, errors.New("port specified more than once")
			}
			port, err := parsePort(strings.TrimPrefix(arg, "--port="))
			if err != nil {
				return openOptions{}, err
			}
			opts.Port = port
		case strings.HasPrefix(arg, "-"):
			return openOptions{}, fmt.Errorf("unknown option %q", arg)
		default:
			if opts.URL != "" {
				return openOptions{}, errors.New("usage: hatch open [--port port] <url>")
			}
			opts.URL = arg
		}
	}
	if opts.URL == "" {
		return openOptions{}, errors.New("usage: hatch open [--port port] <url>")
	}
	return opts, nil
}

func parsePort(raw string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed < 1 || parsed > 65535 {
		return 0, fmt.Errorf("invalid port %q", raw)
	}
	return parsed, nil
}

func loadConfig() (config, error) {
	path, err := configPath()
	if err != nil {
		return config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config{}, fmt.Errorf("configuration not found; run hatch init <hostname[:port]> first")
		}
		return config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Hostname == "" {
		return config{}, errors.New("configuration has no hostname; rerun hatch init <hostname[:port]>")
	}
	if cfg.Port != 0 && (cfg.Port < 1 || cfg.Port > 65535) {
		return config{}, fmt.Errorf("configuration has invalid port %d; rerun hatch init <hostname[:port]>", cfg.Port)
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

func checkDocker() (dockerState, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return dockerMissing, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cli, err := dockerClient()
	if err != nil {
		return dockerStopped, err
	}
	defer cli.Close()
	if _, err := cli.Ping(ctx, mobyclient.PingOptions{NegotiateAPIVersion: true}); err != nil {
		return dockerStopped, err
	}
	return dockerReady, nil
}

func requireDocker() error {
	state, err := checkDocker()
	if err != nil {
		return err
	}
	switch state {
	case dockerMissing:
		return errors.New("Docker is not installed; run 'hatch init' for installation instructions")
	case dockerStopped:
		printDockerGuidance(dockerStopped)
		if err != nil {
			return fmt.Errorf("Docker is installed but not reachable: %w", err)
		}
		return errors.New("Docker is installed but not running")
	default:
		return nil
	}
}

func printDockerGuidance(state dockerState) {
	osName := runtime.GOOS
	if state == dockerMissing {
		fmt.Printf("Docker is not installed.\n\n")
		fmt.Print(dockerInstallInstructions(osName, linuxDistribution()))
		fmt.Printf("\nAfter Docker is installed and running, run:\n  hatch init <hostname[:port]>\n")
		return
	}

	fmt.Printf("Docker is installed but the Docker daemon is not running.\n\n")
	fmt.Print(dockerStartInstructions(osName))
	fmt.Printf("\nThen retry your Hatch command.\n")
}

func dockerInstallInstructions(goos, distro string) string {
	switch goos {
	case "darwin":
		return `macOS installation:
  1. Install Docker Desktop from https://docs.docker.com/desktop/setup/install/mac-install/
  2. Open /Applications/Docker.app and wait for Docker to finish starting.

If you use Homebrew, you can install Docker Desktop with:
  brew install --cask docker
  open -a Docker
`
	case "windows":
		return `Windows installation:
  1. Install Docker Desktop from https://docs.docker.com/desktop/setup/install/windows-install/
  2. Docker Desktop normally uses the WSL 2 backend.
  3. Open Docker Desktop from the Start menu and wait for Docker to finish starting.

If winget is available, you can install it with:
  winget install -e --id Docker.DockerDesktop

Then start Docker Desktop from the Start menu.
`
	case "linux":
		switch distro {
		case "ubuntu":
			return `Linux (Ubuntu) installation:
  Follow Docker's official repository setup at:
  https://docs.docker.com/engine/install/ubuntu/

After configuring Docker's apt repository, install Docker Engine with:
  sudo apt update
  sudo apt install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  sudo systemctl enable --now docker
`
		case "debian":
			return `Linux (Debian) installation:
  Follow Docker's official repository setup at:
  https://docs.docker.com/engine/install/debian/

After configuring Docker's apt repository, install Docker Engine with:
  sudo apt update
  sudo apt install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  sudo systemctl enable --now docker
`
		case "fedora":
			return `Linux (Fedora) installation:
  Follow Docker's official instructions at:
  https://docs.docker.com/engine/install/fedora/

After configuring Docker's repository, install and start Docker Engine with:
  sudo dnf install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  sudo systemctl enable --now docker
`
		case "centos", "rhel":
			return `Linux (CentOS/RHEL) installation:
  Follow Docker's official Linux installation instructions at:
  https://docs.docker.com/engine/install/

After installing Docker Engine, start it with:
  sudo systemctl enable --now docker
`
		default:
			return `Linux installation:
  Install Docker Engine using the instructions for your distribution:
  https://docs.docker.com/engine/install/

On systemd-based distributions, start and enable Docker with:
  sudo systemctl enable --now docker
`
		}
	default:
		return `Install Docker for your operating system:
  https://docs.docker.com/get-started/get-docker/
`
	}
}

func dockerStartInstructions(goos string) string {
	switch goos {
	case "darwin":
		return `Start Docker Desktop on macOS:
  open -a Docker

Or open Docker from Applications. Wait until Docker Desktop reports that the engine is running.
`
	case "windows":
		return `Start Docker Desktop on Windows:
  Open the Start menu, search for "Docker Desktop", and launch it.

From PowerShell, if Docker Desktop is installed in the standard location, you can also run:
  Start-Process "$Env:LOCALAPPDATA\Programs\DockerDesktop\Docker Desktop.exe"

For an all-users installation, use:
  Start-Process "C:\Program Files\Docker\Docker\Docker Desktop.exe"
`
	case "linux":
		return `Start Docker Engine on Linux:
  sudo systemctl start docker

To also start Docker automatically at boot:
  sudo systemctl enable docker

Check its status with:
  sudo systemctl status docker
`
	default:
		return `Start the Docker daemon for your operating system, then verify it with:
  docker info
`
	}
}

func linuxDistribution() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), `"`)
	}

	id := strings.ToLower(values["ID"])
	idLike := strings.ToLower(values["ID_LIKE"])
	for _, candidate := range []string{"ubuntu", "debian", "fedora", "centos", "rhel"} {
		if id == candidate || strings.Contains(idLike, candidate) {
			return candidate
		}
	}
	return id
}

func launch(opts openOptions) error {
	startURL, err := validateStartURL(opts.URL)
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	port, err := selectLaunchPort(cfg, opts.Port)
	if err != nil {
		return err
	}

	ctx := context.Background()
	cli, err := dockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()

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

	created, err := cli.ContainerCreate(ctx, launchContainerOptions(name, startURL, port, labels))
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

	if err := waitForHealthyContainer(ctx, cli, created.ID, defaultReadyWait); err != nil {
		return err
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

func selectLaunchPort(cfg config, requestedPort int) (int, error) {
	if requestedPort != 0 {
		return requireFreePort(requestedPort)
	}
	if cfg.Port != 0 {
		return requireFreePort(cfg.Port)
	}
	return findFreePort(defaultPortMin, defaultPortMax)
}

func requireFreePort(port int) (int, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return 0, fmt.Errorf("port %d is already in use", port)
	}
	_ = ln.Close()
	return port, nil
}

func launchContainerOptions(name, startURL string, port int, labels map[string]string) mobyclient.ContainerCreateOptions {
	return mobyclient.ContainerCreateOptions{
		Name: name,
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
	}
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

func waitForHealthyContainer(ctx context.Context, cli *mobyclient.Client, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		inspect, err := cli.ContainerInspect(ctx, containerID, mobyclient.ContainerInspectOptions{})
		if err != nil {
			return fmt.Errorf("inspect container health: %w", err)
		}
		status, err := inspectHealthStatus(inspect.Container)
		if err != nil {
			return err
		}
		if status == containertypes.Healthy {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("timed out waiting for Hatch container to become healthy")
}

func inspectHealthStatus(container containertypes.InspectResponse) (containertypes.HealthStatus, error) {
	if container.State == nil {
		return "", errors.New("container has no state")
	}
	if container.State.Dead || (!container.State.Running && !container.State.Restarting) {
		if container.State.Error != "" {
			return "", fmt.Errorf("container is %s: %s", container.State.Status, container.State.Error)
		}
		return "", fmt.Errorf("container is %s", container.State.Status)
	}
	if container.State.Health == nil {
		return "", errors.New("container has no healthcheck")
	}
	switch container.State.Health.Status {
	case containertypes.Healthy:
		return containertypes.Healthy, nil
	case containertypes.Unhealthy:
		return containertypes.Unhealthy, errors.New("container became unhealthy")
	case containertypes.Starting:
		return containertypes.Starting, nil
	default:
		return container.State.Health.Status, fmt.Errorf("unexpected container health status %q", container.State.Health.Status)
	}
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
	containers := hatchContainers(result.Items)
	var rows []row
	for _, ctr := range containers {
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
	for _, ctr := range hatchContainers(result.Items) {
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

func stopAllSessions() error {
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

	containers := hatchContainers(result.Items)
	if len(containers) == 0 {
		fmt.Println("No Hatch sessions.")
		return nil
	}
	sort.Slice(containers, func(i, j int) bool {
		return containers[i].Labels["io.everydaydevops.hatch.session"] < containers[j].Labels["io.everydaydevops.hatch.session"]
	})
	for _, ctr := range containers {
		if _, err := cli.ContainerRemove(ctx, ctr.ID, mobyclient.ContainerRemoveOptions{Force: true}); err != nil {
			return fmt.Errorf("remove session %s: %w", ctr.Labels["io.everydaydevops.hatch.session"], err)
		}
		fmt.Printf("Stopped %s\n", ctr.Labels["io.everydaydevops.hatch.session"])
	}
	return nil
}

func hatchContainers(containers []containertypes.Summary) []containertypes.Summary {
	var managed []containertypes.Summary
	for _, ctr := range containers {
		if ctr.Labels["io.everydaydevops.hatch.managed"] == "true" {
			managed = append(managed, ctr)
		}
	}
	return managed
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
