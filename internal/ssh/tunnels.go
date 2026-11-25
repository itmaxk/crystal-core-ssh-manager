package ssh

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"ssh-launcher/internal/crypto"
)

type TunnelDirection string

const (
	DirectionLocal  TunnelDirection = "L"
	DirectionRemote TunnelDirection = "R"
	DirectionSOCKS5 TunnelDirection = "D"
)

type TunnelState string

const (
	StateStopped      TunnelState = "stopped"
	StateConnecting   TunnelState = "connecting"
	StateActive       TunnelState = "active"
	StateReconnecting TunnelState = "reconnecting"
	StateError        TunnelState = "error"
)

type TunnelStatus struct {
	TunnelID    string      `json:"tunnel_id"`
	State       TunnelState `json:"state"`
	ActiveConns int64       `json:"active_conns"`
	LastError   string      `json:"last_error,omitempty"`
}

type TunnelConfig struct {
	ID              string
	Direction       TunnelDirection
	LocalAddr       string
	RemoteAddr      string
	Hostname        string
	Port            int
	Username        string
	PrivateKeyPEM   []byte
	Passphrase      []byte
	HostKeyCallback ssh.HostKeyCallback
}

type runningTunnel struct {
	cancel      context.CancelFunc
	state       TunnelState // защищено TunnelManager.mu
	lastError   string      // защищено TunnelManager.mu
	stopped     bool        // защищено TunnelManager.mu
	activeConns atomic.Int64
}

type TunnelManager struct {
	mu       sync.Mutex
	tunnels  map[string]*runningTunnel
	onStatus func(TunnelStatus)
}

func NewTunnelManager(onStatus func(TunnelStatus)) *TunnelManager {
	return &TunnelManager{
		tunnels:  make(map[string]*runningTunnel),
		onStatus: onStatus,
	}
}

func (m *TunnelManager) Start(cfg TunnelConfig) error {
	defer crypto.ZeroBytes(cfg.PrivateKeyPEM)
	defer crypto.ZeroBytes(cfg.Passphrase)

	signer, err := parseSigner(cfg.PrivateKeyPEM, cfg.Passphrase)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}

	m.mu.Lock()
	if _, exists := m.tunnels[cfg.ID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("tunnel %s is already running", cfg.ID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	rt := &runningTunnel{cancel: cancel, state: StateConnecting}
	m.tunnels[cfg.ID] = rt
	m.mu.Unlock()

	m.emit(cfg.ID, rt)
	go m.runLoop(ctx, cfg, signer, rt)
	return nil
}

func (m *TunnelManager) Stop(tunnelID string) {
	m.mu.Lock()
	rt, ok := m.tunnels[tunnelID]
	if ok {
		delete(m.tunnels, tunnelID)
		rt.stopped = true
		rt.state = StateStopped
		rt.lastError = ""
	}
	m.mu.Unlock()

	if ok {
		rt.cancel()
		m.emit(tunnelID, rt)
	}
}

// Statuses возвращает состояние всех запущенных туннелей (для восстановления UI).
func (m *TunnelManager) Statuses() []TunnelStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]TunnelStatus, 0, len(m.tunnels))
	for id, rt := range m.tunnels {
		out = append(out, m.statusLocked(id, rt))
	}
	return out
}

func (m *TunnelManager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.tunnels))
	for id := range m.tunnels {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		m.Stop(id)
	}
}

func (m *TunnelManager) runLoop(ctx context.Context, cfg TunnelConfig, signer ssh.Signer, rt *runningTunnel) {
	backoff := 2 * time.Second
	const maxBackoff = 30 * time.Second

	sshAddr := net.JoinHostPort(cfg.Hostname, strconv.Itoa(cfg.Port))
	sshCfg := &ssh.ClientConfig{
		User:              cfg.Username,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback:   cfg.HostKeyCallback,
		HostKeyAlgorithms: knownHostKeyAlgorithms(sshAddr),
		Timeout:           15 * time.Second,
	}
	if sshCfg.HostKeyCallback == nil {
		sshCfg.HostKeyCallback = StrictHostKeyCallback()
	}

	for {
		if ctx.Err() != nil {
			return
		}

		m.setState(cfg.ID, rt, StateConnecting, "")
		client, err := dialSSHContext(ctx, sshAddr, sshCfg, sshCfg.Timeout)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.setState(cfg.ID, rt, StateReconnecting, err.Error())
			if !sleepContext(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		backoff = 2 * time.Second
		m.setState(cfg.ID, rt, StateActive, "")

		sessCtx, sessCancel := context.WithCancel(ctx)
		go runKeepalive(sessCtx, client, 20*time.Second, sessCancel)
		go func() {
			// Разрыв транспорта обнаруживаем сразу, не дожидаясь keepalive.
			_ = client.Wait()
			sessCancel()
		}()

		var serveErr error
		switch cfg.Direction {
		case DirectionLocal:
			serveErr = m.serveLocal(sessCtx, client, cfg.LocalAddr, cfg.RemoteAddr, cfg.ID, rt)
		case DirectionRemote:
			serveErr = m.serveRemote(sessCtx, client, cfg.RemoteAddr, cfg.LocalAddr, cfg.ID, rt)
		case DirectionSOCKS5:
			serveErr = m.serveSOCKS5(sessCtx, client, cfg.LocalAddr, cfg.ID, rt)
		default:
			serveErr = fmt.Errorf("unknown direction: %s", cfg.Direction)
		}

		sessCancel()
		_ = client.Close()

		if ctx.Err() != nil {
			return
		}

		errMsg := "connection lost"
		if serveErr != nil {
			errMsg = serveErr.Error()
		}
		m.setState(cfg.ID, rt, StateReconnecting, errMsg)
		if !sleepContext(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func (m *TunnelManager) serveLocal(ctx context.Context, client *ssh.Client, localAddr, remoteAddr, id string, rt *runningTunnel) error {
	ln, err := net.Listen("tcp", NormalizeListenAddr(localAddr))
	if err != nil {
		return fmt.Errorf("listen %s: %w", localAddr, err)
	}
	return m.acceptLoop(ctx, ln, func(localConn net.Conn) {
		defer localConn.Close()
		remoteConn, err := client.Dial("tcp", NormalizeListenAddr(remoteAddr))
		if err != nil {
			return
		}
		defer remoteConn.Close()
		m.proxyWithCounter(localConn, remoteConn, id, rt)
	})
}

func (m *TunnelManager) serveRemote(ctx context.Context, client *ssh.Client, remoteAddr, localAddr, id string, rt *runningTunnel) error {
	ln, err := client.Listen("tcp", NormalizeListenAddr(remoteAddr))
	if err != nil {
		return fmt.Errorf("remote listen %s: %w", remoteAddr, err)
	}
	return m.acceptLoop(ctx, ln, func(remoteConn net.Conn) {
		defer remoteConn.Close()
		localConn, err := net.DialTimeout("tcp", NormalizeListenAddr(localAddr), 10*time.Second)
		if err != nil {
			return
		}
		defer localConn.Close()
		m.proxyWithCounter(remoteConn, localConn, id, rt)
	})
}

func (m *TunnelManager) serveSOCKS5(ctx context.Context, client *ssh.Client, localAddr, id string, rt *runningTunnel) error {
	ln, err := net.Listen("tcp", NormalizeListenAddr(localAddr))
	if err != nil {
		return fmt.Errorf("socks5 listen %s: %w", localAddr, err)
	}
	return m.acceptLoop(ctx, ln, func(conn net.Conn) {
		defer conn.Close()
		targetAddr, err := handshakeSOCKS5(conn)
		if err != nil {
			return
		}
		remoteConn, err := client.Dial("tcp", targetAddr)
		if err != nil {
			_, _ = conn.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		defer remoteConn.Close()

		if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
			return
		}
		m.proxyWithCounter(conn, remoteConn, id, rt)
	})
}

func handshakeSOCKS5(conn net.Conn) (string, error) {
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{})

	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}
	if header[0] != 0x05 {
		return "", errors.New("unsupported socks version")
	}
	methods := make([]byte, header[1])
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", err
	}
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return "", err
	}

	req := make([]byte, 4)
	if _, err := io.ReadFull(conn, req); err != nil {
		return "", err
	}
	if req[1] != 0x01 {
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return "", errors.New("unsupported socks command")
	}

	var host string
	switch req[3] {
	case 0x01:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case 0x03:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		domain := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", err
		}
		host = string(domain)
	case 0x04:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	default:
		return "", errors.New("unsupported address type")
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBuf)
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func (m *TunnelManager) acceptLoop(ctx context.Context, ln net.Listener, handler func(net.Conn)) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go handler(conn)
	}
}

func (m *TunnelManager) proxyWithCounter(a, b net.Conn, id string, rt *runningTunnel) {
	rt.activeConns.Add(1)
	m.emit(id, rt)
	defer func() {
		rt.activeConns.Add(-1)
		m.emit(id, rt)
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		_ = a.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		_ = b.Close()
	}()
	wg.Wait()
}

func runKeepalive(ctx context.Context, client *ssh.Client, interval time.Duration, onDead context.CancelFunc) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				onDead()
				return
			}
		}
	}
}

func (m *TunnelManager) setState(id string, rt *runningTunnel, state TunnelState, errMsg string) {
	m.mu.Lock()
	if rt.stopped {
		// После Stop горутина ещё может завершать работу — не перетираем "stopped".
		m.mu.Unlock()
		return
	}
	rt.state = state
	rt.lastError = errMsg
	m.mu.Unlock()
	m.emit(id, rt)
}

func (m *TunnelManager) emit(id string, rt *runningTunnel) {
	if m.onStatus == nil {
		return
	}
	m.mu.Lock()
	status := m.statusLocked(id, rt)
	m.mu.Unlock()
	m.onStatus(status)
}

func (m *TunnelManager) statusLocked(id string, rt *runningTunnel) TunnelStatus {
	return TunnelStatus{
		TunnelID:    id,
		State:       rt.state,
		ActiveConns: rt.activeConns.Load(),
		LastError:   rt.lastError,
	}
}

// Status возвращает текущее состояние туннеля; false — туннель не запущен.
func (m *TunnelManager) Status(tunnelID string) (TunnelStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt, ok := m.tunnels[tunnelID]
	if !ok {
		return TunnelStatus{TunnelID: tunnelID, State: StateStopped}, false
	}
	return m.statusLocked(tunnelID, rt), true
}

// IsRunning сообщает, запущен ли туннель (в любом состоянии, кроме остановленного).
func (m *TunnelManager) IsRunning(tunnelID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.tunnels[tunnelID]
	return ok
}

// ValidateTunnelAddr проверяет адрес вида "port", ":port" или "host:port".
func ValidateTunnelAddr(addr string) error {
	host, port, err := net.SplitHostPort(NormalizeListenAddr(addr))
	if err != nil {
		return fmt.Errorf("invalid address %q: expected port or host:port", addr)
	}
	if host != "" && ValidateHostname(host) != nil {
		return fmt.Errorf("invalid host in %q", addr)
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("invalid port in %q", addr)
	}
	return nil
}

// NormalizeListenAddr превращает "8080" / ":8080" в "127.0.0.1:8080", чтобы
// локальные туннели по умолчанию не слушали на всех интерфейсах.
func NormalizeListenAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if _, err := strconv.Atoi(addr); err == nil {
		return "127.0.0.1:" + addr
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
