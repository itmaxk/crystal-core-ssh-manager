//go:build !windows

package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// wailsClipboard — буфер через Wails (GTK на Linux, NSPasteboard на macOS).
// Сам секрет не храним: для проверки «буфер всё ещё наш» достаточно хеша.
type wailsClipboard struct{ ctx context.Context }

func newPlatformClipboard(ctx context.Context) clipboardBackend { return &wailsClipboard{ctx: ctx} }

func (c *wailsClipboard) SetSecret(text string) (any, error) {
	if err := runtime.ClipboardSetText(c.ctx, text); err != nil {
		return nil, err
	}
	return sha256.Sum256([]byte(text)), nil
}

func (c *wailsClipboard) Owns(mark any) bool {
	want, ok := mark.([32]byte)
	if !ok {
		return false
	}
	cur, err := runtime.ClipboardGetText(c.ctx)
	if err != nil {
		return true // не смогли прочитать — надёжнее стереть
	}
	got := sha256.Sum256([]byte(cur))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func (c *wailsClipboard) Clear() error { return runtime.ClipboardSetText(c.ctx, "") }
