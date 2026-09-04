package main

import "testing"

func TestRemoteTargetOnlyReturnsHostname(t *testing.T) {
	got := remoteTarget("https://accounts.google.com/o/oauth2/v2/auth?state=secret&code_challenge=also-secret")
	if got != "accounts.google.com" {
		t.Fatalf("remoteTarget() = %q, want accounts.google.com", got)
	}
}

func TestRemoteTargetInvalidURL(t *testing.T) {
	if got := remoteTarget("not a url"); got != "remote authentication" {
		t.Fatalf("remoteTarget() = %q", got)
	}
}
