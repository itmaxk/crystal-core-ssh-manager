package main

import (
	"flag"
	"fmt"
	"os"

	"ssh-launcher/internal/cli"
)

func main() {
	connectHostID := flag.String("connect", "", "Host ID to connect via CLI wrapper")
	sessionID := flag.String("session", "", "Unique session ID for IPC tracking")
	runScript := flag.String("run", "", "Name of a predefined script to run on the host (vps-setup)")
	flag.Parse()

	if *connectHostID == "" {
		fmt.Fprintln(os.Stderr, "usage: ssh-launcher-cli --connect <host_id> --session <session_id>")
		os.Exit(1)
	}
	os.Exit(cli.Run(*connectHostID, *sessionID, *runScript))
}
