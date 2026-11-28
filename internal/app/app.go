package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-launcher/internal/cli"
	"ssh-launcher/internal/crypto"
	"ssh-launcher/internal/db"
	"ssh-launcher/internal/ipc"
	sshsvc "ssh-launcher/internal/ssh"
	"ssh-launcher/internal/ssh/agent"
	"ssh-launcher/internal/terminal"
)

type App struct {
	ctx       context.Context
	cancelBg  context.CancelFunc
	store     *db.Store
	vault     *crypto.Vault
	tunnels   *sshsvc.TunnelManager
	monitor   *sshsvc.ReachabilityMonitor
	ipcServer *ipc.Server
	launcher  terminal.Launcher
	pubKeyDir string

	sessionsMu sync.RWMutex
	sessions   map[string]string

	initErr error

	quitting atomic.Bool
	uiReady  atomic.Bool

	clipboard  clipboardBackend
	secretClip secretClipboard

	upd updateState

	bootstrapMu     sync.Mutex
	bootstrapCancel context.CancelFunc
	hostKeyCh       chan bool
}

var tabColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

var errNotReady = errors.New("application is not initialised")

func New() *App {
	return &App{
		sessions: make(map[string]string),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.clipboard = newPlatformClipboard(ctx)
	go a.cleanupUpdates()
	bgCtx, cancel := context.WithCancel(context.Background())
	a.cancelBg = cancel

	fail := func(format string, args ...any) {
		a.initErr = fmt.Errorf(format, args...)
		runtime.LogError(ctx, a.initErr.Error())
	}

	dbPath, err := db.DefaultDBPath()
	if err != nil {
		fail("resolve db path: %v", err)
		return
	}

	vault, err := crypto.NewVault()
	if err != nil {
		fail("init vault (system keyring): %v", err)
		return
	}

	store, err := db.Open(dbPath)
	if err != nil {
		fail("open sqlite: %v", err)
		return
	}
	a.store = store
	a.vault = vault
	a.pubKeyDir = filepath.Join(filepath.Dir(dbPath), db.PubKeyDirName)
	a.syncPubKeys()

	if l, err := terminal.DefaultLauncher(); err == nil {
		a.launcher = l
	}

	a.tunnels = sshsvc.NewTunnelManager(func(status sshsvc.TunnelStatus) {
		runtime.EventsEmit(a.ctx, "tunnel:status", status)
	})

	a.monitor = sshsvc.NewReachabilityMonitor(sshsvc.MonitorConfig{
		Interval:    45 * time.Second,
		DialTimeout: 2 * time.Second,
		FetchTargets: func(ctx context.Context) ([]sshsvc.HostTarget, error) {
			hosts, err := a.store.Hosts.List(ctx)
			if err != nil {
				return nil, err
			}
			targets := make([]sshsvc.HostTarget, 0, len(hosts))
			for _, h := range hosts {
				targets = append(targets, sshsvc.HostTarget{
					HostID:   h.ID,
					Hostname: h.Hostname,
					Port:     h.Port,
				})
			}
			return targets, nil
		},
		OnUpdate: func(res sshsvc.PingResult) {
			runtime.EventsEmit(a.ctx, "host:ping", res)
		},
	})
	go a.monitor.Start(bgCtx)

	srv, err := ipc.NewServer(a.handleIPCMessage)
	if err != nil {
		runtime.LogErrorf(ctx, "start ipc server: %v", err)
	} else {
		a.ipcServer = srv
		go func() { _ = srv.Start(bgCtx) }()
	}

	startGlobalHotkey(bgCtx, a.showAndFocusOmnibar)
	a.startTray()
	go a.startAutoTunnels()
}

func (a *App) DomReady(ctx context.Context) {
	if a.initErr != nil {
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "Crystal-Core SSH Manager",
			Message: "Не удалось запустить приложение:\n\n" + a.initErr.Error(),
		})
		runtime.Quit(ctx)
		return
	}
	a.uiReady.Store(true)
	a.emitAgentStatus()
}

// emitAgentStatus сообщает UI, доступен ли агент и сколько ключей сейчас
// реально загружено в него (а не сколько лежит в хранилище).
func (a *App) emitAgentStatus() {
	if a.ctx == nil || !a.uiReady.Load() {
		return
	}
	keyCount := 0
	if loaded, err := agent.ListFingerprints(); err == nil {
		keyCount = len(loaded)
	}
	runtime.EventsEmit(a.ctx, "agent:status", map[string]any{
		"available": agent.IsAvailable(),
		"keys":      keyCount,
	})
}

func (a *App) ready() error {
	if a.store == nil || a.vault == nil {
		if a.initErr != nil {
			return a.initErr
		}
		return errNotReady
	}
	return nil
}

// decryptKey расшифровывает приватный ключ и пароль к нему; вызывающий
// обязан затереть оба буфера.
func (a *App) decryptKey(k *db.SSHKey) (privPEM, passphrase []byte, err error) {
	privPEM, err = a.vault.Decrypt(k.PrivateKeyEnc)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt private key: %w", err)
	}
	if len(k.PassphraseEnc) > 0 {
		passphrase, err = a.vault.Decrypt(k.PassphraseEnc)
		if err != nil {
			crypto.ZeroBytes(privPEM)
			return nil, nil, fmt.Errorf("decrypt passphrase: %w", err)
		}
	}
	return privPEM, passphrase, nil
}

// hostWithKey загружает хост и его ключ в расшифрованном виде.
func (a *App) hostWithKey(hostID string) (*db.Host, []byte, []byte, error) {
	if err := a.ready(); err != nil {
		return nil, nil, nil, err
	}
	host, err := a.store.Hosts.GetByID(a.ctx, hostID)
	if err != nil {
		return nil, nil, nil, err
	}
	if host.KeyID == nil || *host.KeyID == "" {
		return nil, nil, nil, errors.New("host has no assigned ssh key")
	}
	k, err := a.store.Keys.GetByID(a.ctx, *host.KeyID)
	if err != nil {
		return nil, nil, nil, err
	}
	privPEM, passphrase, err := a.decryptKey(k)
	if err != nil {
		return nil, nil, nil, err
	}
	return host, privPEM, passphrase, nil
}

// BeforeClose: при наличии трея закрытие окна (крестик, Alt+F4) только
// прячет его, а UI показывает подсказку, что выйти можно из меню трея.
// Настоящий выход идёт через ExitApp, который выставляет quitting.
func (a *App) BeforeClose(ctx context.Context) (prevent bool) {
	if a.quitting.Load() || !trayAvailable {
		// Пароль не должен пережить приложение в буфере обмена.
		_ = a.clearSecret(0)
		return false
	}
	runtime.EventsEmit(ctx, "ui:close_requested", nil)
	return true
}

// TrayAvailable сообщает UI, есть ли иконка в трее (на macOS её нет).
func (a *App) TrayAvailable() bool { return trayAvailable }

// HideToTray прячет окно; приложение продолжает работать в трее.
// Ключи при этом выгружаются из ssh-agent: туннели работают на ключах в памяти,
// а внешние программы больше не могут войти на серверы без повторного подключения.
func (a *App) HideToTray() {
	if trayAvailable {
		if n, err := a.LockKeys(); err != nil {
			a.warnf("lock keys on hide: %v", err)
		} else if n > 0 {
			a.warnf("locked %d ssh-agent key(s) on hide", n)
		}
		runtime.WindowHide(a.ctx)
	}
}

// ExitApp полностью завершает приложение (туннели останавливаются).
func (a *App) ExitApp() {
	a.quitting.Store(true)
	_ = a.clearSecret(0)
	runtime.Quit(a.ctx)
}

func (a *App) Shutdown(ctx context.Context) {
	a.stopTray()
	a.CancelBootstrap()
	if a.tunnels != nil {
		a.tunnels.StopAll()
	}
	a.uiReady.Store(false)
	// Не оставляем ключи в ssh-agent после выхода: любой процесс пользователя
	// иначе сможет пользоваться ими бесконечно.
	if a.store != nil {
		if _, err := a.LockKeys(); err != nil {
			a.warnf("lock keys on shutdown: %v", err)
		}
	}
	if a.cancelBg != nil {
		a.cancelBg()
	}
	if a.store != nil {
		_ = a.store.Close()
	}
}

func (a *App) handleIPCMessage(msg ipc.Message) {
	if msg.Type != ipc.MsgFocusOmnibar && msg.SessionID == "" {
		return
	}
	switch msg.Type {
	case ipc.MsgFocusOmnibar:
		a.showAndFocusOmnibar()
	case ipc.MsgSessionStarted:
		a.sessionsMu.Lock()
		a.sessions[msg.SessionID] = msg.HostID
		a.sessionsMu.Unlock()
		_ = a.store.Hosts.TouchLastConnected(context.Background(), msg.HostID)
		runtime.EventsEmit(a.ctx, "session:update", msg)
	case ipc.MsgSessionEnded:
		a.sessionsMu.Lock()
		delete(a.sessions, msg.SessionID)
		a.sessionsMu.Unlock()
		runtime.EventsEmit(a.ctx, "session:update", msg)
	}
}

// --- Hosts ---

func (a *App) ListHosts() ([]db.Host, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.store.Hosts.List(a.ctx)
}

func (a *App) SaveHost(h db.Host) (*db.Host, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	h.Name = strings.TrimSpace(h.Name)
	h.Hostname = strings.TrimSpace(h.Hostname)
	h.Username = strings.TrimSpace(h.Username)
	if h.Port == 0 {
		h.Port = 22
	}
	if err := sshsvc.ValidateHostname(h.Hostname); err != nil {
		return nil, err
	}
	if err := sshsvc.ValidateUsername(h.Username); err != nil {
		return nil, err
	}
	if err := sshsvc.ValidatePort(h.Port); err != nil {
		return nil, err
	}
	if h.Name == "" {
		h.Name = h.Hostname
	}
	switch h.EnvTag {
	case "", "prod", "stage", "dev", "infra":
	default:
		return nil, fmt.Errorf("invalid env tag %q", h.EnvTag)
	}
	if !tabColorRe.MatchString(h.TabColor) {
		h.TabColor = "#2D7D9A"
	}
	if h.KeyID != nil && *h.KeyID == "" {
		h.KeyID = nil
	}
	if h.JumpHostID != nil && (*h.JumpHostID == "" || *h.JumpHostID == h.ID) {
		h.JumpHostID = nil
	}
	if h.KeyID != nil {
		if _, err := a.store.Keys.GetByID(a.ctx, *h.KeyID); err != nil {
			return nil, fmt.Errorf("SSH-ключ не найден: %w", err)
		}
	}
	if h.JumpHostID != nil {
		jump, err := a.store.Hosts.GetByID(a.ctx, *h.JumpHostID)
		if err != nil {
			return nil, fmt.Errorf("jump-хост не найден: %w", err)
		}
		if h.ID != "" && jump.JumpHostID != nil && *jump.JumpHostID == h.ID {
			return nil, fmt.Errorf("циклическая цепочка: %q уже подключается через этот сервер", jump.Name)
		}
	}

	var prev *db.Host
	if h.ID != "" {
		existing, err := a.store.Hosts.GetByID(a.ctx, h.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("load host: %w", err)
		}
		prev = existing
	}
	if err := a.store.Hosts.Upsert(a.ctx, &h); err != nil {
		return nil, err
	}
	a.RefreshHostsStatus()
	if prev != nil && connectionChanged(prev, &h) {
		a.restartHostTunnels(h.ID)
	}
	return a.store.Hosts.GetByID(a.ctx, h.ID)
}

// connectionChanged: изменились параметры, с которыми туннели держат SSH-сессию.
func connectionChanged(a, b *db.Host) bool {
	return a.Hostname != b.Hostname || a.Port != b.Port || a.Username != b.Username ||
		ptrValue(a.KeyID) != ptrValue(b.KeyID)
}

func ptrValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// restartHostTunnels переподключает работающие туннели хоста с новыми параметрами.
func (a *App) restartHostTunnels(hostID string) {
	if a.tunnels == nil {
		return
	}
	tunnels, err := a.store.Tunnels.ListByHost(a.ctx, hostID)
	if err != nil {
		runtime.LogErrorf(a.ctx, "list tunnels for restart: %v", err)
		return
	}
	for _, t := range tunnels {
		if !a.tunnels.IsRunning(t.ID) {
			continue
		}
		a.tunnels.Stop(t.ID)
		if err := a.StartTunnel(t.ID, hostID); err != nil {
			runtime.EventsEmit(a.ctx, "tunnel:status", sshsvc.TunnelStatus{
				TunnelID: t.ID, State: sshsvc.StateError, LastError: err.Error(),
			})
		}
	}
}

func (a *App) DeleteHost(hostID string) error {
	if err := a.ready(); err != nil {
		return err
	}
	// Туннели удаляются каскадно в БД, но работающие нужно остановить явно.
	if tunnels, err := a.store.Tunnels.ListByHost(a.ctx, hostID); err == nil {
		for _, t := range tunnels {
			a.StopTunnel(t.ID)
		}
	}
	return a.store.Hosts.Delete(a.ctx, hostID)
}

func (a *App) RefreshHostsStatus() {
	if a.monitor != nil {
		a.monitor.RefreshNow()
	}
}

func (a *App) GetHostStatuses() map[string]sshsvc.PingResult {
	if a.monitor == nil {
		return map[string]sshsvc.PingResult{}
	}
	return a.monitor.Snapshot()
}

func (a *App) ConnectHosts(hostIDs []string, mode string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if a.launcher == nil {
		return terminal.ErrNoTerminalFound
	}
	if len(hostIDs) == 0 {
		return nil
	}

	exePath, err := resolveCLIExecutable()
	if err != nil {
		return err
	}

	specs := make([]terminal.SessionSpec, 0, len(hostIDs))
	for _, id := range hostIDs {
		host, err := a.store.Hosts.GetByID(a.ctx, id)
		if err != nil {
			return fmt.Errorf("load host %s: %w", id, err)
		}
		if host.KeyID != nil && *host.KeyID != "" {
			if err := a.ensureKeyReady(*host.KeyID); err != nil {
				return fmt.Errorf("prepare key for %s: %w", host.Name, err)
			}
		}
		specs = append(specs, terminal.SessionSpec{
			HostID:    host.ID,
			SessionID: uuid.NewString(),
			Title:     host.Name,
			TabColor:  host.TabColor,
		})
	}

	launchMode := terminal.ModeTabs
	if mode == string(terminal.ModeSplit) {
		launchMode = terminal.ModeSplit
	}
	return a.launcher.Launch(exePath, specs, launchMode)
}

// RunHostScript открывает вкладку терминала и выполняет на сервере один из
// предопределённых скриптов (сейчас только "vps-setup"). Терминал нужен, потому
// что установщик интерактивный: sudo спрашивает пароль, есть диалоги.
func (a *App) RunHostScript(hostID, script string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if !cli.IsScript(script) {
		return fmt.Errorf("неизвестный скрипт %q", script)
	}
	if a.launcher == nil {
		return terminal.ErrNoTerminalFound
	}
	exePath, err := resolveCLIExecutable()
	if err != nil {
		return err
	}
	host, err := a.store.Hosts.GetByID(a.ctx, hostID)
	if err != nil {
		return fmt.Errorf("load host %s: %w", hostID, err)
	}
	if host.KeyID != nil && *host.KeyID != "" {
		if err := a.ensureKeyReady(*host.KeyID); err != nil {
			return fmt.Errorf("prepare key for %s: %w", host.Name, err)
		}
	}
	return a.launcher.Launch(exePath, []terminal.SessionSpec{{
		HostID:    host.ID,
		SessionID: uuid.NewString(),
		Title:     host.Name + " · " + script,
		TabColor:  host.TabColor,
		Script:    script,
	}}, terminal.ModeTabs)
}

// --- Keys ---

type AddKeyInput struct {
	Name          string `json:"name"`
	KeyType       string `json:"key_type"`
	Fingerprint   string `json:"fingerprint"`
	PublicKey     string `json:"public_key"`
	PrivateKeyPEM string `json:"private_key_pem"`
	Passphrase    string `json:"passphrase"`
	IsDefault     bool   `json:"is_default"`
}

func (a *App) ListKeys() ([]db.SSHKey, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.store.Keys.List(a.ctx)
}

func (a *App) AddKey(in AddKeyInput) (*db.SSHKey, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	privBytes := []byte(in.PrivateKeyPEM)
	defer crypto.ZeroBytes(privBytes)

	privEnc, err := a.vault.Encrypt(privBytes)
	if err != nil {
		return nil, fmt.Errorf("encrypt private key: %w", err)
	}

	var passEnc []byte
	if in.Passphrase != "" {
		passBytes := []byte(in.Passphrase)
		defer crypto.ZeroBytes(passBytes)
		passEnc, err = a.vault.Encrypt(passBytes)
		if err != nil {
			return nil, fmt.Errorf("encrypt passphrase: %w", err)
		}
	}

	k := &db.SSHKey{
		Name:          in.Name,
		KeyType:       in.KeyType,
		Fingerprint:   in.Fingerprint,
		PublicKey:     in.PublicKey,
		PrivateKeyEnc: privEnc,
		PassphraseEnc: passEnc,
		IsDefault:     in.IsDefault,
	}
	if err := a.store.Keys.Create(a.ctx, k); err != nil {
		return nil, err
	}
	if err := a.ensureKeyReady(k.ID); err != nil {
		// Ключ сохранён; агент может быть не запущен — загрузим при подключении.
		a.warnf("load key %s into ssh-agent: %v", k.Name, err)
	}
	return k, nil
}

func (a *App) GenerateKey(name, passphrase string, isDefault bool) (*db.SSHKey, error) {
	passBytes := []byte(passphrase)
	defer crypto.ZeroBytes(passBytes)

	gen, err := crypto.GenerateEd25519(name, passBytes)
	if err != nil {
		return nil, err
	}
	defer crypto.ZeroBytes(gen.PrivateKeyPEM)

	return a.AddKey(AddKeyInput{
		Name:          name,
		KeyType:       gen.KeyType,
		Fingerprint:   gen.Fingerprint,
		PublicKey:     gen.PublicKey,
		PrivateKeyPEM: string(gen.PrivateKeyPEM),
		Passphrase:    passphrase,
		IsDefault:     isDefault,
	})
}

func (a *App) ImportKey(name, privateKeyPEM, passphrase string, isDefault bool) (*db.SSHKey, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	privBytes := []byte(privateKeyPEM)
	passBytes := []byte(passphrase)
	defer crypto.ZeroBytes(privBytes)
	defer crypto.ZeroBytes(passBytes)

	meta, err := crypto.ParseKeyMetadata(privBytes, passBytes)
	if err != nil {
		return nil, err
	}
	return a.AddKey(AddKeyInput{
		Name:          name,
		KeyType:       meta.KeyType,
		Fingerprint:   meta.Fingerprint,
		PublicKey:     meta.PublicKey,
		PrivateKeyPEM: privateKeyPEM,
		Passphrase:    passphrase,
		IsDefault:     isDefault,
	})
}

func (a *App) SetDefaultKey(keyID string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.store.Keys.SetDefault(a.ctx, keyID)
}

func (a *App) DeleteKey(keyID string) error {
	if err := a.ready(); err != nil {
		return err
	}
	k, err := a.store.Keys.GetByID(a.ctx, keyID)
	if err != nil {
		return err
	}
	if err := a.store.Keys.Delete(a.ctx, keyID); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(a.pubKeyDir, keyID+".pub"))
	if err := agent.RemoveKey(k.PublicKey); err != nil {
		a.warnf("remove key %s from ssh-agent: %v", k.Name, err)
	}
	a.emitAgentStatus()
	return nil
}

// AgentKeyCount возвращает число ключей, реально загруженных в ssh-agent.
func (a *App) AgentKeyCount() (int, error) {
	loaded, err := agent.ListFingerprints()
	if err != nil {
		return 0, err
	}
	return len(loaded), nil
}

// LockKeys выгружает из системного ssh-agent все ключи, сохранённые в Key Vault.
// Пока ключ находится в агенте, любая программа или агент, запущенные от того же
// пользователя, могут использовать его для входа на серверы, поэтому ключи
// выгружаются при сворачивании в трей, выходе и по кнопке «Заблокировать».
func (a *App) LockKeys() (int, error) {
	if a.store == nil {
		return 0, errNotReady
	}

	keys, err := a.store.Keys.List(context.Background())
	if err != nil {
		return 0, err
	}
	loaded, err := agent.ListFingerprints()
	if err != nil {
		return 0, err
	}

	removed := 0
	var errs []error
	for i := range keys {
		k := &keys[i]
		if !loaded[k.Fingerprint] {
			continue
		}
		if err := agent.RemoveKey(k.PublicKey); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", k.Name, err))
			continue
		}
		removed++
	}
	a.emitAgentStatus()
	return removed, errors.Join(errs...)
}

// writePubKey кладёт публичную часть ключа в pubkeys/<id>.pub: на неё
// ссылаются IdentityFile в ~/.ssh/config и аргумент -i системного ssh.
func (a *App) writePubKey(k *db.SSHKey) error {
	if a.pubKeyDir == "" {
		return nil
	}
	pubPath := filepath.Join(a.pubKeyDir, k.ID+".pub")
	if err := os.WriteFile(pubPath, []byte(strings.TrimSpace(k.PublicKey)+"\n"), 0600); err != nil {
		return fmt.Errorf("write pubkey cache: %w", err)
	}
	return nil
}

// syncPubKeys восстанавливает .pub-файлы всех ключей (например, после
// импорта бэкапа или для ключей, которыми ещё ни разу не подключались).
func (a *App) syncPubKeys() {
	keys, err := a.store.Keys.List(a.ctx)
	if err != nil {
		a.warnf("list keys for pubkey sync: %v", err)
		return
	}
	for i := range keys {
		if err := a.writePubKey(&keys[i]); err != nil {
			a.warnf("%v", err)
		}
	}
}

func (a *App) ensureKeyReady(keyID string) error {
	k, err := a.store.Keys.GetByID(a.ctx, keyID)
	if err != nil {
		return err
	}
	if err := a.writePubKey(k); err != nil {
		return err
	}
	if loaded, err := agent.ListFingerprints(); err == nil && loaded[k.Fingerprint] {
		a.emitAgentStatus()
		return nil
	}
	privPEM, passphrase, err := a.decryptKey(k)
	if err != nil {
		return err
	}
	if err := agent.AddKey(privPEM, passphrase, k.Name); err != nil {
		return err
	}
	a.emitAgentStatus()
	return nil
}

// --- Bootstrap ---

type BootstrapInput struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	KeyID    string `json:"key_id"`
}

func (a *App) RunBootstrap(in BootstrapInput) (*sshsvc.BootstrapResult, error) {
	password := []byte(in.Password)
	defer crypto.ZeroBytes(password)

	if err := a.ready(); err != nil {
		return nil, err
	}

	a.bootstrapMu.Lock()
	if a.bootstrapCancel != nil {
		a.bootstrapMu.Unlock()
		return nil, errors.New("bootstrap is already running")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.bootstrapCancel = cancel
	a.bootstrapMu.Unlock()
	defer func() {
		cancel()
		a.bootstrapMu.Lock()
		a.bootstrapCancel = nil
		a.hostKeyCh = nil
		a.bootstrapMu.Unlock()
	}()

	k, err := a.store.Keys.GetByID(ctx, in.KeyID)
	if err != nil {
		return nil, fmt.Errorf("load key: %w", err)
	}
	privPEM, passphrase, err := a.decryptKey(k)
	if err != nil {
		return nil, err
	}

	req := sshsvc.BootstrapRequest{
		Hostname:      strings.TrimSpace(in.Hostname),
		Port:          in.Port,
		Username:      strings.TrimSpace(in.Username),
		Password:      password,
		PublicKey:     k.PublicKey,
		PrivateKeyPEM: privPEM,
		Passphrase:    passphrase,
		ConfirmHostKey: func(ctx context.Context, info sshsvc.HostKeyInfo) (bool, error) {
			ch := make(chan bool, 1)
			a.bootstrapMu.Lock()
			a.hostKeyCh = ch
			a.bootstrapMu.Unlock()

			runtime.EventsEmit(a.ctx, "bootstrap:confirm_host_key", info)
			timer := time.NewTimer(sshsvc.HostKeyConfirmTimeout)
			defer timer.Stop()
			select {
			case approved := <-ch:
				return approved, nil
			case <-timer.C:
				return false, errors.New("истекло время ожидания подтверждения HostKey")
			case <-ctx.Done():
				return false, ctx.Err()
			}
		},
		OnProgress: func(step sshsvc.BootstrapStep, message string) {
			runtime.EventsEmit(a.ctx, "bootstrap:progress", map[string]any{
				"step":    step,
				"message": message,
			})
		},
	}
	res, err := sshsvc.RunBootstrap(ctx, req)
	if err == nil {
		if loadErr := a.ensureKeyReady(k.ID); loadErr != nil {
			a.warnf("load key into ssh-agent: %v", loadErr)
		}
	}
	return res, err
}

// CancelBootstrap прерывает текущий Bootstrap (закрытие модального окна).
func (a *App) CancelBootstrap() {
	a.bootstrapMu.Lock()
	cancel := a.bootstrapCancel
	a.bootstrapMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) ResolveHostKey(approved bool) {
	a.bootstrapMu.Lock()
	ch := a.hostKeyCh
	a.bootstrapMu.Unlock()
	if ch != nil {
		select {
		case ch <- approved:
		default:
		}
	}
}

// --- Tunnels ---

func (a *App) ListTunnels(hostID string) ([]db.Tunnel, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.store.Tunnels.ListByHost(a.ctx, hostID)
}

func (a *App) ListAllTunnels() ([]db.Tunnel, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.store.Tunnels.ListAll(a.ctx)
}

func (a *App) DeleteTunnel(tunnelID string) error {
	if err := a.ready(); err != nil {
		return err
	}
	a.StopTunnel(tunnelID)
	return a.store.Tunnels.Delete(a.ctx, tunnelID)
}

// startAutoTunnels поднимает туннели с auto_start после запуска приложения.
func (a *App) startAutoTunnels() {
	tunnels, err := a.store.Tunnels.ListAll(a.ctx)
	if err != nil {
		runtime.LogErrorf(a.ctx, "list tunnels for auto-start: %v", err)
		return
	}
	for _, t := range tunnels {
		if !t.AutoStart {
			continue
		}
		if err := a.StartTunnel(t.ID, t.HostID); err != nil {
			a.warnf("auto-start tunnel %s: %v", t.Name, err)
			runtime.EventsEmit(a.ctx, "tunnel:status", sshsvc.TunnelStatus{
				TunnelID: t.ID, State: sshsvc.StateError, LastError: err.Error(),
			})
		}
	}
}

func (a *App) SaveTunnel(t db.Tunnel) (*db.Tunnel, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	t.Name = strings.TrimSpace(t.Name)
	t.LocalAddr = strings.TrimSpace(t.LocalAddr)
	t.RemoteAddr = strings.TrimSpace(t.RemoteAddr)
	if t.Name == "" {
		return nil, errors.New("tunnel name is required")
	}
	if err := sshsvc.ValidateTunnelAddr(t.LocalAddr); err != nil {
		return nil, fmt.Errorf("local: %w", err)
	}
	switch t.Direction {
	case "L", "R":
		if err := sshsvc.ValidateTunnelAddr(t.RemoteAddr); err != nil {
			return nil, fmt.Errorf("remote: %w", err)
		}
	case "D":
		t.RemoteAddr = ""
	default:
		return nil, fmt.Errorf("invalid tunnel direction %q", t.Direction)
	}

	if t.ID != "" {
		// host_id существующего туннеля не меняется (Upsert его не обновляет).
		existing, err := a.store.Tunnels.GetByID(a.ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("load tunnel: %w", err)
		}
		t.HostID = existing.HostID
	} else if _, err := a.store.Hosts.GetByID(a.ctx, t.HostID); err != nil {
		return nil, fmt.Errorf("load host: %w", err)
	}

	// Изменённый работающий туннель перезапускаем с новыми параметрами.
	wasRunning := t.ID != "" && a.tunnels.IsRunning(t.ID)
	if wasRunning {
		a.tunnels.Stop(t.ID)
	}
	if err := a.store.Tunnels.Upsert(a.ctx, &t); err != nil {
		return nil, err
	}
	if wasRunning {
		if err := a.StartTunnel(t.ID, t.HostID); err != nil {
			return &t, fmt.Errorf("saved, but restart failed: %w", err)
		}
	}
	return &t, nil
}

func (a *App) GetTunnelStatuses() []sshsvc.TunnelStatus {
	if a.tunnels == nil {
		return nil
	}
	return a.tunnels.Statuses()
}

func (a *App) StartTunnel(tunnelID, hostID string) error {
	host, privPEM, passphrase, err := a.hostWithKey(hostID)
	if err != nil {
		return err
	}
	tunnels, err := a.store.Tunnels.ListByHost(a.ctx, hostID)
	if err != nil {
		crypto.ZeroBytes(privPEM)
		crypto.ZeroBytes(passphrase)
		return err
	}
	var target *db.Tunnel
	for i := range tunnels {
		if tunnels[i].ID == tunnelID {
			target = &tunnels[i]
			break
		}
	}
	if target == nil {
		crypto.ZeroBytes(privPEM)
		crypto.ZeroBytes(passphrase)
		return fmt.Errorf("tunnel %s not found", tunnelID)
	}

	// TunnelManager.Start затирает ключевой материал сам.
	return a.tunnels.Start(sshsvc.TunnelConfig{
		ID:            target.ID,
		Direction:     sshsvc.TunnelDirection(target.Direction),
		LocalAddr:     target.LocalAddr,
		RemoteAddr:    target.RemoteAddr,
		Hostname:      host.Hostname,
		Port:          host.Port,
		Username:      host.Username,
		PrivateKeyPEM: privPEM,
		Passphrase:    passphrase,
	})
}

func (a *App) StopTunnel(tunnelID string) {
	if a.tunnels != nil {
		a.tunnels.Stop(tunnelID)
	}
}

// --- SFTP & Config Sync ---

func (a *App) UploadFileToHost(hostID, localPath, remoteDir string) (string, error) {
	host, privPEM, passphrase, err := a.hostWithKey(hostID)
	if err != nil {
		return "", err
	}
	// UploadFile затирает ключевой материал сам.
	return sshsvc.UploadFile(a.ctx, sshsvc.UploadRequest{
		HostID:        host.ID,
		Hostname:      host.Hostname,
		Port:          host.Port,
		Username:      host.Username,
		PrivateKeyPEM: privPEM,
		Passphrase:    passphrase,
		LocalPath:     localPath,
		RemoteDir:     remoteDir,
		OnProgress: func(p sshsvc.UploadProgress) {
			runtime.EventsEmit(a.ctx, "sftp:progress", p)
		},
	})
}

func (a *App) SyncSSHConfig() error {
	if err := a.ready(); err != nil {
		return err
	}
	hosts, err := a.store.Hosts.List(a.ctx)
	if err != nil {
		return err
	}
	var syncList []sshsvc.SyncHost
	readyKeys := map[string]bool{}
	for _, h := range hosts {
		if h.IsTemporary {
			continue
		}
		item := sshsvc.SyncHost{
			ID:       h.ID,
			Name:     h.Name,
			Hostname: h.Hostname,
			Port:     h.Port,
			Username: h.Username,
		}
		if h.KeyID != nil && *h.KeyID != "" {
			// IdentityFile указывает на .pub, а приватная часть берётся из
			// ssh-agent: создаём файл и загружаем ключ в агент заранее, иначе
			// VS Code / git не смогут войти, пока сервер не открыт из приложения.
			if !readyKeys[*h.KeyID] {
				if err := a.ensureKeyReady(*h.KeyID); err != nil {
					a.warnf("prepare key %s for ssh config: %v", *h.KeyID, err)
				}
				readyKeys[*h.KeyID] = true
			}
			item.PubKeyPath = filepath.Join(a.pubKeyDir, *h.KeyID+".pub")
		}
		if h.JumpHostID != nil {
			item.JumpHostID = *h.JumpHostID
		}
		syncList = append(syncList, item)
	}
	return sshsvc.ExportToSSHConfig("", syncList)
}

func (a *App) PreviewImportSSHConfig() ([]sshsvc.ImportedHost, error) {
	return sshsvc.ImportFromSSHConfig("")
}

func resolveCLIExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if goruntime.GOOS == "windows" {
		// GUI-бинарник собран с -H windowsgui и не имеет консоли, поэтому
		// внутри вкладки wt.exe нужна отдельная консольная обёртка.
		cliPath := filepath.Join(filepath.Dir(exe), "ssh-launcher-cli.exe")
		if _, err := os.Stat(cliPath); err != nil {
			return "", fmt.Errorf("ssh-launcher-cli.exe not found next to %s", filepath.Base(exe))
		}
		return cliPath, nil
	}
	return exe, nil
}
