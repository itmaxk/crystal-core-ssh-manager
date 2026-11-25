package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMigrateAndCRUD(t *testing.T) {
	// Пробелы и '#' в пути ломали DSN с префиксом "file:".
	path := filepath.Join(t.TempDir(), "my dir #1", "vault.db")
	if err := mkdirAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	k := &SSHKey{Name: "k", KeyType: "ed25519", Fingerprint: "SHA256:x", PublicKey: "ssh-ed25519 AAAA", PrivateKeyEnc: []byte{1}, IsDefault: true}
	if err := store.Keys.Create(ctx, k); err != nil {
		t.Fatal(err)
	}
	h := &Host{Name: "web", Hostname: "10.0.0.1", Username: "root", KeyID: &k.ID}
	if err := store.Hosts.Upsert(ctx, h); err != nil {
		t.Fatal(err)
	}
	if err := store.Hosts.TouchLastConnected(ctx, h.ID); err != nil {
		t.Fatal(err)
	}
	got, err := store.Hosts.GetByID(ctx, h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != 22 || got.LastConnectedAt == nil || got.KeyID == nil {
		t.Errorf("unexpected host: %+v", got)
	}

	// ON DELETE SET NULL требует foreign_keys(ON).
	if err := store.Keys.Delete(ctx, k.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = store.Hosts.GetByID(ctx, h.ID)
	if got.KeyID != nil {
		t.Errorf("key_id not cleared after key delete (foreign keys off?)")
	}

	// Повторное открытие не должно повторно применять миграции.
	store.Close()
	if store, err = Open(path); err != nil {
		t.Fatal(err)
	}
}

func mkdirAll(dir string) error { return os.MkdirAll(dir, 0700) }
