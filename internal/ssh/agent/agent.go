package agent

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
	"ssh-launcher/internal/crypto"
)

func IsAvailable() bool {
	conn, err := dialAgent()
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

const (
	// keyLifetime ограничивает время жизни ключа в ssh-agent: даже если
	// приложение не выгрузит ключ, агент забудет его через 8 часов.
	keyLifetime = 8 * time.Hour
)

func AddKey(privateKeyPEM, passphrase []byte, comment string) error {
	defer crypto.ZeroBytes(privateKeyPEM)
	defer crypto.ZeroBytes(passphrase)

	var (
		rawKey any
		err    error
	)

	if len(passphrase) > 0 {
		rawKey, err = ssh.ParseRawPrivateKeyWithPassphrase(privateKeyPEM, passphrase)
	} else {
		rawKey, err = ssh.ParseRawPrivateKey(privateKeyPEM)
	}
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}

	conn, err := dialAgent()
	if err != nil {
		return fmt.Errorf("connect to ssh-agent: %w", err)
	}
	defer conn.Close()

	client := sshagent.NewClient(conn)
	added := sshagent.AddedKey{
		PrivateKey:   rawKey,
		Comment:      comment,
		LifetimeSecs: uint32(keyLifetime / time.Second),
	}
	if err := client.Add(added); err != nil {
		// Часть агентов (например, Windows OpenSSH) не поддерживает TTL.
		// Не ломаем подключение: добавляем без срока жизни, но приложение
		// всё равно выгрузит ключ при сворачивании/выходе.
		added.LifetimeSecs = 0
		if fallbackErr := client.Add(added); fallbackErr != nil {
			return fmt.Errorf("add key to ssh-agent: %w", err)
		}
	}

	return nil
}

func ListFingerprints() (map[string]bool, error) {
	conn, err := dialAgent()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	keys, err := sshagent.NewClient(conn).List()
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(keys))
	for _, k := range keys {
		result[ssh.FingerprintSHA256(k)] = true
	}
	return result, nil
}

// RemoveKey выгружает ключ из ssh-agent (например, при удалении из Key Vault).
// Отсутствие ключа в агенте ошибкой не считается.
func RemoveKey(authorizedKey string) error {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}
	loaded, err := ListFingerprints()
	if err != nil {
		return err
	}
	if !loaded[ssh.FingerprintSHA256(pub)] {
		return nil
	}

	conn, err := dialAgent()
	if err != nil {
		return fmt.Errorf("connect to ssh-agent: %w", err)
	}
	defer conn.Close()
	return sshagent.NewClient(conn).Remove(pub)
}
