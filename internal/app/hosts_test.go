package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"ssh-launcher/internal/crypto"
	"ssh-launcher/internal/db"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return &App{ctx: context.Background(), store: store, vault: &crypto.Vault{}}
}

func strPtr(s string) *string { return &s }

func TestSaveHostEdit(t *testing.T) {
	a := newTestApp(t)

	h, err := a.SaveHost(db.Host{Name: " web ", Hostname: "10.0.0.1", Username: "root", EnvTag: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if h.Name != "web" || h.Port != 22 {
		t.Fatalf("unexpected host after create: %+v", h)
	}

	h.Hostname = "10.0.0.2"
	h.Port = 2222
	h.EnvTag = "prod"
	edited, err := a.SaveHost(*h)
	if err != nil {
		t.Fatal(err)
	}
	if edited.ID != h.ID || edited.Hostname != "10.0.0.2" || edited.Port != 2222 || edited.EnvTag != "prod" {
		t.Fatalf("edit not applied: %+v", edited)
	}
	if hosts, _ := a.ListHosts(); len(hosts) != 1 {
		t.Fatalf("edit created a duplicate: %d hosts", len(hosts))
	}
}

func TestSaveHostRejectsUnknownRefs(t *testing.T) {
	a := newTestApp(t)

	if _, err := a.SaveHost(db.Host{Hostname: "h", Username: "u", KeyID: strPtr("missing")}); err == nil {
		t.Error("expected error for unknown key")
	}
	if _, err := a.SaveHost(db.Host{Hostname: "h", Username: "u", JumpHostID: strPtr("missing")}); err == nil {
		t.Error("expected error for unknown jump host")
	}
}

func TestSaveHostRejectsJumpCycle(t *testing.T) {
	a := newTestApp(t)

	bastion, err := a.SaveHost(db.Host{Name: "bastion", Hostname: "b", Username: "u"})
	if err != nil {
		t.Fatal(err)
	}
	inner, err := a.SaveHost(db.Host{Name: "inner", Hostname: "i", Username: "u", JumpHostID: &bastion.ID})
	if err != nil {
		t.Fatal(err)
	}

	bastion.JumpHostID = &inner.ID
	if _, err := a.SaveHost(*bastion); err == nil || !strings.Contains(err.Error(), "циклическая") {
		t.Fatalf("expected cycle error, got %v", err)
	}

	// Ссылка на себя тихо сбрасывается.
	inner.JumpHostID = &inner.ID
	got, err := a.SaveHost(*inner)
	if err != nil {
		t.Fatal(err)
	}
	if got.JumpHostID != nil {
		t.Errorf("self jump should be cleared, got %v", *got.JumpHostID)
	}
}

func TestConnectionChanged(t *testing.T) {
	base := db.Host{Hostname: "h", Port: 22, Username: "u", KeyID: strPtr("k1")}
	same := base
	same.Name, same.EnvTag = "renamed", "prod"
	if connectionChanged(&base, &same) {
		t.Error("name/env change must not restart tunnels")
	}
	for name, mutate := range map[string]func(*db.Host){
		"hostname": func(h *db.Host) { h.Hostname = "h2" },
		"port":     func(h *db.Host) { h.Port = 2222 },
		"user":     func(h *db.Host) { h.Username = "u2" },
		"key":      func(h *db.Host) { h.KeyID = nil },
	} {
		c := base
		mutate(&c)
		if !connectionChanged(&base, &c) {
			t.Errorf("%s change not detected", name)
		}
	}
}
