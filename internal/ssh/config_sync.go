package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	blockBegin = "# BEGIN CRYSTAL-CORE SSH MANAGER (managed automatically, do not edit)"
	blockEnd   = "# END CRYSTAL-CORE SSH MANAGER"
)

type SyncHost struct {
	ID         string
	Name       string
	Hostname   string
	Port       int
	Username   string
	PubKeyPath string
	JumpHostID string
}

type ImportedHost struct {
	Name         string `json:"name"`
	Hostname     string `json:"hostname"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	IdentityFile string `json:"identity_file,omitempty"`
}

func DefaultSSHConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

func ExportToSSHConfig(configPath string, hosts []SyncHost) error {
	if configPath == "" {
		var err error
		configPath, err = DefaultSSHConfigPath()
		if err != nil {
			return err
		}
	}

	// ~/.ssh/config часто является симлинком на dotfiles — пишем в реальный файл,
	// а не заменяем симлинк обычным файлом.
	if resolved, err := filepath.EvalSymlinks(configPath); err == nil {
		configPath = resolved
	}

	sshDir := filepath.Dir(configPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("create .ssh dir: %w", err)
	}

	existingBytes, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read ssh config: %w", err)
	}

	cleaned := stripManagedBlock(string(existingBytes))
	managed := buildManagedBlock(hosts)

	var out strings.Builder
	cleaned = strings.TrimRight(cleaned, "\r\n")
	if cleaned != "" {
		out.WriteString(cleaned)
		out.WriteString("\n\n")
	}
	if managed != "" {
		out.WriteString(managed)
		out.WriteString("\n")
	}

	tmpPath := configPath + ".crystal-core.tmp"
	if err := os.WriteFile(tmpPath, []byte(out.String()), 0600); err != nil {
		return fmt.Errorf("write temp ssh config: %w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace ssh config: %w", err)
	}
	return nil
}

func ImportFromSSHConfig(configPath string) ([]ImportedHost, error) {
	if configPath == "" {
		var err error
		configPath, err = DefaultSSHConfigPath()
		if err != nil {
			return nil, err
		}
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read ssh config: %w", err)
	}

	var result []ImportedHost
	var current *ImportedHost
	inManagedBlock := false

	flush := func() {
		if current != nil && current.Hostname != "" {
			if current.Port == 0 {
				current.Port = 22
			}
			if current.Username == "" {
				current.Username = "root"
			}
			result = append(result, *current)
		}
		current = nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "# BEGIN CRYSTAL-CORE SSH MANAGER") {
			flush()
			inManagedBlock = true
			continue
		}
		if strings.HasPrefix(line, "# END CRYSTAL-CORE SSH MANAGER") {
			inManagedBlock = false
			continue
		}
		if inManagedBlock || line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, val, ok := splitConfigDirective(line)
		if !ok {
			continue
		}

		switch strings.ToLower(key) {
		case "host":
			flush()
			fields := strings.Fields(val)
			if len(fields) == 0 || strings.ContainsAny(fields[0], "*?!") {
				continue
			}
			current = &ImportedHost{Name: fields[0], Hostname: fields[0], Port: 22}
		case "hostname":
			if current != nil {
				current.Hostname = val
			}
		case "user":
			if current != nil {
				current.Username = val
			}
		case "port":
			if current != nil {
				if p, err := strconv.Atoi(val); err == nil && p > 0 && p <= 65535 {
					current.Port = p
				}
			}
		case "identityfile":
			if current != nil && current.IdentityFile == "" {
				current.IdentityFile = strings.Trim(val, `"`)
			}
		}
	}

	flush()
	return result, scanner.Err()
}

var aliasRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func SanitizeAlias(name string) string {
	alias := strings.ToLower(strings.TrimSpace(name))
	alias = aliasRe.ReplaceAllString(alias, "-")
	alias = strings.Trim(alias, "-")
	if alias == "" {
		return "server"
	}
	return alias
}

func buildManagedBlock(hosts []SyncHost) string {
	valid := make([]SyncHost, 0, len(hosts))
	for _, h := range hosts {
		// Значения с пробелами/переводами строк могли бы внедрить директивы в конфиг.
		if ValidateHostname(h.Hostname) != nil || ValidateUsername(h.Username) != nil {
			continue
		}
		valid = append(valid, h)
	}
	if len(valid) == 0 {
		return ""
	}

	// Алиасы вычисляются один раз, чтобы ProxyJump ссылался на тот же
	// (дедуплицированный) алиас, что и сам jump-хост.
	aliasByID := make(map[string]string, len(valid))
	usedAliases := make(map[string]int, len(valid))
	for _, h := range valid {
		alias := SanitizeAlias(h.Name)
		if count := usedAliases[alias]; count > 0 {
			alias = fmt.Sprintf("%s-%d", alias, count+1)
		}
		usedAliases[alias]++
		aliasByID[h.ID] = alias
	}

	var b strings.Builder
	b.WriteString(blockBegin)
	b.WriteString("\n")

	for _, h := range valid {
		fmt.Fprintf(&b, "\nHost %s\n", aliasByID[h.ID])
		fmt.Fprintf(&b, "    HostName %s\n", h.Hostname)
		fmt.Fprintf(&b, "    User %s\n", h.Username)
		if h.Port > 0 && h.Port != 22 {
			fmt.Fprintf(&b, "    Port %d\n", h.Port)
		}
		if h.PubKeyPath != "" {
			b.WriteString("    IdentitiesOnly yes\n")
			fmt.Fprintf(&b, "    IdentityFile \"%s\"\n", filepath.ToSlash(h.PubKeyPath))
		}
		if jump, ok := aliasByID[h.JumpHostID]; ok && h.JumpHostID != "" && h.JumpHostID != h.ID {
			fmt.Fprintf(&b, "    ProxyJump %s\n", jump)
		}
	}

	b.WriteString("\n")
	b.WriteString(blockEnd)
	return b.String()
}

func stripManagedBlock(content string) string {
	var out strings.Builder
	inBlock := false

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# BEGIN CRYSTAL-CORE SSH MANAGER") {
			inBlock = true
			continue
		}
		if strings.HasPrefix(trimmed, "# END CRYSTAL-CORE SSH MANAGER") {
			inBlock = false
			continue
		}
		if !inBlock {
			out.WriteString(line)
			out.WriteString("\n")
		}
	}
	return out.String()
}

func splitConfigDirective(line string) (key, val string, ok bool) {
	idx := strings.IndexAny(line, " \t=")
	if idx <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	val = strings.TrimSpace(strings.TrimLeft(line[idx:], " \t="))
	return key, val, key != "" && val != ""
}
