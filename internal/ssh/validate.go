package ssh

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var hostnameRe = regexp.MustCompile(`^[A-Za-z0-9._:%\[\]-]+$`)

// ValidateHostname отсекает значения, которые системный ssh или ~/.ssh/config
// интерпретируют как опции/директивы (например, "-oProxyCommand=...").
func ValidateHostname(h string) error {
	if h == "" {
		return errors.New("hostname is required")
	}
	if strings.HasPrefix(h, "-") {
		return fmt.Errorf("invalid hostname %q: must not start with '-'", h)
	}
	if !hostnameRe.MatchString(h) {
		return fmt.Errorf("invalid hostname %q", h)
	}
	return nil
}

// ValidateUsername допускает доменные логины (user@domain, DOMAIN\user),
// но запрещает пробелы, управляющие символы и ведущий '-'.
func ValidateUsername(u string) error {
	if u == "" {
		return errors.New("username is required")
	}
	if strings.HasPrefix(u, "-") {
		return fmt.Errorf("invalid username %q: must not start with '-'", u)
	}
	for _, r := range u {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == ':' || r == '"' || r == '\'' {
			return fmt.Errorf("invalid username %q", u)
		}
	}
	return nil
}

// ValidatePort проверяет диапазон TCP-порта.
func ValidatePort(p int) error {
	if p < 1 || p > 65535 {
		return fmt.Errorf("invalid port %d", p)
	}
	return nil
}
