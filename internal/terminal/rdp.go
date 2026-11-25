package terminal

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	goruntime "runtime"
	"strings"
)

// rdpClient — найденный RDP-клиент и аргументы для подключения к addr.
type rdpClient struct {
	bin  string
	args []string
}

// linuxRDPClients в порядке предпочтения. Remmina сама показывает диалог
// сертификата; FreeRDP без терминала не сможет спросить, поэтому /cert:tofu.
func linuxRDPClients(addr string) []rdpClient {
	freerdp := []string{"/v:" + addr, "/cert:tofu", "+clipboard", "/dynamic-resolution"}
	return []rdpClient{
		{"remmina", []string{"-c", "rdp://" + addr}},
		{"xfreerdp3", freerdp},
		{"xfreerdp", freerdp},
		{"wlfreerdp", freerdp},
	}
}

// RDPCommand подбирает команду запуска RDP-клиента для текущей ОС.
func RDPCommand(addr string) (*exec.Cmd, error) {
	switch goruntime.GOOS {
	case "windows":
		return exec.Command("mstsc.exe", "/v:"+addr), nil
	case "darwin":
		// URI-схема Windows App / Microsoft Remote Desktop.
		return exec.Command("open", "rdp://full%20address=s:"+url.PathEscape(addr)), nil
	default:
		for _, c := range linuxRDPClients(addr) {
			if path, err := exec.LookPath(c.bin); err == nil {
				return exec.Command(path, c.args...), nil
			}
		}
		return nil, errors.New("RDP-клиент не найден: установите Remmina или FreeRDP (xfreerdp)")
	}
}

// OpenRDP запускает RDP-клиент, подключающийся к addr ("127.0.0.1:13389").
func OpenRDP(addr string) error {
	if strings.ContainsAny(addr, " \t\r\n\"'") || strings.HasPrefix(addr, "-") {
		return fmt.Errorf("invalid RDP address %q", addr)
	}
	cmd, err := RDPCommand(addr)
	if err != nil {
		return err
	}
	if goruntime.GOOS == "darwin" {
		// open завершается сразу и сообщает, если нет приложения для rdp://.
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("не удалось открыть RDP (установите Windows App из App Store): %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск RDP-клиента: %w", err)
	}
	go func() { _ = cmd.Wait() }() // не оставляем зомби-процесс
	return nil
}
