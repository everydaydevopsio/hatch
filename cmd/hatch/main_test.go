package main

import (
	"net"
	"strings"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
)

func TestNormalizeHostname(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "hostname", input: "devbox.tailnet.ts.net", want: "devbox.tailnet.ts.net"},
		{name: "url", input: "https://devbox.tailnet.ts.net:8443/path", want: "devbox.tailnet.ts.net"},
		{name: "host port", input: "devbox.tailnet.ts.net:8443", want: "devbox.tailnet.ts.net"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeHostname(tt.input)
			if err != nil {
				t.Fatalf("normalizeHostname() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeHostname() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		hostname string
		port     int
	}{
		{name: "no configured port", args: []string{"devbox.tailnet.ts.net"}, hostname: "devbox.tailnet.ts.net"},
		{name: "host colon port", args: []string{"devbox.tailnet.ts.net:8443"}, hostname: "devbox.tailnet.ts.net", port: 8443},
		{name: "separate port", args: []string{"devbox.tailnet.ts.net", "8443"}, hostname: "devbox.tailnet.ts.net", port: 8443},
		{name: "url port", args: []string{"https://devbox.tailnet.ts.net:9443/path"}, hostname: "devbox.tailnet.ts.net", port: 9443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeEndpoint(tt.args)
			if err != nil {
				t.Fatalf("normalizeEndpoint() error = %v", err)
			}
			if got.Hostname != tt.hostname || got.Port != tt.port {
				t.Fatalf("normalizeEndpoint() = %+v, want hostname=%q port=%d", got, tt.hostname, tt.port)
			}
		})
	}
}

func TestNormalizeEndpointRejectsInvalidPort(t *testing.T) {
	for _, args := range [][]string{
		{"devbox.tailnet.ts.net", "0"},
		{"devbox.tailnet.ts.net", "65536"},
		{"devbox.tailnet.ts.net", "not-a-port"},
		{"devbox.tailnet.ts.net:not-a-port"},
		{"devbox.tailnet.ts.net:8443", "9443"},
	} {
		if _, err := normalizeEndpoint(args); err == nil {
			t.Fatalf("normalizeEndpoint(%v) accepted invalid port", args)
		}
	}
}

func TestValidateStartURL(t *testing.T) {
	if _, err := validateStartURL("https://example.com/oauth?client_id=abc"); err != nil {
		t.Fatalf("valid URL rejected: %v", err)
	}

	for _, raw := range []string{"example.com", "file:///tmp/test", "javascript:alert(1)"} {
		if _, err := validateStartURL(raw); err == nil {
			t.Fatalf("invalid URL %q accepted", raw)
		}
	}
}

func TestRedactedStartURLRemovesSensitiveParts(t *testing.T) {
	got := redactedStartURL("https://example.com/oauth/authorize?client_id=abc&state=secret#token")
	want := "https://example.com/oauth/authorize"
	if got != want {
		t.Fatalf("redactedStartURL() = %q, want %q", got, want)
	}
}

func TestRunRequiresOpenCommandForURL(t *testing.T) {
	if err := run([]string{"https://example.com/oauth?client_id=abc"}); err == nil {
		t.Fatalf("implicit URL command accepted")
	}
}

func TestRunOpenUsage(t *testing.T) {
	for _, args := range [][]string{
		{"open"},
		{"open", "https://example.com/oauth?client_id=abc", "extra"},
		{"open", "--port"},
	} {
		if err := run(args); err == nil || !strings.Contains(err.Error(), "usage: hatch open [--port port] <url>") {
			t.Fatalf("run(%v) error = %v, want open usage", args, err)
		}
	}
}

func TestRunStopUsage(t *testing.T) {
	for _, args := range [][]string{
		{"stop"},
		{"stop", "abc", "extra"},
	} {
		if err := run(args); err == nil || !strings.Contains(err.Error(), "usage: hatch stop <session>|--all") {
			t.Fatalf("run(%v) error = %v, want stop usage", args, err)
		}
	}
}

func TestParseOpenArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		url  string
		port int
	}{
		{name: "dynamic port", args: []string{"https://example.com/oauth"}, url: "https://example.com/oauth"},
		{name: "separate port", args: []string{"--port", "8443", "https://example.com/oauth"}, url: "https://example.com/oauth", port: 8443},
		{name: "equals port", args: []string{"--port=9443", "https://example.com/oauth"}, url: "https://example.com/oauth", port: 9443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOpenArgs(tt.args)
			if err != nil {
				t.Fatalf("parseOpenArgs() error = %v", err)
			}
			if got.URL != tt.url || got.Port != tt.port {
				t.Fatalf("parseOpenArgs() = %+v, want URL=%q port=%d", got, tt.url, tt.port)
			}
		})
	}
}

func TestParseOpenArgsRejectsInvalidPort(t *testing.T) {
	for _, args := range [][]string{
		{"--port", "0", "https://example.com/oauth"},
		{"--port=not-a-port", "https://example.com/oauth"},
		{"--port", "8443", "--port", "9443", "https://example.com/oauth"},
		{"--bogus", "https://example.com/oauth"},
	} {
		if _, err := parseOpenArgs(args); err == nil {
			t.Fatalf("parseOpenArgs(%v) accepted invalid args", args)
		}
	}
}

func TestLaunchContainerOptionsUseLocalImage(t *testing.T) {
	labels := map[string]string{"io.everydaydevops.hatch.managed": "true"}

	opts := launchContainerOptions("hatch-test", "https://example.com/oauth", 8443, labels)

	if opts.Image != "" {
		t.Fatalf("ContainerCreateOptions.Image = %q, want empty because Config.Image is set", opts.Image)
	}
	if opts.Config == nil || opts.Config.Image != defaultImage {
		t.Fatalf("Config.Image = %q, want %q", opts.Config.Image, defaultImage)
	}
	if opts.HostConfig == nil || opts.HostConfig.NetworkMode != "host" {
		t.Fatalf("NetworkMode = %q, want host", opts.HostConfig.NetworkMode)
	}
}

func TestSelectLaunchPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on dynamic port: %v", err)
	}
	defer ln.Close()
	occupiedPort := ln.Addr().(*net.TCPAddr).Port

	freePort, err := findFreePort(defaultPortMin, defaultPortMax)
	if err != nil {
		t.Fatalf("findFreePort() error = %v", err)
	}

	got, err := selectLaunchPort(config{Hostname: "devbox.tailnet.ts.net"}, 0)
	if err != nil {
		t.Fatalf("selectLaunchPort(dynamic) error = %v", err)
	}
	if got < defaultPortMin || got > defaultPortMax {
		t.Fatalf("selectLaunchPort(dynamic) = %d, want range %d-%d", got, defaultPortMin, defaultPortMax)
	}

	got, err = selectLaunchPort(config{Hostname: "devbox.tailnet.ts.net", Port: freePort}, 0)
	if err != nil {
		t.Fatalf("selectLaunchPort(config port) error = %v", err)
	}
	if got != freePort {
		t.Fatalf("selectLaunchPort(config port) = %d, want %d", got, freePort)
	}

	if _, err := selectLaunchPort(config{Hostname: "devbox.tailnet.ts.net"}, occupiedPort); err == nil {
		t.Fatalf("selectLaunchPort() accepted occupied requested port")
	}
}

func TestInspectHealthStatus(t *testing.T) {
	tests := []struct {
		name    string
		state   *containertypes.State
		want    containertypes.HealthStatus
		wantErr string
	}{
		{
			name: "healthy",
			state: &containertypes.State{
				Running: true,
				Health:  &containertypes.Health{Status: containertypes.Healthy},
			},
			want: containertypes.Healthy,
		},
		{
			name: "starting",
			state: &containertypes.State{
				Running: true,
				Health:  &containertypes.Health{Status: containertypes.Starting},
			},
			want: containertypes.Starting,
		},
		{
			name: "unhealthy",
			state: &containertypes.State{
				Running: true,
				Health:  &containertypes.Health{Status: containertypes.Unhealthy},
			},
			wantErr: "container became unhealthy",
		},
		{
			name:    "exited",
			state:   &containertypes.State{Status: "exited"},
			wantErr: "container is exited",
		},
		{
			name:    "missing healthcheck",
			state:   &containertypes.State{Running: true},
			wantErr: "container has no healthcheck",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := inspectHealthStatus(containertypes.InspectResponse{State: tt.state})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("inspectHealthStatus() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("inspectHealthStatus() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("inspectHealthStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHatchContainersOnlyReturnsManagedContainers(t *testing.T) {
	containers := []containertypes.Summary{
		{
			ID:     "managed",
			Labels: map[string]string{"io.everydaydevops.hatch.managed": "true"},
		},
		{
			ID:     "unmanaged",
			Labels: map[string]string{"io.everydaydevops.hatch.managed": "false"},
		},
		{
			ID: "unlabeled",
		},
	}

	got := hatchContainers(containers)
	if len(got) != 1 {
		t.Fatalf("hatchContainers() returned %d containers, want 1", len(got))
	}
	if got[0].ID != "managed" {
		t.Fatalf("hatchContainers()[0].ID = %q, want managed", got[0].ID)
	}
}

func TestDockerInstallInstructions(t *testing.T) {
	tests := []struct {
		goos   string
		distro string
		want   string
	}{
		{goos: "darwin", want: "Docker Desktop"},
		{goos: "windows", want: "winget"},
		{goos: "linux", distro: "ubuntu", want: "apt install docker-ce"},
		{goos: "linux", distro: "debian", want: "engine/install/debian"},
		{goos: "linux", distro: "fedora", want: "dnf install docker-ce"},
		{goos: "linux", distro: "arch", want: "engine/install"},
	}

	for _, tt := range tests {
		got := dockerInstallInstructions(tt.goos, tt.distro)
		if !strings.Contains(got, tt.want) {
			t.Fatalf("dockerInstallInstructions(%q, %q) did not contain %q: %s", tt.goos, tt.distro, tt.want, got)
		}
	}
}

func TestDockerStartInstructions(t *testing.T) {
	for goos, want := range map[string]string{
		"darwin":  "open -a Docker",
		"windows": "Docker Desktop",
		"linux":   "systemctl start docker",
	} {
		if got := dockerStartInstructions(goos); !strings.Contains(got, want) {
			t.Fatalf("dockerStartInstructions(%q) did not contain %q: %s", goos, want, got)
		}
	}
}
