//go:build !windows

package app

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	sshagent "golang.org/x/crypto/ssh/agent"

	"ssh-launcher/internal/crypto"
	"ssh-launcher/internal/db"
	"ssh-launcher/internal/ssh/agent"
)

func startTestAgent(t *testing.T) {
	t.Helper()

	sock := filepath.Join(t.TempDir(), "agent.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	keyring := sshagent.NewKeyring()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = sshagent.ServeAgent(keyring, conn)
			}()
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", sock)
}

func TestLockKeysRemovesVaultKeys(t *testing.T) {
	startTestAgent(t)

	gen, err := crypto.GenerateEd25519("lock-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := agent.AddKey(gen.PrivateKeyPEM, nil, "lock-test"); err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	store, err := db.Open(filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	if err := store.Keys.Create(ctx, &db.SSHKey{
		Name:          "lock-test",
		KeyType:       gen.KeyType,
		Fingerprint:   gen.Fingerprint,
		PublicKey:     gen.PublicKey,
		PrivateKeyEnc: []byte{1},
	}); err != nil {
		t.Fatal(err)
	}

	a := &App{ctx: ctx, store: store}
	removed, err := a.LockKeys()
	if err != nil {
		t.Fatalf("LockKeys: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}

	loaded, err := agent.ListFingerprints()
	if err != nil {
		t.Fatal(err)
	}
	if loaded[gen.Fingerprint] {
		t.Fatal("vault key still present in ssh-agent after LockKeys")
	}
}
