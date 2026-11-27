package cli

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSSHArgs(t *testing.T) {
	args, err := BuildSSHArgs(ConnectConfig{Hostname: "10.0.0.1", Port: 2222, Username: "root", PubKeyPath: "/k.pub", JumpHost: "u@j:22"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-p", "2222", "-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3",
		"-o", "IdentitiesOnly=yes", "-i", "/k.pub", "-J", "u@j:22", "--", "root@10.0.0.1"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("got %v\nwant %v", args, want)
	}
}

func TestBuildSSHArgsRejectsOptionInjection(t *testing.T) {
	bad := []ConnectConfig{
		{Hostname: "-oProxyCommand=calc", Port: 22, Username: "root"},
		{Hostname: "host", Port: 22, Username: "-oProxyCommand=calc"},
		{Hostname: "host", Port: 0, Username: "root"},
	}
	for _, c := range bad {
		if _, err := BuildSSHArgs(c); err == nil {
			t.Errorf("BuildSSHArgs(%+v) accepted invalid input", c)
		}
	}
}

func TestBuildSSHArgsWithScript(t *testing.T) {
	cmd, err := remoteCommand(ScriptVPSSetup)
	if err != nil {
		t.Fatal(err)
	}
	args, err := BuildSSHArgs(ConnectConfig{Hostname: "10.0.0.1", Port: 22, Username: "root", RemoteCommand: cmd})
	if err != nil {
		t.Fatal(err)
	}
	n := len(args)
	if args[n-4] != "-t" || args[n-3] != "--" || args[n-2] != "root@10.0.0.1" || args[n-1] != cmd {
		t.Fatalf("args tail: %q", args[n-4:])
	}
	if _, err := remoteCommand("rm -rf /"); err == nil {
		t.Fatal("unknown script accepted")
	}
}

func TestVPSSetupScriptQuoting(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	cmd, _ := remoteCommand(ScriptVPSSetup)
	// Удалённая оболочка получает строку cmd: проверяем, что она разбирается
	// ровно в «sh -c <скрипт>», а сам скрипт синтаксически корректен.
	out, err := exec.Command("sh", "-c", "printf '%s\\n' "+cmd).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "sh\n-c\n" + scripts[ScriptVPSSetup] + "\n"
	if string(out) != want {
		t.Fatalf("quoted script round-trip mismatch:\n%s", out)
	}
	if out, err := exec.Command("sh", "-n", "-c", scripts[ScriptVPSSetup]).CombinedOutput(); err != nil {
		t.Fatalf("script syntax: %v\n%s", err, out)
	}
	if !strings.Contains(scripts[ScriptVPSSetup], VPSSetupURL) || !strings.Contains(scripts[ScriptVPSSetup], "sudo ./vps-setup") {
		t.Fatal("script lost its download/run steps")
	}
}
