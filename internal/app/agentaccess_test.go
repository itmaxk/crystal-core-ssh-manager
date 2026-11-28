package app

import (
	"strings"
	"testing"
)

const testAgentPub = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKeyMaterial cc-agent@crystal-core"

func TestBuildAuthorizedKeyLineShell(t *testing.T) {
	line, err := buildAuthorizedKeyLine(testAgentPub, "cc-agent", "shell", "203.0.113.10", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`from="203.0.113.10"`,
		"no-port-forwarding",
		"no-agent-forwarding",
		"no-x11-forwarding",
		"no-user-rc",
		"crystal-core-agent:cc-agent",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("line does not contain %q: %s", want, line)
		}
	}
	if strings.Contains(line, "restrict") {
		t.Errorf("shell mode must not use restrict: %s", line)
	}
}

func TestBuildAuthorizedKeyLineTunnel(t *testing.T) {
	line, err := buildAuthorizedKeyLine(testAgentPub, "cc-agent", "tunnel", "", "127.0.0.1:5432")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"restrict", "port-forwarding", `permitopen="127.0.0.1:5432"`} {
		if !strings.Contains(line, want) {
			t.Errorf("line does not contain %q: %s", want, line)
		}
	}
	if strings.Contains(line, "no-port-forwarding") {
		t.Errorf("tunnel mode must allow port forwarding: %s", line)
	}
}

func TestValidateAgentSource(t *testing.T) {
	for _, ok := range []string{"", "203.0.113.10", "10.0.0.0/8", "203.0.113.10,10.0.0.0/8"} {
		if err := validateAgentSource(ok); err != nil {
			t.Errorf("validateAgentSource(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"example.com", "10.0.0.0/99", "1.2.3.4,", "127.0.0.1/"} {
		if err := validateAgentSource(bad); err == nil {
			t.Errorf("validateAgentSource(%q) = nil, want error", bad)
		}
	}
}

func TestValidatePermitOpen(t *testing.T) {
	for _, ok := range []string{"127.0.0.1:5432", "db.internal:3306", "[::1]:6379"} {
		if err := validatePermitOpen(ok); err != nil {
			t.Errorf("validatePermitOpen(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:70000", "host name:22"} {
		if err := validatePermitOpen(bad); err == nil {
			t.Errorf("validatePermitOpen(%q) = nil, want error", bad)
		}
	}
}

func TestAuthorizedKeysScriptQuotesMarker(t *testing.T) {
	script := authorizedKeysScript("upsert", agentGrantMarker("cc-agent"))
	if !strings.Contains(script, `grep -vF -- "$1"`) {
		t.Errorf("script must remove existing marker line: %s", script)
	}
	if !strings.Contains(script, `printf '%s\n' "$2"`) {
		t.Errorf("script must append the new line: %s", script)
	}
}
