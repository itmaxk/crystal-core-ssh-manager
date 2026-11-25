//go:build windows

package terminal

import (
	"fmt"
	"net"
	"os/exec"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	procCredWriteW = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
)

const (
	credTypeGeneric    = 1
	credPersistSession = 1
	// Сколько держать пароль в диспетчере учётных данных: mstsc читает его
	// при подключении, дольше хранить незачем.
	rdpCredLifetime = 90 * time.Second
)

// credentialW повторяет раскладку CREDENTIALW из wincred.h.
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// rdpTargets — имена, по которым mstsc ищет сохранённые данные (TERMSRV/<host>).
func rdpTargets(addr string) []string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return []string{"TERMSRV/" + host, "TERMSRV/" + addr}
}

func credWrite(target, user string, password []byte) error {
	t, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	u, err := syscall.UTF16PtrFromString(user)
	if err != nil {
		return err
	}
	// Пароль хранится как UTF-16LE без завершающего нуля.
	units := utf16.Encode([]rune(string(password)))
	blob := make([]byte, len(units)*2)
	for i, v := range units {
		blob[2*i], blob[2*i+1] = byte(v), byte(v>>8)
	}
	defer func() {
		for i := range blob {
			blob[i] = 0
		}
	}()

	cred := credentialW{
		Type:               credTypeGeneric,
		TargetName:         t,
		CredentialBlobSize: uint32(len(blob)),
		Persist:            credPersistSession,
		UserName:           u,
	}
	if len(blob) > 0 {
		cred.CredentialBlob = &blob[0]
	}
	if r, _, e := procCredWriteW.Call(uintptr(unsafe.Pointer(&cred)), 0); r == 0 {
		return fmt.Errorf("CredWrite %s: %w", target, e)
	}
	return nil
}

func credDelete(target string) {
	if t, err := syscall.UTF16PtrFromString(target); err == nil {
		_, _, _ = procCredDelete.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0)
	}
}

func openRDP(addr string, creds *RDPCredentials) error {
	if creds != nil && len(creds.Password) > 0 {
		targets := rdpTargets(addr)
		for _, target := range targets {
			if err := credWrite(target, creds.Account(), creds.Password); err != nil {
				return fmt.Errorf("сохранение пароля RDP для mstsc: %w", err)
			}
		}
		time.AfterFunc(rdpCredLifetime, func() {
			for _, target := range targets {
				credDelete(target)
			}
		})
	}
	cmd := exec.Command("mstsc.exe", "/v:"+addr)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск mstsc: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
