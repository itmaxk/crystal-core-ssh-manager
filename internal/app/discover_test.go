package app

import (
	"testing"

	"ssh-launcher/internal/db"
	sshsvc "ssh-launcher/internal/ssh"
)

func TestSuggestLocalPort(t *testing.T) {
	allFree := func(int) bool { return true }
	busy := func(ports ...int) func(int) bool {
		return func(p int) bool {
			for _, b := range ports {
				if b == p {
					return false
				}
			}
			return true
		}
	}
	cases := []struct {
		name   string
		remote int
		rdp    bool
		taken  map[int]bool
		free   func(int) bool
		want   int
	}{
		{"same port", 5432, false, nil, allFree, 5432},
		{"privileged", 80, false, nil, allFree, 10080},
		{"rdp avoids 3389", 3389, true, nil, allFree, 13389},
		{"busy locally", 5432, false, nil, busy(5432), 15432},
		{"taken by tunnel", 6379, false, map[int]bool{6379: true, 16379: true}, allFree, 20000},
		{"high port overflow", 60000, false, nil, busy(60000), 20000},
	}
	for _, c := range cases {
		taken := c.taken
		if taken == nil {
			taken = map[int]bool{}
		}
		if got := suggestLocalPort(c.remote, c.rdp, taken, c.free); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestBuildDiscoveryMarksExistingAndAvoidsTakenPorts(t *testing.T) {
	a := newTestApp(t)
	h, err := a.SaveHost(db.Host{Name: "srv", Hostname: "10.0.0.1", Username: "root"})
	if err != nil {
		t.Fatal(err)
	}
	// Уже есть туннель к PostgreSQL; локальный 6379 занят туннелем другого назначения.
	if _, err := a.SaveTunnel(db.Tunnel{HostID: h.ID, Name: "pg", Direction: "L", LocalAddr: "15432", RemoteAddr: "localhost:5432"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveTunnel(db.Tunnel{HostID: h.ID, Name: "other", Direction: "L", LocalAddr: "6379", RemoteAddr: "localhost:7000"}); err != nil {
		t.Fatal(err)
	}

	res, err := a.buildDiscovery(h.ID, 22, &sshsvc.DiscoverOutput{OS: "Ubuntu 24.04.1 LTS", Sockets: []sshsvc.ListenSocket{
		{Addr: "0.0.0.0", Port: 22},
		{Addr: "127.0.0.1", Port: 5432, Process: "postgres"},
		{Addr: "127.0.0.1", Port: 6379, Process: "redis-server"},
		{Addr: "0.0.0.0", Port: 3389, Process: "xrdp"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsUbuntu || len(res.Services) != 4 {
		t.Fatalf("result: %+v", res)
	}
	byPort := map[int]DiscoveredService{}
	for _, s := range res.Services {
		byPort[s.Port] = s
	}
	if s := byPort[22]; !s.System || s.LocalPort != 0 {
		t.Errorf("ssh: %+v", s)
	}
	if s := byPort[5432]; s.ExistingTunnel != "pg" || s.LocalPort != 0 {
		t.Errorf("postgres should be marked as tunneled: %+v", s)
	}
	if s := byPort[6379]; s.LocalPort == 6379 || s.LocalPort == 0 {
		t.Errorf("redis must not reuse a port taken by another tunnel: %+v", s)
	}
	if s := byPort[3389]; !s.IsRDP || s.LocalPort == 3389 || s.RemoteAddr != "127.0.0.1:3389" {
		t.Errorf("rdp: %+v", s)
	}
}
