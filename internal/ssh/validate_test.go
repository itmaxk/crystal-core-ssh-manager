package ssh

import "testing"

func TestValidateHostname(t *testing.T) {
	ok := []string{"example.com", "10.0.0.1", "localhost", "fe80::1", "[::1]", "my_host-01"}
	bad := []string{"", "-oProxyCommand=calc", "host name", "host\nHostName evil", "a;b", "user@host"}
	for _, h := range ok {
		if err := ValidateHostname(h); err != nil {
			t.Errorf("ValidateHostname(%q) = %v, want nil", h, err)
		}
	}
	for _, h := range bad {
		if err := ValidateHostname(h); err == nil {
			t.Errorf("ValidateHostname(%q) = nil, want error", h)
		}
	}
}

func TestValidateUsername(t *testing.T) {
	ok := []string{"root", "ubuntu", "john.doe", "user@corp.local", `CORP\user`}
	bad := []string{"", "-oProxyCommand=calc", "a b", "a\nb", "a:b"}
	for _, u := range ok {
		if err := ValidateUsername(u); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", u, err)
		}
	}
	for _, u := range bad {
		if err := ValidateUsername(u); err == nil {
			t.Errorf("ValidateUsername(%q) = nil, want error", u)
		}
	}
}
