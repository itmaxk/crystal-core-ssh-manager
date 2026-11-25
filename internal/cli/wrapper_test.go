package cli

import (
	"reflect"
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
