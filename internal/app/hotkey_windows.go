//go:build windows

package app

import (
	"context"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modNoRepeat = 0x4000
	vkS         = 0x53
	wmHotkey    = 0x0312
	wmQuit      = 0x0012
	hotkeyID    = 1
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procPostThreadMsgW   = user32.NewProc("PostThreadMessageW")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentThread = kernel32.NewProc("GetCurrentThreadId")
)

type winMsg struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

func startGlobalHotkey(ctx context.Context, onTrigger func()) {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		threadID, _, _ := procGetCurrentThread.Call()
		r, _, _ := procRegisterHotKey.Call(0, hotkeyID, modControl|modAlt|modNoRepeat, vkS)
		if r == 0 {
			return
		}
		defer procUnregisterHotKey.Call(0, hotkeyID)

		go func() {
			<-ctx.Done()
			procPostThreadMsgW.Call(threadID, wmQuit, 0, 0)
		}()

		var msg winMsg
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(ret) <= 0 {
				return
			}
			if msg.Message == wmHotkey && msg.WParam == hotkeyID && onTrigger != nil {
				onTrigger()
			}
		}
	}()
}
