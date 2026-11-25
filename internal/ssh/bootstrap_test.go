package ssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Проверяет удалённый скрипт установки ключа локально через POSIX sh.
func TestDeployScriptIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX sh")
	}
	home := t.TempDir()
	body := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEXAMPLEKEYBODY"
	akPath := filepath.Join(home, ".ssh", "authorized_keys")
	_ = os.MkdirAll(filepath.Dir(akPath), 0700)
	// Существующий файл без завершающего перевода строки.
	if err := os.WriteFile(akPath, []byte("ssh-rsa AAAAother user@old"), 0600); err != nil {
		t.Fatal(err)
	}

	run := func(line string) string {
		cmd := exec.Command("sh", "-c", deployScript, "sh", body)
		cmd.Env = append(os.Environ(), "HOME="+home)
		cmd.Stdin = strings.NewReader(line + "\n")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("script failed: %v: %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}

	if got := run(body + " first-comment"); got != "ADDED" {
		t.Fatalf("first run = %q, want ADDED", got)
	}
	// Тот же ключ с другим комментарием не должен дублироваться.
	if got := run(body + " other-comment"); got != "EXISTS" {
		t.Fatalf("second run = %q, want EXISTS", got)
	}

	data, _ := os.ReadFile(akPath)
	want := "ssh-rsa AAAAother user@old\n" + body + " first-comment\n"
	if string(data) != want {
		t.Errorf("authorized_keys =\n%q\nwant\n%q", data, want)
	}
	if fi, _ := os.Stat(akPath); fi.Mode().Perm() != 0600 {
		t.Errorf("authorized_keys perms = %v", fi.Mode().Perm())
	}
}

func TestShellQuote(t *testing.T) {
	out, err := exec.Command("sh", "-c", "printf %s "+shellQuote(`it's "q" $HOME`)).Output()
	if err != nil {
		t.Skip("sh unavailable")
	}
	if string(out) != `it's "q" $HOME` {
		t.Errorf("shellQuote roundtrip = %q", out)
	}
}
