package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// База версии 1 (v1.0–v1.3) должна обновиться без потери туннелей.
func TestMigrateV1ToRDPCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	init, _ := migrationsFS.ReadFile("migrations/001_init.sql")
	for _, q := range []string{string(init), "PRAGMA user_version = 1",
		`INSERT INTO hosts (id, name, hostname, username) VALUES ('h', 'h', '10.0.0.1', 'root')`,
		`INSERT INTO tunnels (id, host_id, name, direction, local_addr, remote_addr) VALUES ('t', 'h', 'rdp', 'L', '13389', 'localhost:3389')`,
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	raw.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	tn, err := store.Tunnels.GetByID(ctx, "t")
	if err != nil || tn.RemoteAddr != "localhost:3389" || tn.RDPUsername != "" || tn.HasRDPPassword {
		t.Fatalf("after migration: %+v, %v", tn, err)
	}
	if err := store.Tunnels.SetRDPCredentials(ctx, "t", "ivan", "", []byte{1, 2, 3}, false); err != nil {
		t.Fatal(err)
	}
	if tn, _ = store.Tunnels.GetByID(ctx, "t"); !tn.HasRDPPassword || tn.RDPUsername != "ivan" {
		t.Fatalf("credentials: %+v", tn)
	}
	var v int
	_ = store.DB.QueryRow("PRAGMA user_version").Scan(&v)
	if v != 2 {
		t.Fatalf("user_version = %d, want 2", v)
	}
}
