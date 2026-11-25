//go:build !windows

package ipc

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func socketPath() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "crystal-core-ssh.sock")
	}
	// Без XDG_RUNTIME_DIR /tmp общий для всех пользователей — добавляем uid,
	// чтобы инстансы разных пользователей не мешали друг другу.
	return filepath.Join(os.TempDir(), fmt.Sprintf("crystal-core-ssh-%d.sock", os.Getuid()))
}

func listenTransport() (net.Listener, error) {
	path := socketPath()
	_ = os.Remove(path)

	// umask на время bind, чтобы сокет сразу создавался с правами 0600 (без гонки с chmod).
	old := syscall.Umask(0177)
	ln, err := net.Listen("unix", path)
	syscall.Umask(old)
	if err != nil {
		return nil, err
	}
	return ln, nil
}

func dialTransport() (net.Conn, error) {
	return net.DialTimeout("unix", socketPath(), 2*time.Second)
}
