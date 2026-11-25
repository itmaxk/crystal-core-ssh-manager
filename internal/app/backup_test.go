package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ssh-launcher/internal/backup"
	"ssh-launcher/internal/crypto"
	"ssh-launcher/internal/db"
)

// seedApp: ключ, bastion, сервер через bastion и туннель к нему.
func seedApp(t *testing.T, a *App) (keyID, bastionID, innerID string) {
	t.Helper()
	gen, err := crypto.GenerateEd25519("test", nil)
	if err != nil {
		t.Fatal(err)
	}
	k, err := a.AddKeyForTest(gen)
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.SaveHost(db.Host{Name: "bastion", Hostname: "b.example", Username: "ops", KeyID: &k.ID, EnvTag: "infra"})
	if err != nil {
		t.Fatal(err)
	}
	in, err := a.SaveHost(db.Host{Name: "inner", Hostname: "10.0.0.5", Username: "root", KeyID: &k.ID, JumpHostID: &b.ID, EnvTag: "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveTunnel(db.Tunnel{HostID: in.ID, Name: "rdp", Direction: "L", LocalAddr: "13389", RemoteAddr: "localhost:3389"}); err != nil {
		t.Fatal(err)
	}
	return k.ID, b.ID, in.ID
}

func TestBackupRoundTripToFreshInstall(t *testing.T) {
	src := newTestApp(t)
	keyID, bastionID, innerID := seedApp(t, src)

	path := filepath.Join(t.TempDir(), "b"+backup.Extension)
	if err := src.writeBackup(path, []byte("long password")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0600 && os.PathSeparator == '/' {
		t.Errorf("backup file mode %v, want 0600", info.Mode().Perm())
	}

	// Другая машина: другой мастер-ключ, пустая база.
	dst := newTestApp(t)
	if _, err := dst.ImportBackup(path, "wrong password"); !errors.Is(err, backup.ErrWrongPassword) {
		t.Fatalf("wrong password: got %v", err)
	}
	sum, err := dst.ImportBackup(path, "long password")
	if err != nil {
		t.Fatal(err)
	}
	if sum.KeysAdded != 1 || sum.HostsAdded != 2 || sum.TunnelsAdded != 1 || sum.Skipped != 0 {
		t.Fatalf("unexpected summary: %+v", sum)
	}

	inner, err := dst.store.Hosts.GetByID(dst.ctx, innerID)
	if err != nil {
		t.Fatal(err)
	}
	if inner.JumpHostID == nil || *inner.JumpHostID != bastionID || inner.KeyID == nil || *inner.KeyID != keyID {
		t.Fatalf("references lost: %+v", inner)
	}
	// Приватный ключ перешифрован мастер-ключом новой машины.
	k, err := dst.store.Keys.GetByID(dst.ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := dst.decryptKey(k); err != nil {
		t.Fatalf("imported key not decryptable: %v", err)
	}

	// Повторный импорт ничего не дублирует.
	sum, err = dst.ImportBackup(path, "long password")
	if err != nil {
		t.Fatal(err)
	}
	if sum.KeysAdded != 0 || sum.KeysExisting != 1 || sum.HostsAdded != 0 || sum.HostsUpdated != 2 || sum.TunnelsUpdated != 1 {
		t.Fatalf("re-import summary: %+v", sum)
	}
	keys, _ := dst.ListKeys()
	hosts, _ := dst.ListHosts()
	tunnels, _ := dst.ListAllTunnels()
	if len(keys) != 1 || len(hosts) != 2 || len(tunnels) != 1 {
		t.Fatalf("duplicates after re-import: %d keys, %d hosts, %d tunnels", len(keys), len(hosts), len(tunnels))
	}
}

func TestImportRemapsExistingKeyAndRenamesConflicts(t *testing.T) {
	src := newTestApp(t)
	_, _, innerID := seedApp(t, src)
	path := filepath.Join(t.TempDir(), "b"+backup.Extension)
	if err := src.writeBackup(path, []byte("long password")); err != nil {
		t.Fatal(err)
	}

	dst := newTestApp(t)
	// Тот же приватный ключ уже есть под другим ID, а имя "test" занято другим ключом.
	srcKeys, _ := src.ListKeys()
	full, _ := src.store.Keys.GetByID(src.ctx, srcKeys[0].ID)
	priv, _, _ := src.decryptKey(full)
	same, err := dst.AddKey(AddKeyInput{Name: "mine", KeyType: full.KeyType, Fingerprint: full.Fingerprint, PublicKey: full.PublicKey, PrivateKeyPEM: string(priv)})
	if err != nil {
		t.Fatal(err)
	}

	sum, err := dst.ImportBackup(path, "long password")
	if err != nil {
		t.Fatal(err)
	}
	if sum.KeysExisting != 1 || sum.KeysAdded != 0 {
		t.Fatalf("summary: %+v", sum)
	}
	inner, _ := dst.store.Hosts.GetByID(dst.ctx, innerID)
	if inner.KeyID == nil || *inner.KeyID != same.ID {
		t.Fatalf("host key not remapped to existing key: %v", inner.KeyID)
	}
}

func TestRDPConnectAddr(t *testing.T) {
	for in, want := range map[string]string{
		"13389":            "127.0.0.1:13389",
		":13389":           "127.0.0.1:13389",
		"0.0.0.0:13389":    "127.0.0.1:13389",
		"localhost:3390":   "127.0.0.1:3390",
		"192.168.1.5:3389": "192.168.1.5:3389",
	} {
		got, err := rdpConnectAddr(in)
		if err != nil || got != want {
			t.Errorf("rdpConnectAddr(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// AddKeyForTest сохраняет сгенерированный ключ под именем "test".
func (a *App) AddKeyForTest(gen *crypto.GeneratedKey) (*db.SSHKey, error) {
	return a.AddKey(AddKeyInput{
		Name: "test", KeyType: gen.KeyType, Fingerprint: gen.Fingerprint,
		PublicKey: gen.PublicKey, PrivateKeyPEM: string(gen.PrivateKeyPEM), IsDefault: true,
	})
}

// Экспорт в ~/.ssh/config ссылается на pubkeys/<id>.pub — файл должен
// существовать и для ключей, которыми ещё ни разу не подключались.
func TestSyncPubKeysRecreatesMissingFiles(t *testing.T) {
	a := newTestApp(t)
	keyID, _, _ := seedApp(t, a)
	a.pubKeyDir = t.TempDir()
	pub := filepath.Join(a.pubKeyDir, keyID+".pub")
	if _, err := os.Stat(pub); !os.IsNotExist(err) {
		t.Fatalf("precondition: pub file should be absent, got %v", err)
	}
	a.syncPubKeys()
	data, err := os.ReadFile(pub)
	if err != nil || len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("pub file not written: %q, %v", data, err)
	}
}
