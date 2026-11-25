//go:build !windows

package agent

import (
	"errors"
	"net"
	"os"
	"time"
)

func dialAgent() (net.Conn, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK environment variable is not set")
	}
	return net.DialTimeout("unix", sock, 2*time.Second)
}
