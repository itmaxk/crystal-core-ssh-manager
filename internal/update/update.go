// Package update проверяет GitHub Releases, скачивает сборку для текущей
// платформы, сверяет SHA256 и подменяет исполняемые файлы приложения.
package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	Repo        = "itmaxk/crystal-core-ssh-manager"
	ReleasesURL = "https://github.com/" + Repo + "/releases"
	latestAPI   = "https://api.github.com/repos/" + Repo + "/releases/latest"
	sumsName    = "SHA256SUMS.txt"

	maxArchiveSize = 300 << 20 // защита от бесконечной загрузки
	maxBinarySize  = 300 << 20
)

// Info — результат проверки обновлений для UI.
type Info struct {
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Available  bool   `json:"available"`
	Notes      string `json:"notes"`
	URL        string `json:"url"`
	Published  string `json:"published"`
	AssetName  string `json:"asset_name,omitempty"`
	AssetSize  int64  `json:"asset_size,omitempty"`
	CanInstall bool   `json:"can_install"`
	// Reason объясняет, почему установить автоматически нельзя.
	Reason string `json:"reason,omitempty"`

	assetURL string
	sumsURL  string
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// Updater хранит параметры, которые подменяются в тестах.
type Updater struct {
	Client    *http.Client
	APIURL    string
	UserAgent string
	GOOS      string
	GOARCH    string
	// ExePath — путь к запущенному бинарнику (os.Executable по умолчанию).
	ExePath string
}

func New(currentVersion string) *Updater {
	return &Updater{
		Client:    &http.Client{Timeout: 10 * time.Minute},
		APIURL:    latestAPI,
		UserAgent: "CrystalCoreSSHManager/" + currentVersion,
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
	}
}

// AssetName — имя архива релиза для платформы ("" — сборки нет).
func AssetName(goos, goarch string) string {
	if goarch != "amd64" {
		return ""
	}
	switch goos {
	case "windows":
		return "crystal-core-ssh-manager-windows-amd64.zip"
	case "linux":
		return "crystal-core-ssh-manager-linux-amd64.tar.gz"
	}
	return ""
}

// binaries — какие файлы из архива ставить и куда (относительно папки exe).
// Главный бинарник ставится по фактическому пути exe, даже если файл переименован.
func binaries(goos string) (main string, extra []string) {
	if goos == "windows" {
		return "CrystalCoreSSHManager.exe", []string{"ssh-launcher-cli.exe"}
	}
	return "CrystalCoreSSHManager", nil
}

// Check запрашивает последний релиз и сравнивает версии.
func (u *Updater) Check(ctx context.Context, current string) (*Info, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.APIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", u.UserAgent)
	client := *u.Client
	client.Timeout = 20 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("нет связи с GitHub: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, errors.New("GitHub ограничил частоту запросов, попробуйте позже")
	case resp.StatusCode == http.StatusNotFound:
		return nil, errors.New("релизы не найдены")
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GitHub ответил %s", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("разбор ответа GitHub: %w", err)
	}
	return u.info(current, &rel), nil
}

func (u *Updater) info(current string, rel *ghRelease) *Info {
	in := &Info{
		Current: strings.TrimPrefix(current, "v"),
		Latest:  strings.TrimPrefix(rel.TagName, "v"),
		Notes:   rel.Body,
		URL:     rel.HTMLURL,
	}
	if in.URL == "" {
		in.URL = ReleasesURL
	}
	if !rel.PublishedAt.IsZero() {
		in.Published = rel.PublishedAt.Format(time.RFC3339)
	}
	in.Available = !rel.Draft && !rel.Prerelease && CompareVersions(in.Latest, in.Current) > 0
	if !in.Available {
		return in
	}

	name := AssetName(u.GOOS, u.GOARCH)
	for _, a := range rel.Assets {
		switch a.Name {
		case name:
			in.AssetName, in.AssetSize, in.assetURL = a.Name, a.Size, a.URL
		case sumsName:
			in.sumsURL = a.URL
		}
	}
	switch {
	case name == "":
		in.Reason = "Для этой платформы нет автоматической установки — скачайте сборку со страницы релиза."
	case in.assetURL == "":
		in.Reason = "В релизе нет сборки для этой платформы — скачайте её со страницы релиза, когда она появится."
	case in.sumsURL == "":
		in.Reason = "В релизе нет SHA256SUMS.txt — без проверки контрольной суммы обновление не устанавливается."
	default:
		if err := u.checkWritable(); err != nil {
			in.Reason = err.Error()
		} else {
			in.CanInstall = true
		}
	}
	return in
}

func (u *Updater) exePath() (string, error) {
	if u.ExePath != "" {
		return u.ExePath, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// checkWritable проверяет, что в папку приложения можно писать без прав администратора.
func (u *Updater) checkWritable() error {
	exe, err := u.exePath()
	if err != nil {
		return fmt.Errorf("не удалось определить путь к программе: %w", err)
	}
	dir := filepath.Dir(exe)
	f, err := os.CreateTemp(dir, ".ccsm-write-test-*")
	if err != nil {
		return fmt.Errorf("нет прав на запись в %s — обновите вручную или переместите программу в папку пользователя", dir)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

// CompareVersions сравнивает версии вида 1.2.3 (суффиксы -rc и т.п. игнорируются).
func CompareVersions(a, b string) int {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		n, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		out[i] = n
	}
	return out
}

// Progress сообщает о ходе загрузки (total может быть 0, если размер неизвестен).
type Progress func(done, total int64)

// Staged — скачанное и проверенное обновление, готовое к установке.
type Staged struct {
	Version string
	Dir     string
	// Files: имя файла в архиве → путь распакованного файла.
	Files map[string]string
}

// Download скачивает архив в stageRoot, сверяет SHA256 и распаковывает нужные бинарники.
func (u *Updater) Download(ctx context.Context, in *Info, stageRoot string, progress Progress) (*Staged, error) {
	if in == nil || !in.CanInstall {
		return nil, errors.New("обновление нельзя установить автоматически")
	}
	sums, err := u.fetchSums(ctx, in.sumsURL)
	if err != nil {
		return nil, err
	}
	want, ok := sums[in.AssetName]
	if !ok {
		return nil, fmt.Errorf("в SHA256SUMS.txt нет %s", in.AssetName)
	}

	if err := os.MkdirAll(stageRoot, 0o700); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(stageRoot, "v"+in.Latest+"-")
	if err != nil {
		return nil, err
	}
	ok = false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir)
		}
	}()

	archive := filepath.Join(dir, in.AssetName)
	got, err := u.fetchFile(ctx, in.assetURL, archive, progress)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(got, want) {
		return nil, fmt.Errorf("контрольная сумма не совпала (%s): файл повреждён или подменён", in.AssetName)
	}

	mainName, extra := binaries(u.GOOS)
	wanted := append([]string{mainName}, extra...)
	files, err := extract(archive, dir, wanted)
	if err != nil {
		return nil, fmt.Errorf("распаковка: %w", err)
	}
	for _, name := range wanted {
		if _, found := files[name]; !found {
			return nil, fmt.Errorf("в архиве нет %s", name)
		}
	}
	_ = os.Remove(archive)
	ok = true
	return &Staged{Version: in.Latest, Dir: dir, Files: files}, nil
}

func (u *Updater) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", u.UserAgent)
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("загрузка: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("загрузка %s: %s", filepath.Base(url), resp.Status)
	}
	return resp, nil
}

func (u *Updater) fetchSums(ctx context.Context, url string) (map[string]string, error) {
	resp, err := u.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ParseSums(io.LimitReader(resp.Body, 1<<20))
}

// ParseSums читает вывод sha256sum: "<hex>  <имя>" (или "<hex> *<имя>").
func ParseSums(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		sum, name := strings.ToLower(fields[0]), strings.TrimPrefix(fields[1], "*")
		if b, err := hex.DecodeString(sum); err != nil || len(b) != sha256.Size {
			continue
		}
		out[name] = sum
	}
	return out, sc.Err()
}

func (u *Updater) fetchFile(ctx context.Context, url, dst string, progress Progress) (string, error) {
	resp, err := u.get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.ContentLength > maxArchiveSize {
		return "", errors.New("архив обновления слишком большой")
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	pr := &progressReader{r: io.LimitReader(resp.Body, maxArchiveSize+1), total: resp.ContentLength, fn: progress}
	n, err := io.Copy(io.MultiWriter(f, h), pr)
	if err != nil {
		return "", fmt.Errorf("загрузка прервана: %w", err)
	}
	if n > maxArchiveSize {
		return "", errors.New("архив обновления слишком большой")
	}
	if progress != nil {
		progress(n, n)
	}
	return hex.EncodeToString(h.Sum(nil)), f.Close()
}

type progressReader struct {
	r     io.Reader
	done  int64
	total int64
	fn    Progress
	last  time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.fn != nil && time.Since(p.last) > 150*time.Millisecond {
		p.last = time.Now()
		p.fn(p.done, p.total)
	}
	return n, err
}

// extract достаёт из архива только файлы с именами из wanted (без подпапок),
// так что пути из архива никогда не используются для записи.
func extract(archive, dir string, wanted []string) (map[string]string, error) {
	want := map[string]bool{}
	for _, w := range wanted {
		want[w] = true
	}
	files := map[string]string{}
	write := func(name string, r io.Reader) error {
		dst := filepath.Join(dir, "bin-"+name)
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		n, err := io.Copy(f, io.LimitReader(r, maxBinarySize+1))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		if n > maxBinarySize {
			return fmt.Errorf("%s слишком большой", name)
		}
		files[name] = dst
		return nil
	}

	if strings.HasSuffix(archive, ".zip") {
		zr, err := zip.OpenReader(archive)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		for _, zf := range zr.File {
			if zf.FileInfo().IsDir() || !want[zf.Name] {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				return nil, err
			}
			err = write(zf.Name, rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
		}
		return files, nil
	}

	f, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if hdr.Typeflag != tar.TypeReg || !want[name] {
			continue
		}
		if err := write(name, tr); err != nil {
			return nil, err
		}
	}
}
