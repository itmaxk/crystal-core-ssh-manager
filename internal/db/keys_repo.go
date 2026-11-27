package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type SSHKey struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	KeyType       string    `json:"key_type"`
	Fingerprint   string    `json:"fingerprint"`
	PublicKey     string    `json:"public_key"`
	PrivateKeyEnc []byte    `json:"-"`
	PassphraseEnc []byte    `json:"-"`
	HasPassphrase bool      `json:"has_passphrase"`
	IsDefault     bool      `json:"is_default"`
	CreatedAt     time.Time `json:"created_at" ts_type:"string"`
	UpdatedAt     time.Time `json:"updated_at" ts_type:"string"`
}

type KeysRepo struct {
	db *sql.DB
}

func (r *KeysRepo) Create(ctx context.Context, k *SSHKey) error {
	if k.ID == "" {
		k.ID = uuid.NewString()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if k.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE ssh_keys SET is_default = 0`); err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO ssh_keys (id, name, key_type, fingerprint, public_key, private_key_enc, passphrase_enc, is_default)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		k.ID, k.Name, k.KeyType, k.Fingerprint, k.PublicKey, k.PrivateKeyEnc, k.PassphraseEnc, boolToInt(k.IsDefault),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *KeysRepo) List(ctx context.Context) ([]SSHKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, key_type, fingerprint, public_key, is_default, created_at, updated_at,
		       length(coalesce(passphrase_enc, '')) > 0
		FROM ssh_keys
		ORDER BY is_default DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []SSHKey
	for rows.Next() {
		var k SSHKey
		var isDef int
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyType, &k.Fingerprint, &k.PublicKey, &isDef, &k.CreatedAt, &k.UpdatedAt, &k.HasPassphrase); err != nil {
			return nil, err
		}
		k.IsDefault = isDef == 1
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *KeysRepo) GetByID(ctx context.Context, id string) (*SSHKey, error) {
	var k SSHKey
	var isDef int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, key_type, fingerprint, public_key, private_key_enc, passphrase_enc, is_default, created_at, updated_at
		FROM ssh_keys WHERE id = ?`, id,
	).Scan(&k.ID, &k.Name, &k.KeyType, &k.Fingerprint, &k.PublicKey, &k.PrivateKeyEnc, &k.PassphraseEnc, &isDef, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, err
	}
	k.IsDefault = isDef == 1
	k.HasPassphrase = len(k.PassphraseEnc) > 0
	return &k, nil
}

func (r *KeysRepo) SetDefault(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE ssh_keys SET is_default = 0`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE ssh_keys SET is_default = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *KeysRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ssh_keys WHERE id = ?`, id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
