//go:build windows

package app

import (
	"context"
	"syscall"
	"time"
	"unsafe"
)

var (
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	procGlobalAlloc                = kernel32.NewProc("GlobalAlloc")
	procGlobalFree                 = kernel32.NewProc("GlobalFree")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory              = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// Форматы, которыми Windows просят не сохранять данные в журнал буфера
// (Win+V), не синхронизировать их в облако и не отдавать мониторам буфера.
var secretClipboardFormats = []struct {
	name  string
	value uint32
}{
	{"ExcludeClipboardContentFromMonitorProcessing", 0},
	{"CanIncludeInClipboardHistory", 0},
	{"CanUploadToCloudClipboard", 0},
}

// winClipboard работает с буфером напрямую через WinAPI: Wails не умеет
// ставить флаги исключения из журнала. «Наш ли ещё буфер» проверяем по
// GetClipboardSequenceNumber, не читая содержимое.
type winClipboard struct{}

func newPlatformClipboard(context.Context) clipboardBackend { return winClipboard{} }

func openClipboard() error {
	// Буфер может быть ненадолго занят другим процессом.
	var err error
	for i := 0; i < 10; i++ {
		r, _, e := procOpenClipboard.Call(0)
		if r != 0 {
			return nil
		}
		err = e
		time.Sleep(20 * time.Millisecond)
	}
	return err
}

func globalCopy(src unsafe.Pointer, size uintptr) (uintptr, error) {
	h, _, err := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return 0, err
	}
	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return 0, err
	}
	procRtlMoveMemory.Call(p, uintptr(src), size)
	procGlobalUnlock.Call(h)
	return h, nil
}

// setData передаёт блок памяти буферу; после успеха им владеет система.
func setData(format uintptr, src unsafe.Pointer, size uintptr) error {
	h, err := globalCopy(src, size)
	if err != nil {
		return err
	}
	if r, _, e := procSetClipboardData.Call(format, h); r == 0 {
		procGlobalFree.Call(h)
		return e
	}
	return nil
}

func (winClipboard) SetSecret(text string) (any, error) {
	u, err := syscall.UTF16FromString(text)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range u {
			u[i] = 0
		}
	}()
	if err := writeSecret(u); err != nil {
		return nil, err
	}
	// Номер меняется при каждом изменении буфера; берём его после CloseClipboard.
	seq, _, _ := procGetClipboardSequenceNumber.Call()
	return uint32(seq), nil
}

func writeSecret(u []uint16) error {
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if r, _, e := procEmptyClipboard.Call(); r == 0 {
		return e
	}
	if err := setData(cfUnicodeText, unsafe.Pointer(&u[0]), uintptr(len(u)*2)); err != nil {
		return err
	}
	for _, f := range secretClipboardFormats {
		name, _ := syscall.UTF16PtrFromString(f.name)
		id, _, _ := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(name)))
		if id == 0 {
			continue
		}
		v := f.value
		_ = setData(id, unsafe.Pointer(&v), unsafe.Sizeof(v))
	}
	return nil
}

func (winClipboard) Owns(mark any) bool {
	want, ok := mark.(uint32)
	if !ok {
		return false
	}
	seq, _, _ := procGetClipboardSequenceNumber.Call()
	return uint32(seq) == want
}

func (winClipboard) Clear() error {
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if r, _, e := procEmptyClipboard.Call(); r == 0 {
		return e
	}
	return nil
}
