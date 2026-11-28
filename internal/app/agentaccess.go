package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	sshsvc "ssh-launcher/internal/ssh"
)

const agentAccessFileName = "agent-access.json"

// AgentGrant описывает выданный агенту доступ к одному серверу.
type AgentGrant struct {
	HostID     string `json:"host_id"`
	HostName   string `json:"host_name"`
	Mode       string `json:"mode"` // shell | tunnel
	SourceIP   string `json:"source_ip,omitempty"`
	PermitOpen string `json:"permit_open,omitempty"`
	SSHCommand string `json:"ssh_command"`
	CreatedAt  string `json:"created_at"`
}

// AgentAccessState — состояние локального пользователя-агента и его доступов.
type AgentAccessState struct {
	Supported  bool         `json:"supported"`
	Username   string       `json:"username"`
	Home       string       `json:"home"`
	PublicKey  string       `json:"public_key"`
	CreatedAt  string       `json:"created_at"`
	UserExists bool         `json:"user_exists"`
	Grants     []AgentGrant `json:"grants"`
}

type AgentSetupInput struct {
	Username  string `json:"username"`
	CloseHome bool   `json:"close_home"`
}

type AgentSetupResult struct {
	State          AgentAccessState `json:"state"`
	NeedsElevation bool             `json:"needs_elevation"`
	ScriptPath     string           `json:"script_path,omitempty"`
	Command        string           `json:"command,omitempty"`
	Output         string           `json:"output,omitempty"`
}

type AgentGrantInput struct {
	HostID     string `json:"host_id"`
	Mode       string `json:"mode"`
	SourceIP   string `json:"source_ip"`
	PermitOpen string `json:"permit_open"`
}

type agentAccessFile struct {
	Username  string       `json:"username"`
	Home      string       `json:"home"`
	PublicKey string       `json:"public_key"`
	CreatedAt string       `json:"created_at"`
	Grants    []AgentGrant `json:"grants,omitempty"`
}

type agentSetupFile struct {
	Username  string `json:"username"`
	Home      string `json:"home"`
	PublicKey string `json:"public_key"`
	CreatedAt string `json:"created_at"`
}

var agentUsernameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{2,31}$`)

func agentConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "CrystalCoreSSHManager"), nil
}

func loadAgentAccess() (*agentAccessFile, error) {
	path, err := agentConfigDir()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(path, agentAccessFileName))
	if errors.Is(err, os.ErrNotExist) {
		return &agentAccessFile{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg agentAccessFile
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent access config: %w", err)
	}
	return &cfg, nil
}

func saveAgentAccess(cfg *agentAccessFile) error {
	path, err := agentAccessConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func agentAccessConfigPath() (string, error) {
	dir, err := agentConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, agentAccessFileName), nil
}

// mergeAgentSetupFile переносит результат root-скрипта в основной конфиг.
func mergeAgentSetupFile(cfg *agentAccessFile) (*agentAccessFile, error) {
	dir, err := agentConfigDir()
	if err != nil {
		return cfg, err
	}
	path := filepath.Join(dir, "agent-setup.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	var setup agentSetupFile
	if err := json.Unmarshal(raw, &setup); err != nil {
		return cfg, fmt.Errorf("parse agent setup result: %w", err)
	}
	if setup.Username == "" || setup.PublicKey == "" {
		return cfg, errors.New("agent setup result is incomplete")
	}
	if cfg.Username != "" && cfg.Username != setup.Username {
		// Сменили пользователя — старые записи на серверах больше не актуальны.
		cfg.Grants = nil
	}
	if cfg.PublicKey != "" && cfg.PublicKey != setup.PublicKey {
		// Ключ пересоздан: старые строки authorized_keys больше не соответствуют
		// новому ключу, поэтому сбрасываем учёт доступов.
		cfg.Grants = nil
	}
	cfg.Username = setup.Username
	cfg.Home = setup.Home
	cfg.PublicKey = setup.PublicKey
	cfg.CreatedAt = setup.CreatedAt
	if err := saveAgentAccess(cfg); err != nil {
		return cfg, err
	}
	_ = os.Remove(path)
	return cfg, nil
}

func (a *App) stateFromConfig(cfg *agentAccessFile) AgentAccessState {
	st := AgentAccessState{
		Supported: agentSetupSupported(),
		Username:  cfg.Username,
		Home:      cfg.Home,
		PublicKey: cfg.PublicKey,
		CreatedAt: cfg.CreatedAt,
		Grants:    cfg.Grants,
	}
	if cfg.Username != "" {
		if _, err := user.Lookup(cfg.Username); err == nil {
			st.UserExists = true
		}
	}
	if st.Grants == nil {
		st.Grants = []AgentGrant{}
	}
	return st
}

// GetAgentAccess возвращает состояние без попытки что-либо менять.
func (a *App) GetAgentAccess() (AgentAccessState, error) {
	a.agentAccessMu.Lock()
	defer a.agentAccessMu.Unlock()

	cfg, err := loadAgentAccess()
	if err != nil {
		return AgentAccessState{}, err
	}
	cfg, err = mergeAgentSetupFile(cfg)
	if err != nil {
		return a.stateFromConfig(cfg), err
	}
	return a.stateFromConfig(cfg), nil
}

// SetupAgentUser создаёт отдельного локального пользователя и ключ Ed25519.
func (a *App) SetupAgentUser(in AgentSetupInput) (AgentSetupResult, error) {
	a.agentAccessMu.Lock()
	defer a.agentAccessMu.Unlock()

	name := strings.TrimSpace(in.Username)
	if !agentUsernameRe.MatchString(name) {
		return AgentSetupResult{}, errors.New("имя пользователя: 3–32 символа, латиница в нижнем регистре, цифры, _ и -")
	}
	if name == "root" || name == "ubuntu" || name == "admin" {
		return AgentSetupResult{}, errors.New("это имя зарезервировано")
	}

	cfg, err := loadAgentAccess()
	if err != nil {
		return AgentSetupResult{}, err
	}

	res, err := setupLocalAgentUser(name, in.CloseHome)
	if err != nil {
		return AgentSetupResult{}, err
	}
	cfg, err = mergeAgentSetupFile(cfg)
	if err != nil {
		return res, err
	}
	res.State = a.stateFromConfig(cfg)
	if !res.NeedsElevation && res.ScriptPath != "" {
		_ = os.RemoveAll(filepath.Dir(res.ScriptPath))
	}
	return res, nil
}

// GrantAgentAccess добавляет ограниченную строку в authorized_keys на сервере.
func (a *App) GrantAgentAccess(in AgentGrantInput) (AgentGrant, error) {
	if err := a.ready(); err != nil {
		return AgentGrant{}, err
	}
	a.agentAccessMu.Lock()
	defer a.agentAccessMu.Unlock()

	cfg, err := loadAgentAccess()
	if err != nil {
		return AgentGrant{}, err
	}
	cfg, err = mergeAgentSetupFile(cfg)
	if err != nil {
		return AgentGrant{}, err
	}
	if cfg.Username == "" || cfg.PublicKey == "" {
		return AgentGrant{}, errors.New("сначала создайте пользователя-агента")
	}

	mode := strings.TrimSpace(in.Mode)
	if mode != "shell" && mode != "tunnel" {
		return AgentGrant{}, errors.New("режим доступа: shell или tunnel")
	}
	source := strings.TrimSpace(in.SourceIP)
	if err := validateAgentSource(source); err != nil {
		return AgentGrant{}, err
	}
	permit := strings.TrimSpace(in.PermitOpen)
	if mode == "tunnel" {
		if err := validatePermitOpen(permit); err != nil {
			return AgentGrant{}, err
		}
	} else {
		permit = ""
	}

	host, err := a.store.Hosts.GetByID(context.Background(), in.HostID)
	if err != nil {
		return AgentGrant{}, err
	}
	if host.KeyID == nil || *host.KeyID == "" {
		return AgentGrant{}, errors.New("у сервера нет SSH-ключа в приложении")
	}
	k, err := a.store.Keys.GetByID(context.Background(), *host.KeyID)
	if err != nil {
		return AgentGrant{}, err
	}
	privPEM, passphrase, err := a.decryptKey(k)
	if err != nil {
		return AgentGrant{}, err
	}
	defer zeroBytes(privPEM)
	defer zeroBytes(passphrase)

	line, err := buildAuthorizedKeyLine(cfg.PublicKey, cfg.Username, mode, source, permit)
	if err != nil {
		return AgentGrant{}, err
	}
	marker := agentGrantMarker(cfg.Username)
	script := authorizedKeysScript("upsert", marker)
	cmd := fmt.Sprintf("sh -c %s sh %s %s", shQuote(script), shQuote(marker), shQuote(line))

	if _, err := sshsvc.RunCommand(context.Background(), sshsvc.RunRequest{
		Hostname:      host.Hostname,
		Port:          host.Port,
		Username:      host.Username,
		PrivateKeyPEM: privPEM,
		Passphrase:    passphrase,
		Command:       cmd,
	}); err != nil {
		return AgentGrant{}, err
	}

	grant := AgentGrant{
		HostID:     host.ID,
		HostName:   host.Name,
		Mode:       mode,
		SourceIP:   source,
		PermitOpen: permit,
		SSHCommand: agentSSHCommand(cfg.Username, cfg.Home, host.Username, host.Hostname, host.Port, mode, permit),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	replaced := false
	for i := range cfg.Grants {
		if cfg.Grants[i].HostID == host.ID {
			cfg.Grants[i] = grant
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Grants = append(cfg.Grants, grant)
	}
	if err := saveAgentAccess(cfg); err != nil {
		return grant, err
	}
	return grant, nil
}

// RevokeAgentAccess удаляет выданную агенту строку из authorized_keys.
func (a *App) RevokeAgentAccess(hostID string) error {
	if err := a.ready(); err != nil {
		return err
	}
	a.agentAccessMu.Lock()
	defer a.agentAccessMu.Unlock()

	cfg, err := loadAgentAccess()
	if err != nil {
		return err
	}
	if cfg.Username == "" {
		return errors.New("агент не настроен")
	}

	var grant *AgentGrant
	for i := range cfg.Grants {
		if cfg.Grants[i].HostID == hostID {
			grant = &cfg.Grants[i]
			break
		}
	}
	if grant == nil {
		return errors.New("доступ для этого сервера не найден")
	}

	host, err := a.store.Hosts.GetByID(context.Background(), hostID)
	if err != nil {
		return err
	}
	if host.KeyID == nil || *host.KeyID == "" {
		return errors.New("у сервера нет SSH-ключа в приложении")
	}
	k, err := a.store.Keys.GetByID(context.Background(), *host.KeyID)
	if err != nil {
		return err
	}
	privPEM, passphrase, err := a.decryptKey(k)
	if err != nil {
		return err
	}
	defer zeroBytes(privPEM)
	defer zeroBytes(passphrase)

	marker := agentGrantMarker(cfg.Username)
	script := authorizedKeysScript("remove", marker)
	cmd := fmt.Sprintf("sh -c %s sh %s", shQuote(script), shQuote(marker))
	if _, err := sshsvc.RunCommand(context.Background(), sshsvc.RunRequest{
		Hostname:      host.Hostname,
		Port:          host.Port,
		Username:      host.Username,
		PrivateKeyPEM: privPEM,
		Passphrase:    passphrase,
		Command:       cmd,
	}); err != nil {
		return err
	}

	kept := cfg.Grants[:0]
	for _, g := range cfg.Grants {
		if g.HostID != hostID {
			kept = append(kept, g)
		}
	}
	cfg.Grants = kept
	return saveAgentAccess(cfg)
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func agentGrantMarker(username string) string {
	return "crystal-core-agent:" + username
}

func buildAuthorizedKeyLine(publicKey, username, mode, source, permit string) (string, error) {
	pub := strings.TrimSpace(publicKey)
	if pub == "" || !strings.HasPrefix(pub, "ssh-") {
		return "", errors.New("некорректный публичный ключ агента")
	}
	pub = strings.ReplaceAll(strings.ReplaceAll(pub, "\r", ""), "\n", "")

	var opts []string
	if source != "" {
		opts = append(opts, fmt.Sprintf(`from="%s"`, source))
	}
	switch mode {
	case "shell":
		opts = append(opts,
			"no-port-forwarding", "no-agent-forwarding", "no-x11-forwarding", "no-user-rc")
	case "tunnel":
		opts = append(opts, "restrict", "port-forwarding", fmt.Sprintf(`permitopen="%s"`, permit))
	default:
		return "", errors.New("неизвестный режим доступа")
	}
	return strings.Join(opts, ",") + " " + pub + " " + agentGrantMarker(username), nil
}

func validateAgentSource(source string) error {
	if source == "" {
		return nil
	}
	for _, part := range strings.Split(source, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return errors.New("пустое значение в списке IP")
		}
		if net.ParseIP(part) == nil {
			if _, _, err := net.ParseCIDR(part); err != nil {
				return fmt.Errorf("IP или CIDR %q некорректен", part)
			}
		}
	}
	return nil
}

func validatePermitOpen(permit string) error {
	host, portStr, err := net.SplitHostPort(permit)
	if err != nil || host == "" {
		return errors.New(`укажите адрес в виде "127.0.0.1:5432"`)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("порт должен быть 1–65535")
	}
	for _, r := range host {
		if !(r == '.' || r == '-' || r == '_' || r == ':' || r == '[' || r == ']' ||
			r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return errors.New("в адресе допустимы только буквы, цифры, точка, дефис, _ и :")
		}
	}
	return nil
}

func agentSSHCommand(agentUser, home, remoteUser, hostname string, port int, mode, permit string) string {
	key := filepath.Join(home, ".ssh", "id_ed25519")
	target := fmt.Sprintf("%s@%s", remoteUser, hostname)
	portArg := ""
	if port != 22 {
		portArg = fmt.Sprintf(" -p %d", port)
	}
	if mode == "tunnel" {
		return fmt.Sprintf("sudo -u %s ssh -N -L 15432:%s -i %s%s %s",
			agentUser, permit, key, portArg, target)
	}
	return fmt.Sprintf("sudo -u %s ssh -i %s%s %s", agentUser, key, portArg, target)
}

// authorizedKeysScript добавляет или удаляет строку с marker в authorized_keys.
func authorizedKeysScript(action, marker string) string {
	remove := `grep -vF -- "$1" "$f" > "$tmp" 2>/dev/null || true`
	add := `printf '%s\n' "$2" >> "$tmp"`
	if action == "remove" {
		add = ":"
	}
	return `set -eu
umask 077
f="$HOME/.ssh/authorized_keys"
mkdir -p "$HOME/.ssh"
chmod 700 "$HOME/.ssh"
touch "$f"
chmod 600 "$f"
tmp="$f.crystal-core.tmp"
` + remove + `
` + add + `
cat "$tmp" > "$f"
rm -f "$tmp"
chmod 600 "$f"`
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
