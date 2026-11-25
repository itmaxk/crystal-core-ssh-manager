package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var (
	// ErrHostKeyUnknown — хост ещё не записан в ~/.ssh/known_hosts.
	ErrHostKeyUnknown = errors.New("host key is not in ~/.ssh/known_hosts: connect once via terminal or One-Click Bootstrap to confirm it")
	// ErrHostKeyMismatch — ключ хоста отличается от сохранённого (возможна MITM-атака).
	ErrHostKeyMismatch = errors.New("REMOTE HOST KEY HAS CHANGED (possible MITM attack): fix ~/.ssh/known_hosts manually")
)

var knownHostsMu sync.Mutex

// KnownHostsPath возвращает путь к пользовательскому ~/.ssh/known_hosts,
// общему с системным OpenSSH-клиентом.
func KnownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

func loadKnownHosts() (ssh.HostKeyCallback, error) {
	path, err := KnownHostsPath()
	if err != nil {
		return nil, err
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return func(string, net.Addr, ssh.PublicKey) error {
				return &knownhosts.KeyError{}
			}, nil
		}
		return nil, fmt.Errorf("read known_hosts: %w", err)
	}
	return cb, nil
}

// checkKnownHost: (true, nil) — ключ совпал; (false, nil) — хост неизвестен;
// ErrHostKeyMismatch — для хоста записан другой ключ того же типа.
func checkKnownHost(hostname string, remote net.Addr, key ssh.PublicKey) (bool, error) {
	cb, err := loadKnownHosts()
	if err != nil {
		return false, err
	}
	err = cb(hostname, remote, key)
	if err == nil {
		return true, nil
	}
	var keyErr *knownhosts.KeyError
	if errors.As(err, &keyErr) {
		if len(keyErr.Want) == 0 {
			return false, nil
		}
		return false, fmt.Errorf("%w (%s, got %s %s)", ErrHostKeyMismatch, hostname, key.Type(), ssh.FingerprintSHA256(key))
	}
	return false, err
}

// StrictHostKeyCallback проверяет ключ хоста по ~/.ssh/known_hosts и
// отказывает в подключении к неизвестным хостам.
func StrictHostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		ok, err := checkKnownHost(hostname, remote, key)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w (%s %s)", ErrHostKeyUnknown, key.Type(), ssh.FingerprintSHA256(key))
		}
		return nil
	}
}

// knownHostKeyAlgorithms возвращает алгоритмы ключей, уже записанных для addr,
// чтобы сервер предъявил именно тот ключ, который есть в known_hosts
// (иначе x/crypto может выбрать другой тип и получить ложный mismatch).
func knownHostKeyAlgorithms(addr string) []string {
	cb, err := loadKnownHosts()
	if err != nil {
		return nil
	}
	probe := probeKey{}
	var keyErr *knownhosts.KeyError
	if err := cb(addr, &net.TCPAddr{}, probe); !errors.As(err, &keyErr) || len(keyErr.Want) == 0 {
		return nil
	}

	seen := map[string]bool{}
	var algos []string
	add := func(a string) {
		if !seen[a] {
			seen[a] = true
			algos = append(algos, a)
		}
	}
	for _, k := range keyErr.Want {
		switch t := k.Key.Type(); t {
		case ssh.KeyAlgoRSA:
			add(ssh.KeyAlgoRSASHA512)
			add(ssh.KeyAlgoRSASHA256)
			add(ssh.KeyAlgoRSA)
		default:
			add(t)
		}
	}
	return algos
}

// AddKnownHost дописывает подтверждённый пользователем ключ в ~/.ssh/known_hosts,
// чтобы системный ssh и туннели доверяли хосту без повторного вопроса.
func AddKnownHost(hostname string, key ssh.PublicKey) error {
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()

	path, err := KnownHostsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create .ssh dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open known_hosts: %w", err)
	}
	defer f.Close()

	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	if _, err := fmt.Fprintln(f, line); err != nil {
		return fmt.Errorf("write known_hosts: %w", err)
	}
	return nil
}

// probeKey — фиктивный ключ, который никогда не совпадает ни с одной записью.
type probeKey struct{}

func (probeKey) Type() string                        { return "crystal-core-probe" }
func (probeKey) Marshal() []byte                     { return []byte("crystal-core-probe") }
func (probeKey) Verify([]byte, *ssh.Signature) error { return errors.New("probe key") }
