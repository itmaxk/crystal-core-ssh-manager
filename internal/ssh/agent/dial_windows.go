//go:build windows

package agent

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

const windowsAgentPipe = `\\.\pipe\openssh-ssh-agent`

func dialAgent() (net.Conn, error) {
	timeout := 2 * time.Second
	return winio.DialPipe(windowsAgentPipe, &timeout)
}
