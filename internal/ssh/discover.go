package ssh

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"ssh-launcher/internal/crypto"
)

// ListenSocket — TCP-сокет в состоянии LISTEN на удалённом сервере.
type ListenSocket struct {
	Addr    string // адрес без порта и зоны: "127.0.0.1", "::", "*"
	Port    int
	Process string // имя процесса, если ss/netstat его показали
}

type DiscoverRequest struct {
	Hostname        string
	Port            int
	Username        string
	PrivateKeyPEM   []byte
	Passphrase      []byte
	HostKeyCallback ssh.HostKeyCallback
	Timeout         time.Duration
}

type DiscoverOutput struct {
	OS      string
	Sockets []ListenSocket
}

// discoverScript выполняется в POSIX sh (login-shell может быть fish/csh).
// ss из iproute2 есть в любой Ubuntu; netstat — запасной вариант для старых
// систем. Без root -p показывает процессы только текущего пользователя.
const discoverScript = `. /etc/os-release 2>/dev/null; echo "OS=${PRETTY_NAME:-unknown}"; ` +
	`LC_ALL=C ss -tlnp 2>/dev/null || LC_ALL=C ss -tln 2>/dev/null || LC_ALL=C netstat -tlnp 2>/dev/null || LC_ALL=C netstat -tln`

// ListListeningSockets подключается по ключу и читает список слушающих TCP-портов.
func ListListeningSockets(ctx context.Context, req DiscoverRequest) (*DiscoverOutput, error) {
	defer crypto.ZeroBytes(req.PrivateKeyPEM)
	defer crypto.ZeroBytes(req.Passphrase)

	signer, err := parseSigner(req.PrivateKeyPEM, req.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	addr := net.JoinHostPort(req.Hostname, strconv.Itoa(req.Port))
	cfg := &ssh.ClientConfig{
		User:              req.Username,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback:   req.HostKeyCallback,
		HostKeyAlgorithms: knownHostKeyAlgorithms(addr),
		Timeout:           timeout,
	}
	if cfg.HostKeyCallback == nil {
		cfg.HostKeyCallback = StrictHostKeyCallback()
	}

	client, err := dialSSHContext(ctx, addr, cfg, timeout)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- sess.Run("sh -c " + shellQuote(discoverScript)) }()
	select {
	case <-ctx.Done():
		_ = client.Close()
		return nil, ctx.Err()
	case err := <-done:
		if err != nil && stdout.Len() == 0 {
			return nil, fmt.Errorf("не удалось получить список портов (нужен ss или netstat): %w %s", err, strings.TrimSpace(stderr.String()))
		}
	}
	return ParseListenOutput(stdout.String()), nil
}

var processRe = regexp.MustCompile(`\(\("([^"]+)"`)

// ParseListenOutput разбирает вывод discoverScript: строку OS= и таблицу
// `ss -tln[p]` или `netstat -tln[p]` (заголовки пропускаются).
func ParseListenOutput(out string) *DiscoverOutput {
	res := &DiscoverOutput{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "OS="); ok {
			res.OS = v
			continue
		}
		f := strings.Fields(line)
		var local, process string
		switch {
		case len(f) >= 5 && f[0] == "LISTEN": // ss
			local = f[3]
			if m := processRe.FindStringSubmatch(line); m != nil {
				process = m[1]
			}
		case len(f) >= 6 && strings.HasPrefix(f[0], "tcp") && f[5] == "LISTEN": // netstat
			local = f[3]
			if len(f) >= 7 {
				if _, name, ok := strings.Cut(f[6], "/"); ok {
					process = name
				}
			}
		default:
			continue
		}
		i := strings.LastIndex(local, ":")
		if i <= 0 {
			continue
		}
		port, err := strconv.Atoi(local[i+1:])
		if err != nil || port < 1 || port > 65535 {
			continue
		}
		host := strings.Trim(local[:i], "[]")
		if z := strings.Index(host, "%"); z >= 0 {
			host = host[:z] // 127.0.0.53%lo
		}
		res.Sockets = append(res.Sockets, ListenSocket{Addr: host, Port: port, Process: process})
	}
	return res
}

// Bind — на каких адресах слушает сервис.
const (
	BindLoopback = "loopback" // только 127.0.0.1 / ::1 — снаружи недоступен
	BindAll      = "all"      // 0.0.0.0 / :: — открыт на всех интерфейсах
	BindAddress  = "address"  // конкретный адрес (например, docker0 или LAN)
)

type DiscoveredService struct {
	Port       int      `json:"port"`
	Name       string   `json:"name"`
	Process    string   `json:"process,omitempty"`
	Bind       string   `json:"bind"`
	Addresses  []string `json:"addresses"`
	RemoteAddr string   `json:"remote_addr"` // цель для -L со стороны сервера
	IsRDP      bool     `json:"is_rdp"`
	System     bool     `json:"system"` // есть на чистой Ubuntu — туннель обычно не нужен
	Reason     string   `json:"reason,omitempty"`
}

// knownPorts — типичные сервисы, которые ставят на Ubuntu поверх чистой системы.
var knownPorts = map[int]string{
	21: "FTP", 22: "SSH", 25: "SMTP", 80: "HTTP", 110: "POP3", 143: "IMAP", 443: "HTTPS",
	465: "SMTPS", 587: "SMTP Submission", 993: "IMAPS", 995: "POP3S", 1883: "MQTT", 5678: "n8n", 1433: "Microsoft SQL Server", 1521: "Oracle DB",
	2375: "Docker API", 2376: "Docker API (TLS)", 2379: "etcd", 3000: "Grafana / Node.js",
	3306: "MySQL / MariaDB", 3389: "Удалённый рабочий стол (RDP)", 4369: "Erlang EPMD",
	5000: "HTTP (dev)", 5173: "Vite dev", 5432: "PostgreSQL", 5601: "Kibana", 5672: "RabbitMQ",
	5900: "VNC", 5901: "VNC :1", 6379: "Redis", 6443: "Kubernetes API",
	8000: "HTTP (dev)", 8006: "Proxmox VE", 8080: "HTTP (alt)", 8086: "InfluxDB", 8200: "HashiCorp Vault",
	8443: "HTTPS (alt)", 8500: "Consul", 8888: "Jupyter", 9000: "Portainer / MinIO",
	9090: "Cockpit / Prometheus", 9093: "Alertmanager", 9100: "node_exporter", 9200: "Elasticsearch",
	9443: "Portainer (HTTPS)", 11211: "Memcached", 15672: "RabbitMQ UI", 27017: "MongoDB",
}

var knownProcesses = map[string]string{
	"postgres": "PostgreSQL", "mysqld": "MySQL", "mariadbd": "MariaDB", "redis-server": "Redis",
	"mongod": "MongoDB", "xrdp": "Удалённый рабочий стол (xrdp)", "gnome-remote-de": "Удалённый рабочий стол (GNOME)",
	"nginx": "nginx", "apache2": "Apache", "caddy": "Caddy", "docker-proxy": "Docker (опубликованный порт)",
	"cockpit-ws": "Cockpit", "grafana": "Grafana", "grafana-server": "Grafana", "prometheus": "Prometheus",
	"x11vnc": "VNC", "Xvnc": "VNC", "Xtigervnc": "VNC", "memcached": "Memcached", "beam.smp": "RabbitMQ",
	"java": "Java", "node": "Node.js", "python3": "Python", "code-server": "code-server", "jupyter-lab": "Jupyter",
}

var genericProcesses = map[string]bool{"java": true, "node": true, "python3": true, "docker-proxy": true}

// ClassifyServices группирует сокеты по порту, подписывает известные сервисы
// и помечает порты, которые есть на чистой Ubuntu (Server и Desktop).
func ClassifyServices(sockets []ListenSocket, sshPort int) []DiscoveredService {
	byPort := map[int]*DiscoveredService{}
	var order []int
	for _, s := range sockets {
		d, ok := byPort[s.Port]
		if !ok {
			d = &DiscoveredService{Port: s.Port}
			byPort[s.Port] = d
			order = append(order, s.Port)
		}
		if !containsString(d.Addresses, s.Addr) {
			d.Addresses = append(d.Addresses, s.Addr)
		}
		if d.Process == "" && s.Process != "" {
			d.Process = s.Process
		}
	}
	sort.Ints(order)

	out := make([]DiscoveredService, 0, len(order))
	for _, port := range order {
		d := byPort[port]
		d.Bind, d.RemoteAddr = bindAndTarget(d.Addresses, port)
		d.Name = knownProcesses[d.Process]
		// Для «общих» процессов (java, node, docker-proxy) номер порта говорит больше.
		if d.Name == "" || genericProcesses[d.Process] {
			if n, ok := knownPorts[port]; ok {
				d.Name = n
			} else if d.Name == "" {
				d.Name = "Порт " + strconv.Itoa(port)
			}
		}
		d.IsRDP = port == 3389 || d.Process == "xrdp" || d.Process == "gnome-remote-de"
		d.System, d.Reason = baselineUbuntu(d, sshPort)
		out = append(out, *d)
	}
	return out
}

// baselineUbuntu: порты, которые слушает свежеустановленная Ubuntu 22.04/24.04.
func baselineUbuntu(d *DiscoveredService, sshPort int) (bool, string) {
	switch {
	case d.Port == sshPort || d.Port == 22:
		return true, "SSH — уже используется для подключения"
	case d.Port == 53 && onlyAddrs(d.Addresses, "127.0.0.53", "127.0.0.54"):
		return true, "systemd-resolved (локальный DNS)"
	case d.Port == 5355:
		return true, "systemd-resolved (LLMNR)"
	case d.Port == 111:
		return true, "rpcbind (NFS) — есть в облачных образах Ubuntu"
	case d.Port == 3350 && d.Bind == BindLoopback:
		return true, "xrdp-sesman (служебный порт xrdp)"
	case d.Port == 631 && d.Bind == BindLoopback:
		return true, "CUPS (служба печати Ubuntu Desktop)"
	case d.Port >= 32768 && knownPorts[d.Port] == "" && knownProcesses[d.Process] == "":
		return true, "служебный порт из динамического диапазона"
	}
	return false, ""
}

func bindAndTarget(addrs []string, port int) (bind, target string) {
	var hasAll, hasV4Loop, hasV6Loop bool
	var specific []string
	for _, a := range addrs {
		ip := net.ParseIP(a)
		switch {
		case a == "*" || a == "0.0.0.0" || a == "::":
			hasAll = true
		case ip != nil && ip.IsLoopback():
			if ip.To4() != nil {
				hasV4Loop = true
			} else {
				hasV6Loop = true
			}
		default:
			specific = append(specific, a)
		}
	}
	p := strconv.Itoa(port)
	switch {
	case hasAll:
		return BindAll, "127.0.0.1:" + p
	case hasV4Loop && len(specific) == 0:
		return BindLoopback, "127.0.0.1:" + p
	case hasV6Loop && len(specific) == 0:
		return BindLoopback, "[::1]:" + p
	case hasV4Loop:
		return BindAddress, "127.0.0.1:" + p
	default:
		return BindAddress, net.JoinHostPort(specific[0], p)
	}
}

func onlyAddrs(addrs []string, allowed ...string) bool {
	if len(addrs) == 0 {
		return false
	}
	for _, a := range addrs {
		if !containsString(allowed, a) {
			return false
		}
	}
	return true
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
