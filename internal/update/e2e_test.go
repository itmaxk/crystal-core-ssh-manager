package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRealGitHubE2E скачивает настоящий последний релиз (сеть, ~10 МБ):
// CCSM_UPDATE_E2E=1 go test ./internal/update -run E2E -v
func TestRealGitHubE2E(t *testing.T) {
	if os.Getenv("CCSM_UPDATE_E2E") == "" {
		t.Skip("set CCSM_UPDATE_E2E=1")
	}
	for _, goos := range []string{"windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			dir := t.TempDir()
			exe := filepath.Join(dir, "CrystalCoreSSHManager")
			if goos == "windows" {
				exe += ".exe"
			}
			os.WriteFile(exe, []byte("old"), 0o755)
			u := New("0.0.1")
			u.GOOS, u.GOARCH, u.ExePath = goos, "amd64", exe
			in, err := u.Check(context.Background(), "0.0.1")
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("latest %s, asset %s (%d bytes), can_install=%v %s", in.Latest, in.AssetName, in.AssetSize, in.CanInstall, in.Reason)
			st, err := u.Download(context.Background(), in, t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := u.Install(st); err != nil {
				t.Fatal(err)
			}
			fi, _ := os.Stat(exe)
			if fi.Size() < 1<<20 {
				t.Fatalf("installed exe too small: %d", fi.Size())
			}
			t.Logf("installed %s: %d bytes", filepath.Base(exe), fi.Size())
		})
	}
}
