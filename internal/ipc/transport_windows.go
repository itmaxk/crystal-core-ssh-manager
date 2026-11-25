//go:build windows

package ipc

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

const pipeName = `\\.\pipe\crystal-core-ssh-ipc`

// Доступ к pipe только у SYSTEM и владельца (текущего пользователя).
const pipeSDDL = "D:P(A;;GA;;;SY)(A;;GA;;;OW)"

func listenTransport() (net.Listener, error) {
	cfg := &winio.PipeConfig{
		SecurityDescriptor: pipeSDDL,
		MessageMode:        false,
		InputBufferSize:    4096,
		OutputBufferSize:   4096,
	}
	return winio.ListenPipe(pipeName, cfg)
}

func dialTransport() (net.Conn, error) {
	timeout := 2 * time.Second
	return winio.DialPipe(pipeName, &timeout)
}
