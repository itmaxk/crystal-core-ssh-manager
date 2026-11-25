package agent

import (
	"fmt"

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
	err = client.Add(sshagent.AddedKey{
		PrivateKey: rawKey,
		Comment:    comment,
	})
	if err != nil {
		return fmt.Errorf("add key to ssh-agent: %w", err)
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
