package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-launcher/internal/backup"
	"ssh-launcher/internal/crypto"
	"ssh-launcher/internal/db"
	sshsvc "ssh-launcher/internal/ssh"
)

type backupKey struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	KeyType       string `json:"key_type"`
	Fingerprint   string `json:"fingerprint"`
	PublicKey     string `json:"public_key"`
	PrivateKeyPEM string `json:"private_key_pem"`
	Passphrase    string `json:"passphrase,omitempty"`
	IsDefault     bool   `json:"is_default"`
}

type backupPayload struct {
	ExportedAt time.Time   `json:"exported_at"`
	Keys       []backupKey `json:"keys"`
	Hosts      []db.Host   `json:"hosts"`
	Tunnels    []db.Tunnel `json:"tunnels"`
	// Пароли RDP по ID туннеля (логин и домен лежат в самих туннелях).
	RDPPasswords map[string]string `json:"rdp_passwords,omitempty"`
}

// ImportSummary — итог импорта для уведомления в UI.
type ImportSummary struct {
	KeysAdded      int `json:"keys_added"`
	KeysExisting   int `json:"keys_existing"`
	HostsAdded     int `json:"hosts_added"`
	HostsUpdated   int `json:"hosts_updated"`
	TunnelsAdded   int `json:"tunnels_added"`
	TunnelsUpdated int `json:"tunnels_updated"`
	Skipped        int `json:"skipped"`
}

// ExportBackup спрашивает, куда сохранить файл, и записывает зашифрованный
// паролем бэкап. Пустой путь — пользователь отменил диалог.
func (a *App) ExportBackup(password string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	if len([]rune(password)) < backup.MinPasswordLen {
		return "", fmt.Errorf("Пароль должен быть не короче %d символов", backup.MinPasswordLen)
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "Сохранить резервную копию",
		DefaultFilename:      "crystal-core-backup-" + time.Now().Format("2006-01-02") + backup.Extension,
		Filters:              []runtime.FileFilter{{DisplayName: "Резервная копия (*" + backup.Extension + ")", Pattern: "*" + backup.Extension}},
		CanCreateDirectories: true,
	})
	if err != nil || path == "" {
		return "", err
	}
	if filepath.Ext(path) == "" {
		path += backup.Extension
	}
	if err := a.writeBackup(path, []byte(password)); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) writeBackup(path string, password []byte) error {
	plain, err := a.buildBackup()
	if err != nil {
		return err
	}
	defer crypto.ZeroBytes(plain)
	sealed, err := backup.Seal(plain, password)
	if err != nil {
		return err
	}
	return os.WriteFile(path, sealed, 0600)
}

func (a *App) buildBackup() ([]byte, error) {
	keys, err := a.store.Keys.List(a.ctx)
	if err != nil {
		return nil, err
	}
	hosts, err := a.store.Hosts.List(a.ctx)
	if err != nil {
		return nil, err
	}
	tunnels, err := a.store.Tunnels.ListAll(a.ctx)
	if err != nil {
		return nil, err
	}

	p := backupPayload{ExportedAt: time.Now().UTC(), Hosts: hosts, Tunnels: tunnels}
	for _, summary := range keys {
		k, err := a.store.Keys.GetByID(a.ctx, summary.ID)
		if err != nil {
			return nil, err
		}
		privPEM, passphrase, err := a.decryptKey(k)
		if err != nil {
			return nil, fmt.Errorf("ключ %q: %w", k.Name, err)
		}
		p.Keys = append(p.Keys, backupKey{
			ID: k.ID, Name: k.Name, KeyType: k.KeyType, Fingerprint: k.Fingerprint, PublicKey: k.PublicKey,
			PrivateKeyPEM: string(privPEM), Passphrase: string(passphrase), IsDefault: k.IsDefault,
		})
		crypto.ZeroBytes(privPEM)
		crypto.ZeroBytes(passphrase)
	}
	for _, t := range tunnels {
		if !t.HasRDPPassword {
			continue
		}
		enc, err := a.store.Tunnels.RDPPasswordEnc(a.ctx, t.ID)
		if err != nil {
			return nil, err
		}
		pw, err := a.vault.Decrypt(enc)
		if err != nil {
			return nil, fmt.Errorf("пароль RDP туннеля %q: %w", t.Name, err)
		}
		if p.RDPPasswords == nil {
			p.RDPPasswords = map[string]string{}
		}
		p.RDPPasswords[t.ID] = string(pw)
		crypto.ZeroBytes(pw)
	}
	if p.Hosts == nil {
		p.Hosts = []db.Host{}
	}
	if p.Tunnels == nil {
		p.Tunnels = []db.Tunnel{}
	}
	return json.Marshal(p)
}

// PickBackupFile открывает диалог выбора файла бэкапа. Пустой путь — отмена.
func (a *App) PickBackupFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Открыть резервную копию",
		Filters: []runtime.FileFilter{{DisplayName: "Резервная копия (*" + backup.Extension + ")", Pattern: "*" + backup.Extension}},
	})
}

// ImportBackup расшифровывает файл и сливает его с текущими данными:
// совпадающие по отпечатку ключи не дублируются, серверы и туннели с теми же
// ID обновляются, остальные добавляются.
func (a *App) ImportBackup(path, password string) (*ImportSummary, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение файла: %w", err)
	}
	plain, err := backup.Open(data, []byte(password))
	if err != nil {
		return nil, err
	}
	defer crypto.ZeroBytes(plain)

	var p backupPayload
	if err := json.Unmarshal(plain, &p); err != nil {
		return nil, backup.ErrNotBackup
	}
	sum, err := a.applyBackup(&p)
	for i := range p.Keys {
		p.Keys[i].PrivateKeyPEM, p.Keys[i].Passphrase = "", ""
	}
	p.RDPPasswords = nil
	a.RefreshHostsStatus()
	return sum, err
}

func (a *App) applyBackup(p *backupPayload) (*ImportSummary, error) {
	sum := &ImportSummary{}

	// --- Ключи ---
	existingKeys, err := a.store.Keys.List(a.ctx)
	if err != nil {
		return nil, err
	}
	byFingerprint := map[string]string{}
	usedNames := map[string]bool{}
	hasDefault := false
	for _, k := range existingKeys {
		byFingerprint[k.Fingerprint] = k.ID
		usedNames[k.Name] = true
		hasDefault = hasDefault || k.IsDefault
	}
	keyIDMap := map[string]string{} // ID в бэкапе → ID в базе
	for _, bk := range p.Keys {
		if id, ok := byFingerprint[bk.Fingerprint]; ok && bk.Fingerprint != "" {
			keyIDMap[bk.ID] = id
			sum.KeysExisting++
			continue
		}
		if strings.TrimSpace(bk.PrivateKeyPEM) == "" || strings.TrimSpace(bk.PublicKey) == "" {
			sum.Skipped++
			continue
		}
		k, err := a.importKey(bk, usedNames, !hasDefault && bk.IsDefault)
		if err != nil {
			return sum, fmt.Errorf("ключ %q: %w", bk.Name, err)
		}
		hasDefault = hasDefault || k.IsDefault
		byFingerprint[k.Fingerprint] = k.ID
		keyIDMap[bk.ID] = k.ID
		sum.KeysAdded++
	}

	// --- Серверы: сначала без jump-хостов, чтобы не зависеть от порядка ---
	restart := map[string]bool{} // хосты, чьи туннели нужно переподключить
	imported := map[string]db.Host{}
	for _, h := range p.Hosts {
		h.Name = strings.TrimSpace(h.Name)
		if h.ID == "" || sshsvc.ValidateHostname(h.Hostname) != nil ||
			sshsvc.ValidateUsername(h.Username) != nil || sshsvc.ValidatePort(h.Port) != nil {
			sum.Skipped++
			continue
		}
		if h.Name == "" {
			h.Name = h.Hostname
		}
		switch h.EnvTag {
		case "prod", "stage", "dev", "infra":
		default:
			h.EnvTag = "dev"
		}
		if !tabColorRe.MatchString(h.TabColor) {
			h.TabColor = "#2D7D9A"
		}
		if h.KeyID != nil {
			if id, ok := keyIDMap[*h.KeyID]; ok {
				h.KeyID = &id
			} else if _, err := a.store.Keys.GetByID(a.ctx, *h.KeyID); err != nil {
				h.KeyID = nil
			}
		}
		jump := h.JumpHostID
		h.JumpHostID = nil

		prev, err := a.store.Hosts.GetByID(a.ctx, h.ID)
		if err == nil {
			// Сохраняем существующий jump-хост до второго прохода.
			h.JumpHostID = prev.JumpHostID
			sum.HostsUpdated++
			if connectionChanged(prev, &h) {
				restart[h.ID] = true
			}
		} else {
			sum.HostsAdded++
		}
		if err := a.store.Hosts.Upsert(a.ctx, &h); err != nil {
			return sum, fmt.Errorf("сервер %q: %w", h.Name, err)
		}
		h.JumpHostID = jump
		imported[h.ID] = h
	}
	for id, h := range imported {
		if h.JumpHostID == nil || *h.JumpHostID == id {
			continue
		}
		jump, err := a.store.Hosts.GetByID(a.ctx, *h.JumpHostID)
		if err != nil || (jump.JumpHostID != nil && *jump.JumpHostID == id) {
			continue
		}
		cur, err := a.store.Hosts.GetByID(a.ctx, id)
		if err != nil {
			continue
		}
		cur.JumpHostID = h.JumpHostID
		if err := a.store.Hosts.Upsert(a.ctx, cur); err != nil {
			return sum, fmt.Errorf("jump-хост для %q: %w", h.Name, err)
		}
	}

	// --- Туннели ---
	restartTunnels := map[string]string{} // tunnelID → hostID
	for _, t := range p.Tunnels {
		t.Name = strings.TrimSpace(t.Name)
		if t.ID == "" || t.Name == "" || sshsvc.ValidateTunnelAddr(t.LocalAddr) != nil {
			sum.Skipped++
			continue
		}
		switch t.Direction {
		case "L", "R":
			if sshsvc.ValidateTunnelAddr(t.RemoteAddr) != nil {
				sum.Skipped++
				continue
			}
		case "D":
			t.RemoteAddr = ""
		default:
			sum.Skipped++
			continue
		}
		if existing, err := a.store.Tunnels.GetByID(a.ctx, t.ID); err == nil {
			t.HostID = existing.HostID // Upsert не меняет host_id
			sum.TunnelsUpdated++
			restartTunnels[t.ID] = t.HostID
		} else if _, err := a.store.Hosts.GetByID(a.ctx, t.HostID); err != nil {
			sum.Skipped++
			continue
		} else {
			sum.TunnelsAdded++
		}
		if err := a.store.Tunnels.Upsert(a.ctx, &t); err != nil {
			return sum, fmt.Errorf("туннель %q: %w", t.Name, err)
		}
		if t.RDPUsername != "" {
			pw, hasPw := p.RDPPasswords[t.ID]
			if err := a.SaveRDPCredentials(RDPCredentialsInput{
				TunnelID: t.ID, Username: t.RDPUsername, Domain: t.RDPDomain, Password: pw, KeepPassword: !hasPw,
			}); err != nil {
				return sum, fmt.Errorf("RDP туннеля %q: %w", t.Name, err)
			}
		}
	}

	if a.tunnels != nil {
		for hostID := range restart {
			if list, err := a.store.Tunnels.ListByHost(a.ctx, hostID); err == nil {
				for _, t := range list {
					restartTunnels[t.ID] = hostID
				}
			}
		}
		for tunnelID, hostID := range restartTunnels {
			if !a.tunnels.IsRunning(tunnelID) {
				continue
			}
			a.tunnels.Stop(tunnelID)
			if err := a.StartTunnel(tunnelID, hostID); err != nil {
				runtime.EventsEmit(a.ctx, "tunnel:status", sshsvc.TunnelStatus{
					TunnelID: tunnelID, State: sshsvc.StateError, LastError: err.Error(),
				})
			}
		}
	}
	return sum, nil
}

func (a *App) importKey(bk backupKey, usedNames map[string]bool, makeDefault bool) (*db.SSHKey, error) {
	privBytes := []byte(bk.PrivateKeyPEM)
	defer crypto.ZeroBytes(privBytes)
	privEnc, err := a.vault.Encrypt(privBytes)
	if err != nil {
		return nil, fmt.Errorf("encrypt private key: %w", err)
	}
	var passEnc []byte
	if bk.Passphrase != "" {
		passBytes := []byte(bk.Passphrase)
		defer crypto.ZeroBytes(passBytes)
		if passEnc, err = a.vault.Encrypt(passBytes); err != nil {
			return nil, fmt.Errorf("encrypt passphrase: %w", err)
		}
	}

	name := strings.TrimSpace(bk.Name)
	if name == "" {
		name = "imported"
	}
	for i := 2; usedNames[name]; i++ {
		name = fmt.Sprintf("%s (%d)", strings.TrimSpace(bk.Name), i)
	}
	usedNames[name] = true

	id := bk.ID
	if _, err := a.store.Keys.GetByID(a.ctx, id); id == "" || err == nil {
		id = uuid.NewString()
	}
	k := &db.SSHKey{
		ID: id, Name: name, KeyType: bk.KeyType, Fingerprint: bk.Fingerprint, PublicKey: bk.PublicKey,
		PrivateKeyEnc: privEnc, PassphraseEnc: passEnc, IsDefault: makeDefault,
	}
	if err := a.store.Keys.Create(a.ctx, k); err != nil {
		return nil, err
	}
	if err := a.writePubKey(k); err != nil {
		a.warnf("write pubkey %s: %v", k.Name, err)
	}
	return k, nil
}
