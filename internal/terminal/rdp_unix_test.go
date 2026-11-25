//go:build !windows

package terminal

import (
	"slices"
	"testing"
)

func TestFreeRDPArgsPasswordViaStdin(t *testing.T) {
	args, stdin := freeRDPArgs("127.0.0.1:13389", &RDPCredentials{Username: "ivan", Password: []byte("pw")})
	if slices.ContainsFunc(args, func(a string) bool { return len(a) > 2 && a[:3] == "/p:" }) {
		t.Fatalf("password must not be passed as an argument: %v", args)
	}
	// Без /d: FreeRDP сначала спросит домен — отвечаем пустой строкой.
	if string(stdin) != "\npw\n" || !slices.Contains(args, "/from-stdin:force") || !slices.Contains(args, "/u:ivan") {
		t.Fatalf("args=%v stdin=%q", args, stdin)
	}

	args, stdin = freeRDPArgs("127.0.0.1:13389", &RDPCredentials{Username: "ivan", Domain: "CORP", Password: []byte("pw")})
	if string(stdin) != "pw\n" || !slices.Contains(args, "/d:CORP") {
		t.Fatalf("with domain: args=%v stdin=%q", args, stdin)
	}

	args, stdin = freeRDPArgs("127.0.0.1:13389", &RDPCredentials{Username: "ivan"})
	if stdin != nil || slices.Contains(args, "/from-stdin:force") {
		t.Fatalf("no password: args=%v stdin=%q", args, stdin)
	}
}

func TestLinuxClientOrder(t *testing.T) {
	withPw := linuxRDPClients("127.0.0.1:13389", &RDPCredentials{Username: "ivan", Password: []byte("pw")})
	if withPw[0].bin != "xfreerdp3" {
		t.Errorf("with password FreeRDP must be preferred, got %s", withPw[0].bin)
	}
	noPw := linuxRDPClients("127.0.0.1:13389", nil)
	if noPw[0].bin != "remmina" {
		t.Errorf("without credentials Remmina is preferred, got %s", noPw[0].bin)
	}
	user := linuxRDPClients("127.0.0.1:13389", &RDPCredentials{Username: "ivan", Domain: "CORP"})
	if user[0].bin != "remmina" || user[0].args[1] != `rdp://CORP%5Civan@127.0.0.1:13389` {
		t.Errorf("remmina with username: %v", user[0].args)
	}
}

func TestAccount(t *testing.T) {
	for _, c := range []struct{ user, domain, want string }{
		{"ivan", "", "ivan"}, {"ivan", "CORP", `CORP\ivan`}, {`CORP\ivan`, "X", `CORP\ivan`}, {"ivan@corp.local", "X", "ivan@corp.local"},
	} {
		if got := (&RDPCredentials{Username: c.user, Domain: c.domain}).Account(); got != c.want {
			t.Errorf("Account(%q,%q)=%q want %q", c.user, c.domain, got, c.want)
		}
	}
}
