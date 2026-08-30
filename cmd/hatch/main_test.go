package main

import (
	"strings"
	"testing"
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
