package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

type KeyMetadata struct {
	KeyType     string `json:"key_type"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"public_key"`
}

type GeneratedKey struct {
	KeyMetadata
	PrivateKeyPEM []byte
}

func GenerateEd25519(comment string, passphrase []byte) (*GeneratedKey, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	defer ZeroBytes(privKey)

	var pemBlock *pem.Block
	if len(passphrase) > 0 {
		pemBlock, err = ssh.MarshalPrivateKeyWithPassphrase(privKey, comment, passphrase)
	} else {
		pemBlock, err = ssh.MarshalPrivateKey(privKey, comment)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal openssh private key: %w", err)
	}

	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("convert public key: %w", err)
	}

	pubLine := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))
	if cleanComment := strings.TrimSpace(comment); cleanComment != "" {
		pubLine = fmt.Sprintf("%s %s", pubLine, cleanComment)
	}

	return &GeneratedKey{
		KeyMetadata: KeyMetadata{
			KeyType:     "ed25519",
			Fingerprint: ssh.FingerprintSHA256(sshPub),
			PublicKey:   pubLine,
		},
		PrivateKeyPEM: pem.EncodeToMemory(pemBlock),
	}, nil
}

func ParseKeyMetadata(privateKeyPEM, passphrase []byte) (*KeyMetadata, error) {
	var (
		signer ssh.Signer
		err    error
	)

	if len(passphrase) > 0 {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(privateKeyPEM, passphrase)
	} else {
		signer, err = ssh.ParsePrivateKey(privateKeyPEM)
	}
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	pub := signer.PublicKey()
	return &KeyMetadata{
		KeyType:     normalizeKeyType(pub.Type()),
		Fingerprint: ssh.FingerprintSHA256(pub),
		PublicKey:   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub))),
	}, nil
}

func FingerprintFromPublicKey(authorizedKey string) (string, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
	if err != nil {
		return "", fmt.Errorf("parse public key: %w", err)
	}
	return ssh.FingerprintSHA256(pub), nil
}

func normalizeKeyType(sshType string) string {
	switch {
	case sshType == ssh.KeyAlgoED25519:
		return "ed25519"
	case sshType == ssh.KeyAlgoRSA:
		return "rsa"
	case strings.HasPrefix(sshType, "ecdsa-"):
		return "ecdsa"
	default:
		return sshType
	}
}
