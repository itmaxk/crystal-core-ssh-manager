package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportPreservesUserConfigAndDedupsAliases(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	user := "Host personal\n    HostName 1.2.3.4\n"
	if err := os.WriteFile(cfg, []byte(user), 0600); err != nil {
		t.Fatal(err)
	}

	hosts := []SyncHost{
		{ID: "a", Name: "Web", Hostname: "10.0.0.1", Port: 22, Username: "root"},
		{ID: "b", Name: "web", Hostname: "10.0.0.2", Port: 2222, Username: "ubuntu", JumpHostID: "a"},
		{ID: "c", Name: "evil", Hostname: "-oProxyCommand=calc", Port: 22, Username: "root"},
	}
	for i := 0; i < 2; i++ { // повторный экспорт должен заменять блок, а не дублировать
		if err := ExportToSSHConfig(cfg, hosts); err != nil {
			t.Fatal(err)
		}
	}

	out, _ := os.ReadFile(cfg)
	s := string(out)
	if !strings.HasPrefix(s, user) {
		t.Errorf("user section was modified:\n%s", s)
	}
	if strings.Count(s, blockBegin) != 1 {
		t.Errorf("managed block duplicated:\n%s", s)
	}
	if !strings.Contains(s, "Host web-2\n") || !strings.Contains(s, "ProxyJump web\n") {
		t.Errorf("unexpected aliases:\n%s", s)
	}
	if strings.Contains(s, "ProxyCommand") {
		t.Errorf("invalid host leaked into config:\n%s", s)
	}

	imported, err := ImportFromSSHConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported) != 1 || imported[0].Name != "personal" {
		t.Errorf("import should skip managed block, got %+v", imported)
	}
}

func TestExportFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles-config")
	link := filepath.Join(dir, "config")
	if err := os.WriteFile(real, []byte("Host x\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	if err := ExportToSSHConfig(link, []SyncHost{{ID: "a", Name: "a", Hostname: "h", Username: "u"}}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(link)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink was replaced: %v", err)
	}
	b, _ := os.ReadFile(real)
	if !strings.Contains(string(b), blockBegin) {
		t.Error("managed block not written to symlink target")
	}
}

func TestValidateTunnelAddr(t *testing.T) {
	for _, ok := range []string{"8080", ":1080", "localhost:5432", "10.0.0.5:22", "[::1]:80"} {
		if err := ValidateTunnelAddr(ok); err != nil {
			t.Errorf("ValidateTunnelAddr(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "abc", "host:", "host:0", "host:70000", "-oX:22"} {
		if err := ValidateTunnelAddr(bad); err == nil {
			t.Errorf("ValidateTunnelAddr(%q) = nil, want error", bad)
		}
	}
}

func TestNormalizeListenAddr(t *testing.T) {
	cases := map[string]string{"8080": "127.0.0.1:8080", ":1080": "127.0.0.1:1080", "0.0.0.0:80": "0.0.0.0:80"}
	for in, want := range cases {
		if got := NormalizeListenAddr(in); got != want {
			t.Errorf("NormalizeListenAddr(%q) = %q, want %q", in, got, want)
		}
	}
}
