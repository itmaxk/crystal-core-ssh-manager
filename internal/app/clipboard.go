package app

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-launcher/internal/crypto"
)

// secretClipboardTTL — сколько сохранённый пароль лежит в буфере обмена
// (переменная, чтобы тесты не ждали 15 секунд).
var secretClipboardTTL = 15 * time.Second

// clipboardBackend пишет секрет в системный буфер и позже стирает его, но
// только если пользователь за это время не скопировал туда что-то своё.
type clipboardBackend interface {
	// SetSecret кладёт текст в буфер и возвращает метку для Owns.
	SetSecret(text string) (mark any, err error)
	// Owns сообщает, лежит ли в буфере всё ещё то, что записал SetSecret.
	Owns(mark any) bool
	Clear() error
}

// SecretCopied — событие clipboard:secret для фронтенда.
type SecretCopied struct {
	Label   string `json:"label"`
	Seconds int    `json:"seconds"`
}

type secretClipboard struct {
	mu    sync.Mutex
	gen   uint64
	mark  any
	timer *time.Timer
}

// CopyRDPPassword копирует сохранённый пароль RDP туннеля в буфер на 15 секунд.
func (a *App) CopyRDPPassword(tunnelID string) (*SecretCopied, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	t, err := a.store.Tunnels.GetByID(a.ctx, tunnelID)
	if err != nil {
		return nil, fmt.Errorf("load tunnel: %w", err)
	}
	if !t.HasRDPPassword {
		return nil, errors.New("для этого туннеля пароль RDP не сохранён")
	}
	creds, err := a.rdpCredentials(t)
	if err != nil {
		return nil, err
	}
	defer crypto.ZeroBytes(creds.Password)
	return a.copySecret(creds.Password, "Пароль RDP «"+t.Name+"»")
}

// CopyKeyPassphrase копирует сохранённую парольную фразу ключа в буфер на 15 секунд.
func (a *App) CopyKeyPassphrase(keyID string) (*SecretCopied, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	k, err := a.store.Keys.GetByID(a.ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("load key: %w", err)
	}
	if len(k.PassphraseEnc) == 0 {
		return nil, errors.New("у ключа нет сохранённой парольной фразы")
	}
	pass, err := a.vault.Decrypt(k.PassphraseEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt passphrase: %w", err)
	}
	defer crypto.ZeroBytes(pass)
	if len(pass) == 0 {
		return nil, errors.New("у ключа нет сохранённой парольной фразы")
	}
	return a.copySecret(pass, "Парольная фраза ключа «"+k.Name+"»")
}

// ClearSecretClipboard стирает скопированный пароль досрочно (кнопка «Очистить»).
func (a *App) ClearSecretClipboard() error {
	return a.clearSecret(0)
}

func (a *App) copySecret(secret []byte, label string) (*SecretCopied, error) {
	if a.clipboard == nil {
		return nil, errors.New("буфер обмена недоступен")
	}
	c := &a.secretClip
	c.mu.Lock()
	defer c.mu.Unlock()

	mark, err := a.clipboard.SetSecret(string(secret))
	if err != nil {
		return nil, fmt.Errorf("не удалось скопировать в буфер обмена: %w", err)
	}
	c.gen++
	c.mark = mark
	if c.timer != nil {
		c.timer.Stop()
	}
	gen := c.gen
	c.timer = time.AfterFunc(secretClipboardTTL, func() { _ = a.clearSecret(gen) })
	return &SecretCopied{Label: label, Seconds: int((secretClipboardTTL + time.Second - 1) / time.Second)}, nil
}

// clearSecret стирает буфер, если в нём ещё лежит наш секрет. gen != 0 —
// вызов от таймера конкретного копирования: если после него скопировали
// другой пароль, ничего не делаем (у того свой таймер).
func (a *App) clearSecret(gen uint64) error {
	c := &a.secretClip
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mark == nil || (gen != 0 && gen != c.gen) {
		return nil
	}
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	mark := c.mark
	c.mark = nil
	defer a.emit("clipboard:cleared", nil)
	if !a.clipboard.Owns(mark) {
		return nil // пользователь уже скопировал что-то своё
	}
	return a.clipboard.Clear()
}

func (a *App) emit(event string, data any) {
	// Вне жизненного цикла Wails (тесты, завершение) EventsEmit вызвал бы log.Fatalf.
	if a.ctx == nil || a.ctx.Value("events") == nil || a.quitting.Load() {
		return
	}
	runtime.EventsEmit(a.ctx, event, data)
}
