package terminal

import (
	"fmt"
	"strings"
)

// RDPCredentials — логин/пароль для автоматического входа по RDP.
type RDPCredentials struct {
	Username string
	Domain   string
	Password []byte
}

// Account — имя в виде DOMAIN\user (или просто user).
func (c *RDPCredentials) Account() string {
	if c.Domain != "" && !strings.ContainsAny(c.Username, `\@`) {
		return c.Domain + `\` + c.Username
	}
	return c.Username
}

// OpenRDP запускает RDP-клиент, подключающийся к addr ("127.0.0.1:13389").
// creds может быть nil — тогда логин и пароль спросит сам клиент.
func OpenRDP(addr string, creds *RDPCredentials) error {
	if strings.ContainsAny(addr, " \t\r\n\"'") || strings.HasPrefix(addr, "-") {
		return fmt.Errorf("invalid RDP address %q", addr)
	}
	if creds != nil && creds.Username == "" {
		creds = nil
	}
	return openRDP(addr, creds)
}
