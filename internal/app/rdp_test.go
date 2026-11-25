package app

import (
	"bytes"
	"path/filepath"
	"testing"

	"ssh-launcher/internal/backup"
	"ssh-launcher/internal/db"
)

func rdpTunnel(t *testing.T, a *App) *db.Tunnel {
	t.Helper()
	h, err := a.SaveHost(db.Host{Name: "win", Hostname: "10.0.0.7", Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	tn, err := a.SaveTunnel(db.Tunnel{HostID: h.ID, Name: "RDP", Direction: "L", LocalAddr: "13389", RemoteAddr: "localhost:3389"})
	if err != nil {
		t.Fatal(err)
	}
	return tn
}

func TestRDPCredentialsLifecycle(t *testing.T) {
	a := newTestApp(t)
	tn := rdpTunnel(t, a)

	if err := a.SaveRDPCredentials(RDPCredentialsInput{TunnelID: tn.ID, Username: " ivan ", Domain: "CORP", Password: "p@ss w0rd"}); err != nil {
		t.Fatal(err)
	}
	got, _ := a.store.Tunnels.GetByID(a.ctx, tn.ID)
	if got.RDPUsername != "ivan" || got.RDPDomain != "CORP" || !got.HasRDPPassword {
		t.Fatalf("saved: %+v", got)
	}
	enc, _ := a.store.Tunnels.RDPPasswordEnc(a.ctx, tn.ID)
	if bytes.Contains(enc, []byte("p@ss")) {
		t.Fatal("RDP password stored in plaintext")
	}
	creds, err := a.rdpCredentials(got)
	if err != nil || string(creds.Password) != "p@ss w0rd" || creds.Account() != `CORP\ivan` {
		t.Fatalf("creds: %+v, %v", creds, err)
	}

	// Пустой пароль с KeepPassword сохраняет прежний.
	if err := a.SaveRDPCredentials(RDPCredentialsInput{TunnelID: tn.ID, Username: "ivan2", KeepPassword: true}); err != nil {
		t.Fatal(err)
	}
	got, _ = a.store.Tunnels.GetByID(a.ctx, tn.ID)
	if got.RDPUsername != "ivan2" || got.RDPDomain != "" || !got.HasRDPPassword {
		t.Fatalf("keep password: %+v", got)
	}
	// Без KeepPassword пустой пароль удаляется.
	if err := a.SaveRDPCredentials(RDPCredentialsInput{TunnelID: tn.ID, Username: "ivan2"}); err != nil {
		t.Fatal(err)
	}
	if got, _ = a.store.Tunnels.GetByID(a.ctx, tn.ID); got.HasRDPPassword {
		t.Fatal("password should be cleared")
	}
	// Редактирование туннеля не трогает учётные данные.
	got.Name = "RDP renamed"
	if _, err := a.SaveTunnel(*got); err != nil {
		t.Fatal(err)
	}
	if got, _ = a.store.Tunnels.GetByID(a.ctx, tn.ID); got.RDPUsername != "ivan2" {
		t.Fatalf("SaveTunnel wiped RDP credentials: %+v", got)
	}
	// Пустой логин удаляет всё.
	if err := a.SaveRDPCredentials(RDPCredentialsInput{TunnelID: tn.ID}); err != nil {
		t.Fatal(err)
	}
	if got, _ = a.store.Tunnels.GetByID(a.ctx, tn.ID); got.RDPUsername != "" {
		t.Fatalf("not cleared: %+v", got)
	}
	if err := a.SaveRDPCredentials(RDPCredentialsInput{TunnelID: tn.ID, Username: "a", Password: "x\ny"}); err == nil {
		t.Fatal("newline in password accepted")
	}
	if err := a.SaveRDPCredentials(RDPCredentialsInput{TunnelID: "missing", Username: "a"}); err == nil {
		t.Fatal("unknown tunnel accepted")
	}
}

func TestBackupCarriesRDPCredentials(t *testing.T) {
	src := newTestApp(t)
	tn := rdpTunnel(t, src)
	if err := src.SaveRDPCredentials(RDPCredentialsInput{TunnelID: tn.ID, Username: "ivan", Password: "secret-rdp"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "b"+backup.Extension)
	if err := src.writeBackup(path, []byte("long password")); err != nil {
		t.Fatal(err)
	}

	dst := newTestApp(t) // другой мастер-ключ
	if _, err := dst.ImportBackup(path, "long password"); err != nil {
		t.Fatal(err)
	}
	got, err := dst.store.Tunnels.GetByID(dst.ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	creds, err := dst.rdpCredentials(got)
	if err != nil || creds == nil || creds.Username != "ivan" || string(creds.Password) != "secret-rdp" {
		t.Fatalf("RDP credentials not restored: %+v, %v", creds, err)
	}
}
