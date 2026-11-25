package ssh

import "testing"

// Чистая Ubuntu 24.04 Server (ss без root: процессы не видны).
const ubuntuServerClean = `OS=Ubuntu 24.04.1 LTS
State  Recv-Q Send-Q Local Address:Port  Peer Address:PortProcess
LISTEN 0      4096      127.0.0.54:53         0.0.0.0:*
LISTEN 0      4096   127.0.0.53%lo:53         0.0.0.0:*
LISTEN 0      4096         0.0.0.0:22         0.0.0.0:*
LISTEN 0      4096            [::]:22            [::]:*
`

// Ubuntu 24.04 Desktop с PostgreSQL, Redis в Docker, xrdp и dev-сервером (ss от root).
const ubuntuDesktopWithServices = `OS=Ubuntu 24.04.1 LTS
LISTEN 0 4096 127.0.0.53%lo:53 0.0.0.0:* users:(("systemd-resolve",pid=601,fd=15))
LISTEN 0 4096 127.0.0.1:631 0.0.0.0:* users:(("cupsd",pid=900,fd=7))
LISTEN 0 4096 [::1]:631 [::]:* users:(("cupsd",pid=900,fd=6))
LISTEN 0 200 127.0.0.1:5432 0.0.0.0:* users:(("postgres",pid=1200,fd=6))
LISTEN 0 4096 0.0.0.0:6379 0.0.0.0:* users:(("docker-proxy",pid=1500,fd=4))
LISTEN 0 4096 [::]:6379 [::]:* users:(("docker-proxy",pid=1501,fd=4))
LISTEN 0 2 *:3389 *:* users:(("xrdp",pid=1300,fd=11))
LISTEN 0 2 [::1]:3350 [::]:* users:(("xrdp-sesman",pid=1290,fd=7))
LISTEN 0 4096 0.0.0.0:111 0.0.0.0:* users:(("rpcbind",pid=500,fd=4))
LISTEN 0 511 172.17.0.1:8080 0.0.0.0:* users:(("node",pid=2000,fd=20))
LISTEN 0 128 127.0.0.1:41235 0.0.0.0:* users:(("code-insiders",pid=2100,fd=30))
LISTEN 0 128 0.0.0.0:2222 0.0.0.0:* users:(("sshd",pid=1,fd=3))
`

const netstatOutput = `OS=Ubuntu 18.04.6 LTS
Active Internet connections (only servers)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp        0      0 127.0.0.1:3306          0.0.0.0:*               LISTEN      812/mysqld
tcp6       0      0 :::22                   :::*                    LISTEN      -
`

func TestParseAndClassifyCleanUbuntu(t *testing.T) {
	out := ParseListenOutput(ubuntuServerClean)
	if out.OS != "Ubuntu 24.04.1 LTS" || len(out.Sockets) != 4 {
		t.Fatalf("parse: %+v", out)
	}
	for _, s := range ClassifyServices(out.Sockets, 22) {
		if !s.System {
			t.Errorf("clean Ubuntu port %d (%v) must be system", s.Port, s.Addresses)
		}
	}
}

func TestClassifyDesktopServices(t *testing.T) {
	out := ParseListenOutput(ubuntuDesktopWithServices)
	got := map[int]DiscoveredService{}
	for _, s := range ClassifyServices(out.Sockets, 2222) {
		got[s.Port] = s
	}
	cases := []struct {
		port           int
		name, bind, to string
		system, rdp    bool
	}{
		{53, "", BindLoopback, "127.0.0.1:53", true, false},
		{631, "", BindLoopback, "127.0.0.1:631", true, false},
		{2222, "", BindAll, "127.0.0.1:2222", true, false},
		{41235, "", BindLoopback, "127.0.0.1:41235", true, false},
		{3350, "", BindLoopback, "[::1]:3350", true, false},
		{111, "", BindAll, "127.0.0.1:111", true, false},
		{5432, "PostgreSQL", BindLoopback, "127.0.0.1:5432", false, false},
		{6379, "Redis", BindAll, "127.0.0.1:6379", false, false},
		{3389, "Удалённый рабочий стол (xrdp)", BindAll, "127.0.0.1:3389", false, true},
		{8080, "HTTP (alt)", BindAddress, "172.17.0.1:8080", false, false},
	}
	for _, c := range cases {
		s, ok := got[c.port]
		if !ok {
			t.Errorf("port %d not found", c.port)
			continue
		}
		if (c.name != "" && s.Name != c.name) || s.Bind != c.bind || s.RemoteAddr != c.to || s.System != c.system || s.IsRDP != c.rdp {
			t.Errorf("port %d: got %+v", c.port, s)
		}
	}
	if len(got[631].Addresses) != 2 {
		t.Errorf("631 should merge v4+v6: %v", got[631].Addresses)
	}
}

func TestParseNetstatFallback(t *testing.T) {
	out := ParseListenOutput(netstatOutput)
	if len(out.Sockets) != 2 || out.Sockets[0].Process != "mysqld" || out.Sockets[0].Port != 3306 || out.Sockets[1].Addr != "::" {
		t.Fatalf("netstat parse: %+v", out.Sockets)
	}
	svc := ClassifyServices(out.Sockets, 22)
	if svc[1].Name != "MySQL" || svc[1].System || !svc[0].System {
		t.Fatalf("classify: %+v", svc)
	}
}

func TestIPv6OnlyLoopbackTarget(t *testing.T) {
	svc := ClassifyServices([]ListenSocket{{Addr: "::1", Port: 9000}}, 22)
	if svc[0].RemoteAddr != "[::1]:9000" || svc[0].Bind != BindLoopback {
		t.Fatalf("got %+v", svc[0])
	}
	if err := ValidateTunnelAddr(svc[0].RemoteAddr); err != nil {
		t.Fatalf("target must be a valid tunnel address: %v", err)
	}
}
