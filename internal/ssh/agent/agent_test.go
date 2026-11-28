//go:build !windows

package agent

import (
	"net"
	"path/filepath"
	"testing"

	sshagent "golang.org/x/crypto/ssh/agent"

	"ssh-launcher/internal/crypto"
)

// startTestAgent поднимает временный ssh-agent на unix-сокете, чтобы тесты
// не трогали агент разработчика.
func startTestAgent(t *testing.T) string {
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
	return sock
}

func TestAddListRemoveKey(t *testing.T) {
	startTestAgent(t)

	gen, err := crypto.GenerateEd25519("test-key", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := AddKey(gen.PrivateKeyPEM, nil, "test-key"); err != nil {
		t.Fatalf("AddKey: %v", err)
	}

	loaded, err := ListFingerprints()
	if err != nil {
		t.Fatalf("ListFingerprints: %v", err)
	}
	if !loaded[gen.Fingerprint] {
		t.Fatalf("key %s not loaded into agent", gen.Fingerprint)
	}

	if err := RemoveKey(gen.PublicKey); err != nil {
		t.Fatalf("RemoveKey: %v", err)
	}
	loaded, err = ListFingerprints()
	if err != nil {
		t.Fatalf("ListFingerprints after remove: %v", err)
	}
	if loaded[gen.Fingerprint] {
		t.Fatalf("key %s still loaded after RemoveKey", gen.Fingerprint)
	}
}

func TestRemoveMissingKeyIsNoop(t *testing.T) {
	startTestAgent(t)

	gen, err := crypto.GenerateEd25519("missing", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := RemoveKey(gen.PublicKey); err != nil {
		t.Fatalf("RemoveKey missing: %v", err)
	}
}
