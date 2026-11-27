package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// oldSuffix — так помечаются заменённые бинарники на Windows: запущенный
// .exe нельзя удалить или перезаписать, но можно переименовать.
const oldSuffix = ".old-"

type swap struct {
	target string
	backup string // "" — файла раньше не было
}

// Restart запускает новую версию отдельным процессом (флаг --after-update:
// она дождётся выхода текущей, см. main.go).
func Restart(exe string) error {
	cmd := exec.Command(exe, AfterUpdateFlag)
	cmd.Dir = filepath.Dir(exe)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// AfterUpdateFlag — аргумент запуска после обновления.
const AfterUpdateFlag = "--after-update"

// Install подменяет бинарники приложения скачанными. При ошибке уже
// заменённые файлы возвращаются на место.
func (u *Updater) Install(st *Staged) error {
	exe, err := u.exePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	mainName, extra := binaries(u.GOOS)
	plan := map[string]string{exe: st.Files[mainName]}
	for _, name := range extra {
		plan[filepath.Join(dir, name)] = st.Files[name]
	}

	var done []swap
	rollback := func() {
		for i := len(done) - 1; i >= 0; i-- {
			s := done[i]
			if s.backup != "" {
				_ = os.Rename(s.backup, s.target)
			} else {
				_ = os.Remove(s.target)
			}
		}
	}
	stamp := strconv.FormatInt(time.Now().UnixNano(), 36)
	for target, src := range plan {
		if src == "" {
			rollback()
			return fmt.Errorf("нет файла для %s", filepath.Base(target))
		}
		s, err := replaceFile(src, target, stamp)
		if err != nil {
			rollback()
			return fmt.Errorf("замена %s: %w", filepath.Base(target), err)
		}
		done = append(done, s)
	}
	if u.GOOS != "windows" {
		// На Unix запущенный процесс держит старый inode, резервные копии не нужны.
		for _, s := range done {
			if s.backup != "" {
				_ = os.Remove(s.backup)
			}
		}
	}
	_ = os.RemoveAll(st.Dir)
	return nil
}

// replaceFile копирует src рядом с target (в ту же ФС) и подставляет на его место.
func replaceFile(src, target, stamp string) (swap, error) {
	s := swap{target: target}
	tmp := target + ".new-" + stamp
	if err := copyFile(src, tmp); err != nil {
		_ = os.Remove(tmp)
		return s, err
	}
	if _, err := os.Stat(target); err == nil {
		s.backup = target + oldSuffix + stamp
		if err := os.Rename(target, s.backup); err != nil {
			_ = os.Remove(tmp)
			return s, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tmp)
		return s, err
	}
	if err := os.Rename(tmp, target); err != nil {
		if s.backup != "" {
			_ = os.Rename(s.backup, target)
		}
		_ = os.Remove(tmp)
		return s, err
	}
	return s, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// CleanupOld удаляет бинарники, оставшиеся от прошлого обновления
// (на Windows они освобождаются после выхода старого процесса).
func (u *Updater) CleanupOld() {
	exe, err := u.exePath()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	mainName, extra := binaries(u.GOOS)
	for _, base := range append([]string{filepath.Base(exe), mainName}, extra...) {
		matches, _ := filepath.Glob(filepath.Join(dir, base+oldSuffix+"*"))
		news, _ := filepath.Glob(filepath.Join(dir, base+".new-*"))
		for _, m := range append(matches, news...) {
			if strings.HasPrefix(filepath.Base(m), base) {
				_ = os.Remove(m)
			}
		}
	}
}
