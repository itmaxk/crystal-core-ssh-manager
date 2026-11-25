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

func TestTunnelsRepo(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	h := &Host{Name: "db", Hostname: "10.0.0.1", Username: "root"}
	if err := store.Hosts.Upsert(ctx, h); err != nil {
		t.Fatal(err)
	}
	tn := &Tunnel{HostID: h.ID, Name: "pg", Direction: "L", LocalAddr: "15432", RemoteAddr: "localhost:5432", AutoStart: true}
	if err := store.Tunnels.Upsert(ctx, tn); err != nil {
		t.Fatal(err)
	}
	socks := &Tunnel{HostID: h.ID, Name: "socks", Direction: "D", LocalAddr: "1080"}
	if err := store.Tunnels.Upsert(ctx, socks); err != nil {
		t.Fatal(err)
	}

	got, err := store.Tunnels.GetByID(ctx, tn.ID)
	if err != nil || got.RemoteAddr != "localhost:5432" || !got.AutoStart {
		t.Fatalf("GetByID = %+v, %v", got, err)
	}
	if _, err := store.Tunnels.GetByID(ctx, "missing"); err == nil {
		t.Error("GetByID(missing) should fail")
	}
	all, err := store.Tunnels.ListAll(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("ListAll = %d, %v", len(all), err)
	}

	// Удаление хоста каскадно удаляет его туннели.
	if err := store.Hosts.Delete(ctx, h.ID); err != nil {
		t.Fatal(err)
	}
	if all, _ = store.Tunnels.ListAll(ctx); len(all) != 0 {
		t.Errorf("tunnels not cascaded on host delete: %d left", len(all))
	}
}
