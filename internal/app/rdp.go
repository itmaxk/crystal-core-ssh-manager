package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	sshsvc "ssh-launcher/internal/ssh"
	"ssh-launcher/internal/terminal"
)

// OpenRDP поднимает локальный туннель (если он не запущен), дожидается
// открытия порта и запускает системный RDP-клиент на 127.0.0.1:<порт>.
func (a *App) OpenRDP(tunnelID string) error {
	if err := a.ready(); err != nil {
		return err
	}
	t, err := a.store.Tunnels.GetByID(a.ctx, tunnelID)
	if err != nil {
		return fmt.Errorf("load tunnel: %w", err)
	}
	if t.Direction != string(sshsvc.DirectionLocal) {
		return errors.New("RDP открывается только через локальный туннель (-L)")
	}
	addr, err := rdpConnectAddr(t.LocalAddr)
	if err != nil {
		return err
	}

	if !a.tunnels.IsRunning(t.ID) {
		if err := a.StartTunnel(t.ID, t.HostID); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
	defer cancel()
	if err := a.waitTunnelPort(ctx, t.ID, addr); err != nil {
		return err
	}
	return terminal.OpenRDP(addr)
}

// rdpConnectAddr превращает адрес прослушивания туннеля в адрес для клиента:
// "13389" и "0.0.0.0:13389" → "127.0.0.1:13389".
func rdpConnectAddr(localAddr string) (string, error) {
	host, port, err := net.SplitHostPort(sshsvc.NormalizeListenAddr(localAddr))
	if err != nil {
		return "", fmt.Errorf("invalid local address %q", localAddr)
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]", "localhost":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}

// waitTunnelPort ждёт состояния active и успешного TCP-подключения к addr
// (туннель становится active чуть раньше, чем начинает слушать порт).
func (a *App) waitTunnelPort(ctx context.Context, tunnelID, addr string) error {
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	lastErr := ""
	for {
		st, running := a.tunnels.Status(tunnelID)
		if !running {
			return errors.New("туннель остановлен")
		}
		if st.LastError != "" {
			lastErr = st.LastError
		}
		if st.State == sshsvc.StateActive {
			d := net.Dialer{Timeout: 500 * time.Millisecond}
			if conn, err := d.DialContext(ctx, "tcp", addr); err == nil {
				_ = conn.Close()
				return nil
			}
		}
		select {
		case <-ctx.Done():
			if lastErr != "" {
				return fmt.Errorf("туннель не поднялся: %s", lastErr)
			}
			return errors.New("туннель не поднялся за 25 секунд")
		case <-tick.C:
		}
	}
}
