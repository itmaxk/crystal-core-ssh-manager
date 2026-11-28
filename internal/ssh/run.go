package ssh

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// RunRequest — выполнение одной команды на сервере с проверкой host key.
type RunRequest struct {
	Hostname        string
	Port            int
	Username        string
	PrivateKeyPEM   []byte
	Passphrase      []byte
	Command         string
	Timeout         time.Duration
	HostKeyCallback ssh.HostKeyCallback
}

// RunResult — разделённые stdout/stderr удалённой команды.
type RunResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// RunCommand подключается по SSH и выполняет команду. Используется для
// управления authorized_keys при выдаче доступа ИИ-агенту.
func RunCommand(ctx context.Context, req RunRequest) (*RunResult, error) {
	if err := ValidateHostname(req.Hostname); err != nil {
		return nil, err
	}
	if err := ValidatePort(req.Port); err != nil {
		return nil, err
	}
	if err := ValidateUsername(req.Username); err != nil {
		return nil, err
	}
	if req.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	signer, err := parseSigner(req.PrivateKeyPEM, req.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	hostKeyCallback := req.HostKeyCallback
	if hostKeyCallback == nil {
		hostKeyCallback = StrictHostKeyCallback()
	}

	addr := net.JoinHostPort(req.Hostname, fmt.Sprintf("%d", req.Port))
	client, err := dialSSHContext(ctx, addr, &ssh.ClientConfig{
		User:            req.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}, timeout)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr
	if err := sess.Run(req.Command); err != nil {
		return &RunResult{Stdout: stdout.String(), Stderr: stderr.String()},
			fmt.Errorf("remote command failed: %w (%s)", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return &RunResult{Stdout: stdout.String(), Stderr: stderr.String()}, nil
}
