package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"ssh-launcher/internal/crypto"
)

type BootstrapStep string

// HostKeyConfirmTimeout — сколько ждать решения пользователя по отпечатку хоста.
const HostKeyConfirmTimeout = 2 * time.Minute

const (
	StepConnectPassword BootstrapStep = "connect_password"
	StepDeployKey       BootstrapStep = "deploy_key"
	StepVerifyKey       BootstrapStep = "verify_key"
	StepDone            BootstrapStep = "done"
)

type HostKeyInfo struct {
	Hostname    string `json:"hostname"`
	RemoteAddr  string `json:"remote_addr"`
	KeyType     string `json:"key_type"`
	Fingerprint string `json:"fingerprint"`
}

type BootstrapRequest struct {
	Hostname       string
	Port           int
	Username       string
	Password       []byte
	PublicKey      string
	PrivateKeyPEM  []byte
	Passphrase     []byte
	Timeout        time.Duration
	ConfirmHostKey func(ctx context.Context, info HostKeyInfo) (bool, error)
	OnProgress     func(step BootstrapStep, message string)
}

type BootstrapResult struct {
	HostKeyFingerprint string `json:"host_key_fingerprint"`
	KeyAlreadyPresent  bool   `json:"key_already_present"`
}

func RunBootstrap(ctx context.Context, req BootstrapRequest) (*BootstrapResult, error) {
	defer crypto.ZeroBytes(req.Password)
	defer crypto.ZeroBytes(req.PrivateKeyPEM)
	defer crypto.ZeroBytes(req.Passphrase)

	if err := validateBootstrapReq(req); err != nil {
		return nil, err
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	emit := func(step BootstrapStep, msg string) {
		if req.OnProgress != nil {
			req.OnProgress(step, msg)
		}
	}

	signer, err := parseSigner(req.PrivateKeyPEM, req.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	if !bytes.Equal(pubKey.Marshal(), signer.PublicKey().Marshal()) {
		return nil, errors.New("public key does not match private key")
	}

	addr := net.JoinHostPort(req.Hostname, strconv.Itoa(req.Port))
	var pinnedHostKey ssh.PublicKey
	var pinnedFingerprint string
	var newHostKey bool

	emit(StepConnectPassword, "Подключение к серверу и проверка отпечатка хоста...")

	passCfg := &ssh.ClientConfig{
		User: req.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(string(req.Password)),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = string(req.Password)
				}
				return answers, nil
			}),
		},
		Timeout: timeout,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fp := ssh.FingerprintSHA256(key)
			known, err := checkKnownHost(hostname, remote, key)
			if err != nil {
				return err
			}
			if !known {
				if req.ConfirmHostKey == nil {
					return fmt.Errorf("%w (%s %s)", ErrHostKeyUnknown, key.Type(), fp)
				}
				ok, cbErr := req.ConfirmHostKey(ctx, HostKeyInfo{
					Hostname:    hostname,
					RemoteAddr:  remote.String(),
					KeyType:     key.Type(),
					Fingerprint: fp,
				})
				if cbErr != nil {
					return fmt.Errorf("host key confirmation error: %w", cbErr)
				}
				if !ok {
					return errors.New("подключение отклонено пользователем (HostKey не подтверждён)")
				}
				newHostKey = true
			}
			pinnedHostKey = key
			pinnedFingerprint = fp
			return nil
		},
		HostKeyAlgorithms: knownHostKeyAlgorithms(addr),
	}

	// Handshake включает ожидание решения пользователя по HostKey.
	passClient, err := dialSSHContext(ctx, addr, passCfg, timeout+HostKeyConfirmTimeout)
	if err != nil {
		return nil, fmt.Errorf("step 1 (password auth): %w", err)
	}
	defer passClient.Close()

	if newHostKey {
		if err := AddKnownHost(addr, pinnedHostKey); err != nil {
			return nil, fmt.Errorf("save host key: %w", err)
		}
	}

	emit(StepDeployKey, "Добавление публичного ключа в ~/.ssh/authorized_keys...")

	alreadyPresent, err := deployPublicKey(passClient, pubKey, req.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("step 2 (deploy key): %w", err)
	}
	_ = passClient.Close()

	emit(StepVerifyKey, "Проверка входа по приватному ключу без пароля...")

	keyCfg := &ssh.ClientConfig{
		User:            req.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         timeout,
		HostKeyCallback: ssh.FixedHostKey(pinnedHostKey),
	}

	verifyClient, err := dialSSHContext(ctx, addr, keyCfg, timeout)
	if err != nil {
		return nil, fmt.Errorf("step 3 (key verification failed): %w", err)
	}
	defer verifyClient.Close()

	if err := runSimpleCommand(verifyClient, "true"); err != nil {
		return nil, fmt.Errorf("step 3 (verify session command): %w", err)
	}

	emit(StepDone, "Ключ установлен и проверен. Можно подключаться!")

	return &BootstrapResult{
		HostKeyFingerprint: pinnedFingerprint,
		KeyAlreadyPresent:  alreadyPresent,
	}, nil
}

// deployScript выполняется строго в POSIX sh (а не в login-shell пользователя,
// который может быть fish/csh), поэтому записан в одну строку без переводов
// строк внутри кавычек. $1 — "type base64" для сравнения без учёта
// комментария, полная строка ключа приходит через stdin.
var deployScript = strings.Join([]string{
	`set -e`,
	`KEY=$(cat)`,
	`umask 077`,
	`F="$HOME/.ssh/authorized_keys"`,
	`mkdir -p "$HOME/.ssh"`,
	`chmod 700 "$HOME/.ssh"`,
	`touch "$F"`,
	`chmod 600 "$F"`,
	`if grep -qF -- "$1" "$F"; then echo EXISTS; else ` +
		`if [ -s "$F" ] && [ -n "$(tail -c1 "$F")" ]; then echo >> "$F"; fi; ` +
		`printf '%s\n' "$KEY" >> "$F"; echo ADDED; fi`,
}, "; ")

func deployPublicKey(client *ssh.Client, pubKey ssh.PublicKey, authorizedLine string) (alreadyPresent bool, err error) {
	cleanLine := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(authorizedLine, "\r", ""), "\n", ""))
	if cleanLine == "" {
		return false, errors.New("empty public key")
	}
	// Тело ключа состоит только из [A-Za-z0-9+/=-] и безопасно для shell-quoting.
	keyBody := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pubKey)))

	sess, err := client.NewSession()
	if err != nil {
		return false, err
	}
	defer sess.Close()

	sess.Stdin = strings.NewReader(cleanLine + "\n")
	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	cmd := "sh -c " + shellQuote(deployScript) + " sh " + shellQuote(keyBody)
	if err := sess.Run(cmd); err != nil {
		return false, fmt.Errorf("remote script failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	return strings.Contains(stdout.String(), "EXISTS"), nil
}

// shellQuote оборачивает строку в одинарные кавычки для POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// dialSSHContext устанавливает TCP-соединение и выполняет SSH handshake
// с жёстким дедлайном (ssh.ClientConfig.Timeout покрывает только TCP dial).
func dialSSHContext(ctx context.Context, addr string, cfg *ssh.ClientConfig, handshakeTimeout time.Duration) (*ssh.Client, error) {
	d := net.Dialer{Timeout: cfg.Timeout}
	rawConn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = rawConn.Close()
		case <-done:
		}
	}()

	if handshakeTimeout > 0 {
		_ = rawConn.SetDeadline(time.Now().Add(handshakeTimeout))
	}
	c, chans, reqs, err := ssh.NewClientConn(rawConn, addr, cfg)
	if err != nil {
		_ = rawConn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	_ = rawConn.SetDeadline(time.Time{})
	return ssh.NewClient(c, chans, reqs), nil
}

func parseSigner(privateKeyPEM, passphrase []byte) (ssh.Signer, error) {
	if len(passphrase) > 0 {
		return ssh.ParsePrivateKeyWithPassphrase(privateKeyPEM, passphrase)
	}
	return ssh.ParsePrivateKey(privateKeyPEM)
}

func runSimpleCommand(client *ssh.Client, cmd string) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	return sess.Run(cmd)
}

func validateBootstrapReq(req BootstrapRequest) error {
	if err := ValidateHostname(req.Hostname); err != nil {
		return err
	}
	if err := ValidatePort(req.Port); err != nil {
		return err
	}
	if err := ValidateUsername(req.Username); err != nil {
		return err
	}
	if len(req.Password) == 0 {
		return errors.New("password is required for bootstrap")
	}
	if len(req.PrivateKeyPEM) == 0 {
		return errors.New("private key is required for verification")
	}
	return nil
}
