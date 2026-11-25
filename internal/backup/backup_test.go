package backup

import (
	"bytes"
	"errors"
	"testing"
)

// Облегчённые параметры, чтобы тесты не тратили 64 МиБ на каждый вызов.
func fastKDF(t *testing.T) *KDFParams {
	p, err := DefaultKDF()
	if err != nil {
		t.Fatal(err)
	}
	p.Memory, p.Time, p.Threads = 8*1024, 1, 1
	return &p
}

func TestSealOpenRoundTrip(t *testing.T) {
	data, err := sealWith([]byte(`{"hosts":[]}`), []byte("correct horse"), fastKDF(t))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("hosts")) {
		t.Fatal("plaintext leaked into backup file")
	}
	got, err := Open(data, []byte("correct horse"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"hosts":[]}` {
		t.Fatalf("got %q", got)
	}
}

func TestOpenErrors(t *testing.T) {
	data, err := sealWith([]byte("x"), []byte("password1"), fastKDF(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(data, []byte("password2")); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("wrong password: got %v", err)
	}
	if _, err := Open([]byte(`{"format":"other"}`), []byte("password1")); !errors.Is(err, ErrNotBackup) {
		t.Errorf("foreign file: got %v", err)
	}
	if _, err := Open([]byte("not json"), []byte("password1")); !errors.Is(err, ErrNotBackup) {
		t.Errorf("garbage: got %v", err)
	}
	if _, err := Seal([]byte("x"), []byte("short")); err == nil {
		t.Error("short password accepted")
	}
}
