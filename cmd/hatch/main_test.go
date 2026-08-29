package main

import "testing"

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
