package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "com.crystalcore.sshmanager"
	keyringUser    = "master-key"
	keySize        = 32 // AES-256
)

type Vault struct {
	aead cipher.AEAD
}

func NewVault() (*Vault, error) {
	masterKey, err := loadOrCreateMasterKey()
	if err != nil {
		return nil, fmt.Errorf("init master key: %w", err)
	}
	defer ZeroBytes(masterKey)
	return NewVaultFromKey(masterKey)
}

// NewVaultFromKey создаёт хранилище с заданным 32-байтным ключом (для тестов
// и инструментов; приложение использует ключ из системного keyring).
func NewVaultFromKey(masterKey []byte) (*Vault, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &Vault{aead: aead}, nil
}

func loadOrCreateMasterKey() ([]byte, error) {
	encoded, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		key, decErr := base64.StdEncoding.DecodeString(encoded)
		if decErr != nil || len(key) != keySize {
			return nil, errors.New("corrupted master key in system keyring")
		}
		return key, nil
	}

	if !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("read keyring: %w", err)
	}

	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}

	if err := keyring.Set(keyringService, keyringUser, base64.StdEncoding.EncodeToString(key)); err != nil {
		ZeroBytes(key)
		return nil, fmt.Errorf("save key to keyring: %w", err)
	}

	return key, nil
}

func (v *Vault) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return v.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (v *Vault) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := v.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, data := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := v.aead.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}
	return plaintext, nil
}

func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
