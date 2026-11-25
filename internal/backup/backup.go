// Package backup шифрует резервную копию паролем пользователя.
//
// Мастер-ключ хранилища живёт в системном keyring и не переносится между
// машинами, поэтому бэкап шифруется отдельным ключом, выведенным из пароля
// (Argon2id), и может быть восстановлен на любом компьютере.
package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	Format    = "crystal-core-ssh-manager-backup"
	Version   = 1
	Extension = ".ccbackup"

	MinPasswordLen = 8
)

var (
	ErrNotBackup     = errors.New("Файл не является резервной копией Crystal-Core SSH Manager")
	ErrUnsupported   = errors.New("Резервная копия создана более новой версией приложения")
	ErrWrongPassword = errors.New("Неверный пароль или файл повреждён")
)

type KDFParams struct {
	Name    string `json:"name"`
	Salt    []byte `json:"salt"`
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory_kib"`
	Threads uint8  `json:"threads"`
}

type envelope struct {
	Format     string    `json:"format"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	KDF        KDFParams `json:"kdf"`
	Nonce      []byte    `json:"nonce"`
	Ciphertext []byte    `json:"ciphertext"`
}

// DefaultKDF — параметры OWASP для Argon2id (64 МиБ, 3 прохода).
func DefaultKDF() (KDFParams, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return KDFParams{}, err
	}
	return KDFParams{Name: "argon2id", Salt: salt, Time: 3, Memory: 64 * 1024, Threads: 4}, nil
}

func deriveKey(password []byte, p KDFParams) ([]byte, error) {
	if p.Name != "argon2id" || len(p.Salt) < 16 || p.Time == 0 || p.Threads == 0 ||
		p.Memory < 8*1024 || p.Memory > 1024*1024 || p.Time > 64 {
		return nil, ErrUnsupported
	}
	return argon2.IDKey(password, p.Salt, p.Time, p.Memory, p.Threads, 32), nil
}

func aad() []byte { return []byte(fmt.Sprintf("%s/v%d", Format, Version)) }

// Seal шифрует plaintext паролем и возвращает JSON-файл бэкапа.
func Seal(plaintext, password []byte) ([]byte, error) {
	return sealWith(plaintext, password, nil)
}

func sealWith(plaintext, password []byte, kdf *KDFParams) ([]byte, error) {
	if len(password) < MinPasswordLen {
		return nil, fmt.Errorf("Пароль должен быть не короче %d символов", MinPasswordLen)
	}
	var params KDFParams
	if kdf != nil {
		params = *kdf
	} else {
		var err error
		if params, err = DefaultKDF(); err != nil {
			return nil, err
		}
	}
	key, err := deriveKey(password, params)
	if err != nil {
		return nil, err
	}
	defer zero(key)

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return json.MarshalIndent(envelope{
		Format:     Format,
		Version:    Version,
		CreatedAt:  time.Now().UTC(),
		KDF:        params,
		Nonce:      nonce,
		Ciphertext: gcm.Seal(nil, nonce, plaintext, aad()),
	}, "", "  ")
}

// Open расшифровывает файл бэкапа.
func Open(data, password []byte) ([]byte, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil || env.Format != Format {
		return nil, ErrNotBackup
	}
	if env.Version > Version {
		return nil, ErrUnsupported
	}
	key, err := deriveKey(password, env.KDF)
	if err != nil {
		return nil, err
	}
	defer zero(key)

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(env.Nonce) != gcm.NonceSize() {
		return nil, ErrNotBackup
	}
	plain, err := gcm.Open(nil, env.Nonce, env.Ciphertext, aad())
	if err != nil {
		return nil, ErrWrongPassword
	}
	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
