package app

import (
	"sync"
	"testing"
	"time"
)

// fakeClipboard имитирует системный буфер; mark — номер записи.
type fakeClipboard struct {
	mu   sync.Mutex
	text string
	seq  int
}

func (f *fakeClipboard) SetSecret(text string) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.text = text
	f.seq++
	return f.seq, nil
}

func (f *fakeClipboard) Owns(mark any) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return mark == f.seq
}

func (f *fakeClipboard) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.text = ""
	f.seq++
	return nil
}

func (f *fakeClipboard) userCopy(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.text = text
	f.seq++
}

func (f *fakeClipboard) get() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.text
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestCopyRDPPasswordAutoClears(t *testing.T) {
	old := secretClipboardTTL
	secretClipboardTTL = 50 * time.Millisecond
	t.Cleanup(func() { secretClipboardTTL = old })

	a := newTestApp(t)
	clip := &fakeClipboard{}
	a.clipboard = clip
	tn := rdpTunnel(t, a)

	if _, err := a.CopyRDPPassword(tn.ID); err == nil {
		t.Fatal("copy without saved password should fail")
	}
	if err := a.SaveRDPCredentials(RDPCredentialsInput{TunnelID: tn.ID, Username: "ivan", Password: "s3cret"}); err != nil {
		t.Fatal(err)
	}
	res, err := a.CopyRDPPassword(tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if clip.get() != "s3cret" || res.Seconds != 1 || res.Label == "" {
		t.Fatalf("copied %q, %+v", clip.get(), res)
	}
	waitFor(t, func() bool { return clip.get() == "" })

	// Если пользователь успел скопировать своё, таймер буфер не трогает.
	if _, err := a.CopyRDPPassword(tn.ID); err != nil {
		t.Fatal(err)
	}
	clip.userCopy("mine")
	time.Sleep(120 * time.Millisecond)
	if clip.get() != "mine" {
		t.Fatalf("user clipboard overwritten: %q", clip.get())
	}
}

func TestClearSecretClipboardManually(t *testing.T) {
	a := newTestApp(t)
	clip := &fakeClipboard{}
	a.clipboard = clip
	tn := rdpTunnel(t, a)
	if err := a.SaveRDPCredentials(RDPCredentialsInput{TunnelID: tn.ID, Username: "ivan", Password: "s3cret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CopyRDPPassword(tn.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.ClearSecretClipboard(); err != nil || clip.get() != "" {
		t.Fatalf("clear: %v, %q", err, clip.get())
	}
	// Повторная очистка без секрета в буфере ничего не стирает.
	clip.userCopy("mine")
	if err := a.ClearSecretClipboard(); err != nil || clip.get() != "mine" {
		t.Fatalf("second clear: %v, %q", err, clip.get())
	}
}

func TestCopyKeyPassphrase(t *testing.T) {
	a := newTestApp(t)
	clip := &fakeClipboard{}
	a.clipboard = clip

	withPass, err := a.GenerateKey("with", "phrase-123", false)
	if err != nil {
		t.Fatal(err)
	}
	noPass, err := a.GenerateKey("without", "", false)
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := a.ListKeys()
	for _, k := range keys {
		if k.HasPassphrase != (k.ID == withPass.ID) {
			t.Fatalf("has_passphrase wrong for %s", k.Name)
		}
	}
	if _, err := a.CopyKeyPassphrase(noPass.ID); err == nil {
		t.Fatal("key without passphrase should fail")
	}
	if _, err := a.CopyKeyPassphrase(withPass.ID); err != nil || clip.get() != "phrase-123" {
		t.Fatalf("copy: %v, %q", err, clip.get())
	}
	_ = a.ClearSecretClipboard()
}
