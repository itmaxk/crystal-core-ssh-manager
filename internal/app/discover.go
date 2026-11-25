package app

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	sshsvc "ssh-launcher/internal/ssh"
)

// DiscoveredService — сервис на сервере и предложенный для него туннель.
type DiscoveredService struct {
	Port           int      `json:"port"`
	Name           string   `json:"name"`
	Process        string   `json:"process,omitempty"`
	Bind           string   `json:"bind"`
	Addresses      []string `json:"addresses"`
	RemoteAddr     string   `json:"remote_addr"`
	LocalPort      int      `json:"local_port"`
	IsRDP          bool     `json:"is_rdp"`
	System         bool     `json:"system"`
	Reason         string   `json:"reason,omitempty"`
	ExistingTunnel string   `json:"existing_tunnel,omitempty"`
}

type DiscoveryResult struct {
	OS       string              `json:"os"`
	IsUbuntu bool                `json:"is_ubuntu"`
	Services []DiscoveredService `json:"services"`
}

// DiscoverServices подключается к серверу по ключу, читает слушающие TCP-порты
// (ss -tln) и предлагает локальные туннели к найденным сервисам. Порты чистой
// Ubuntu (SSH, systemd-resolved, CUPS) помечаются как системные.
func (a *App) DiscoverServices(hostID string) (*DiscoveryResult, error) {
	host, privPEM, passphrase, err := a.hostWithKey(hostID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	// ListListeningSockets затирает ключевой материал сам.
	out, err := sshsvc.ListListeningSockets(ctx, sshsvc.DiscoverRequest{
		Hostname:      host.Hostname,
		Port:          host.Port,
		Username:      host.Username,
		PrivateKeyPEM: privPEM,
		Passphrase:    passphrase,
	})
	if err != nil {
		return nil, err
	}
	return a.buildDiscovery(hostID, host.Port, out)
}

func (a *App) buildDiscovery(hostID string, sshPort int, out *sshsvc.DiscoverOutput) (*DiscoveryResult, error) {
	all, err := a.store.Tunnels.ListAll(a.ctx)
	if err != nil {
		return nil, err
	}
	taken := map[int]bool{} // локальные порты, занятые другими туннелями
	existing := map[int]string{}
	for _, t := range all {
		if p := portOfAddr(t.LocalAddr); p > 0 {
			taken[p] = true
		}
		if t.HostID == hostID && t.Direction == string(sshsvc.DirectionLocal) {
			if p := portOfAddr(t.RemoteAddr); p > 0 {
				existing[p] = t.Name
			}
		}
	}

	res := &DiscoveryResult{OS: out.OS, IsUbuntu: strings.Contains(strings.ToLower(out.OS), "ubuntu")}
	for _, s := range sshsvc.ClassifyServices(out.Sockets, sshPort) {
		d := DiscoveredService{
			Port: s.Port, Name: s.Name, Process: s.Process, Bind: s.Bind, Addresses: s.Addresses,
			RemoteAddr: s.RemoteAddr, IsRDP: s.IsRDP, System: s.System, Reason: s.Reason,
			ExistingTunnel: existing[s.Port],
		}
		if d.ExistingTunnel == "" && !d.System {
			d.LocalPort = suggestLocalPort(s.Port, s.IsRDP, taken, localPortFree)
			taken[d.LocalPort] = true
		}
		if d.IsRDP && s.Port != 3389 {
			// Кнопка «RDP» в UI ориентируется на порт 3389.
			d.IsRDP = false
		}
		res.Services = append(res.Services, d)
	}
	return res, nil
}

// suggestLocalPort выбирает локальный порт для туннеля:
//   - RDP → 13389: 3389 на Windows занят собственной службой, а mstsc не
//     подключается к 127.0.0.1:3389;
//   - привилегированные порты (< 1024) → +10000 (80 → 10080);
//   - иначе тот же порт, а если он занят — +10000, затем первый свободный с 20000.
func suggestLocalPort(remotePort int, rdp bool, taken map[int]bool, free func(int) bool) int {
	var candidates []int
	switch {
	case rdp:
		candidates = []int{13389, 23389}
	case remotePort < 1024:
		candidates = []int{remotePort + 10000}
	default:
		candidates = []int{remotePort, remotePort + 10000}
	}
	for _, p := range candidates {
		if p <= 65535 && !taken[p] && free(p) {
			return p
		}
	}
	for p := 20000; p <= 65535; p++ {
		if !taken[p] && free(p) {
			return p
		}
	}
	return 0
}

func localPortFree(port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func portOfAddr(addr string) int {
	_, p, err := net.SplitHostPort(sshsvc.NormalizeListenAddr(addr))
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(p)
	return n
}
