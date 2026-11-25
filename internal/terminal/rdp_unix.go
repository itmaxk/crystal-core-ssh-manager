//go:build !windows

package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	goruntime "runtime"
	"strings"
)

// rdpClient — найденный RDP-клиент и аргументы для подключения к addr.
type rdpClient struct {
	bin   string
	args  []string
	stdin []byte
}

func freeRDPArgs(addr string, creds *RDPCredentials) ([]string, []byte) {
	args := []string{"/v:" + addr, "/cert:tofu", "+clipboard", "/dynamic-resolution"}
	if creds == nil {
		return args, nil
	}
	args = append(args, "/u:"+creds.Username)
	var stdin []byte
	if creds.Domain != "" {
		args = append(args, "/d:"+creds.Domain)
	} else {
		// FreeRDP спрашивает домен, если он не задан: отвечаем пустой строкой.
		stdin = append(stdin, '\n')
	}
	if len(creds.Password) > 0 {
		// Пароль через stdin, а не /p:, чтобы он не попал в список процессов.
		args = append(args, "/from-stdin:force")
		stdin = append(stdin, creds.Password...)
		stdin = append(stdin, '\n')
	} else {
		stdin = nil
	}
	return args, stdin
}

// linuxRDPClients в порядке предпочтения. С сохранённым паролем первым идёт
// FreeRDP (принимает пароль через stdin); Remmina получает только логин.
func linuxRDPClients(addr string, creds *RDPCredentials) []rdpClient {
	freeArgs, stdin := freeRDPArgs(addr, creds)
	remminaURL := "rdp://" + addr
	if creds != nil {
		remminaURL = "rdp://" + url.User(creds.Account()).String() + "@" + addr
	}
	free := []rdpClient{
		{"xfreerdp3", freeArgs, stdin},
		{"xfreerdp", freeArgs, stdin},
		{"wlfreerdp", freeArgs, stdin},
	}
	remmina := rdpClient{"remmina", []string{"-c", remminaURL}, nil}
	if creds != nil && len(creds.Password) > 0 {
		return append(free, remmina)
	}
	return append([]rdpClient{remmina}, free...)
}

func openRDP(addr string, creds *RDPCredentials) error {
	if goruntime.GOOS == "darwin" {
		// Windows App / Microsoft Remote Desktop принимают логин в URI,
		// пароль — нет (его можно сохранить в самом Windows App).
		uri := "rdp://full%20address=s:" + url.PathEscape(addr)
		if creds != nil {
			uri += "&username=s:" + url.QueryEscape(creds.Account())
		}
		if out, err := exec.Command("open", uri).CombinedOutput(); err != nil {
			return fmt.Errorf("не удалось открыть RDP (установите Windows App из App Store): %s", strings.TrimSpace(string(out)))
		}
		return nil
	}

	for _, c := range linuxRDPClients(addr, creds) {
		path, err := exec.LookPath(c.bin)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, c.args...)
		if c.stdin != nil {
			cmd.Stdin = bytes.NewReader(c.stdin)
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("запуск %s: %w", c.bin, err)
		}
		go func(stdin []byte) {
			_ = cmd.Wait() // не оставляем зомби-процесс
			for i := range stdin {
				stdin[i] = 0
			}
		}(c.stdin)
		return nil
	}
	return errors.New("RDP-клиент не найден: установите Remmina или FreeRDP (xfreerdp)")
}
