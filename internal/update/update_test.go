package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.5.0", "1.4.0", 1}, {"v1.4.0", "1.4.0", 0}, {"1.10.0", "1.9.9", 1},
		{"1.4", "1.4.1", -1}, {"2.0.0-rc1", "1.9.0", 1}, {"dev", "1.0.0", -1},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseSums(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	m, err := ParseSums(strings.NewReader(sum + "  a.zip\n" + sum + " *b.tar.gz\nbad line\nzz  c\n"))
	if err != nil || m["a.zip"] != sum || m["b.tar.gz"] != sum || len(m) != 2 {
		t.Fatalf("%v %v", m, err)
	}
}

func makeTarGz(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func makeZip(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	return buf.Bytes()
}

// fakeGitHub отдаёт релиз с архивом и SHA256SUMS.txt.
func fakeGitHub(t *testing.T, tag, assetName string, archive []byte, sumOverride string) *httptest.Server {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(archive)
	hexSum := hex.EncodeToString(sum[:])
	if sumOverride != "" {
		hexSum = sumOverride
	}
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.UserAgent(), "CrystalCoreSSHManager/") {
			t.Errorf("user agent %q", r.UserAgent())
		}
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag, "body": "notes", "html_url": srv.URL + "/rel",
			"assets": []map[string]any{
				{"name": assetName, "browser_download_url": srv.URL + "/a/" + assetName, "size": len(archive)},
				{"name": "SHA256SUMS.txt", "browser_download_url": srv.URL + "/a/SHA256SUMS.txt", "size": 10},
			},
		})
	})
	mux.HandleFunc("/a/"+assetName, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/a/SHA256SUMS.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hexSum, assetName)
	})
	return srv
}

func newTestUpdater(t *testing.T, srv *httptest.Server, goos string) (*Updater, string) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "CrystalCoreSSHManager")
	if goos == "windows" {
		exe += ".exe"
		os.WriteFile(filepath.Join(dir, "ssh-launcher-cli.exe"), []byte("old-cli"), 0o755)
	}
	if err := os.WriteFile(exe, []byte("old-app"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := New("1.4.0")
	u.APIURL = srv.URL + "/latest"
	u.GOOS, u.GOARCH, u.ExePath = goos, "amd64", exe
	return u, exe
}

func TestCheckDownloadInstallLinux(t *testing.T) {
	archive := makeTarGz(t, map[string]string{"CrystalCoreSSHManager": "new-app", "README": "x"})
	srv := fakeGitHub(t, "v1.5.0", AssetName("linux", "amd64"), archive, "")
	u, exe := newTestUpdater(t, srv, "linux")

	in, err := u.Check(context.Background(), "1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if !in.Available || !in.CanInstall || in.Latest != "1.5.0" || in.Notes != "notes" {
		t.Fatalf("info: %+v", in)
	}
	var calls int
	st, err := u.Download(context.Background(), in, t.TempDir(), func(done, total int64) { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Error("no progress reported")
	}
	if err := u.Install(st); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new-app" {
		t.Fatalf("exe not replaced: %q", b)
	}
	// На Windows бита исполняемости нет.
	if fi, _ := os.Stat(exe); runtime.GOOS != "windows" && fi.Mode().Perm()&0o100 == 0 {
		t.Fatal("new exe is not executable")
	}
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), "*.old-*"))
	if len(left) != 0 {
		t.Fatalf("backups left on unix: %v", left)
	}
}

func TestInstallWindowsKeepsBackupsUntilCleanup(t *testing.T) {
	archive := makeZip(t, map[string]string{"CrystalCoreSSHManager.exe": "new-app", "ssh-launcher-cli.exe": "new-cli"})
	srv := fakeGitHub(t, "v1.5.0", AssetName("windows", "amd64"), archive, "")
	u, exe := newTestUpdater(t, srv, "windows")
	// Пользователь мог переименовать exe — ставим по фактическому пути.
	renamed := filepath.Join(filepath.Dir(exe), "SSH.exe")
	os.Rename(exe, renamed)
	u.ExePath = renamed

	in, err := u.Check(context.Background(), "1.4.0")
	if err != nil || !in.CanInstall {
		t.Fatalf("%+v %v", in, err)
	}
	st, err := u.Download(context.Background(), in, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Install(st); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(renamed)
	if b, _ := os.ReadFile(renamed); string(b) != "new-app" {
		t.Fatalf("app: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "ssh-launcher-cli.exe")); string(b) != "new-cli" {
		t.Fatalf("cli: %q", b)
	}
	old, _ := filepath.Glob(filepath.Join(dir, "*.old-*"))
	if len(old) != 2 {
		t.Fatalf("expected 2 backups, got %v", old)
	}
	u.CleanupOld()
	if old, _ = filepath.Glob(filepath.Join(dir, "*.old-*")); len(old) != 0 {
		t.Fatalf("cleanup left %v", old)
	}
}

func TestDownloadRejectsBadChecksum(t *testing.T) {
	archive := makeTarGz(t, map[string]string{"CrystalCoreSSHManager": "evil"})
	srv := fakeGitHub(t, "v1.5.0", AssetName("linux", "amd64"), archive, strings.Repeat("00", 32))
	u, exe := newTestUpdater(t, srv, "linux")
	in, _ := u.Check(context.Background(), "1.4.0")
	stage := t.TempDir()
	if _, err := u.Download(context.Background(), in, stage, nil); err == nil || !strings.Contains(err.Error(), "контрольная сумма") {
		t.Fatalf("err = %v", err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old-app" {
		t.Fatal("exe changed")
	}
	if left, _ := os.ReadDir(stage); len(left) != 0 {
		t.Fatalf("stage not cleaned: %v", left)
	}
}

func TestDownloadRejectsArchiveWithoutBinary(t *testing.T) {
	archive := makeTarGz(t, map[string]string{"../CrystalCoreSSHManager": "evil", "other": "x"})
	srv := fakeGitHub(t, "v1.5.0", AssetName("linux", "amd64"), archive, "")
	u, _ := newTestUpdater(t, srv, "linux")
	in, _ := u.Check(context.Background(), "1.4.0")
	if _, err := u.Download(context.Background(), in, t.TempDir(), nil); err == nil {
		t.Fatal("archive without expected binary accepted")
	}
}

func TestCheckNoUpdateAndUnsupported(t *testing.T) {
	srv := fakeGitHub(t, "v1.4.0", AssetName("linux", "amd64"), []byte("x"), "")
	u, _ := newTestUpdater(t, srv, "linux")
	in, err := u.Check(context.Background(), "1.4.0")
	if err != nil || in.Available {
		t.Fatalf("%+v %v", in, err)
	}

	srv2 := fakeGitHub(t, "v1.5.0", AssetName("linux", "amd64"), []byte("x"), "")
	u2, _ := newTestUpdater(t, srv2, "darwin")
	in, err = u2.Check(context.Background(), "1.4.0")
	if err != nil || !in.Available || in.CanInstall || in.Reason == "" {
		t.Fatalf("darwin: %+v %v", in, err)
	}
}

func TestInstallRollsBackOnFailure(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "CrystalCoreSSHManager.exe")
	os.WriteFile(exe, []byte("old-app"), 0o755)
	os.WriteFile(filepath.Join(dir, "ssh-launcher-cli.exe"), []byte("old-cli"), 0o755)
	stage := t.TempDir()
	newApp := filepath.Join(stage, "bin-app")
	os.WriteFile(newApp, []byte("new-app"), 0o755)
	u := New("1.4.0")
	u.GOOS, u.ExePath = "windows", exe
	// Нет файла для CLI — установка должна откатиться целиком.
	err := u.Install(&Staged{Dir: stage, Files: map[string]string{"CrystalCoreSSHManager.exe": newApp}})
	if err == nil {
		t.Fatal("expected error")
	}
	if b, _ := os.ReadFile(exe); string(b) != "old-app" {
		t.Fatalf("not rolled back: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "ssh-launcher-cli.exe")); string(b) != "old-cli" {
		t.Fatalf("cli changed: %q", b)
	}
}
