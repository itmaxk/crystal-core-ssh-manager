package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"ssh-launcher/internal/crypto"
	"ssh-launcher/internal/db"
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

	creds, err := a.rdpCredentials(t)
	if err != nil {
		return err
	}
	if creds != nil {
		defer crypto.ZeroBytes(creds.Password)
	}
	return terminal.OpenRDP(addr, creds)
}

func (a *App) rdpCredentials(t *db.Tunnel) (*terminal.RDPCredentials, error) {
	if t.RDPUsername == "" {
		return nil, nil
	}
	creds := &terminal.RDPCredentials{Username: t.RDPUsername, Domain: t.RDPDomain}
	if t.HasRDPPassword {
		enc, err := a.store.Tunnels.RDPPasswordEnc(a.ctx, t.ID)
		if err != nil {
			return nil, err
		}
		if creds.Password, err = a.vault.Decrypt(enc); err != nil {
			return nil, fmt.Errorf("decrypt RDP password: %w", err)
		}
	}
	return creds, nil
}

// RDPCredentialsInput — данные из формы туннеля. Пустой Password при
// KeepPassword оставляет сохранённый пароль; пустой Username удаляет всё.
type RDPCredentialsInput struct {
	TunnelID     string `json:"tunnel_id"`
	Username     string `json:"username"`
	Domain       string `json:"domain"`
	Password     string `json:"password"`
	KeepPassword bool   `json:"keep_password"`
}

// SaveRDPCredentials сохраняет логин и (зашифрованный) пароль RDP туннеля.
func (a *App) SaveRDPCredentials(in RDPCredentialsInput) error {
	if err := a.ready(); err != nil {
		return err
	}
	in.Username = strings.TrimSpace(in.Username)
	in.Domain = strings.TrimSpace(in.Domain)
	for _, v := range []string{in.Username, in.Domain, in.Password} {
		if strings.ContainsAny(v, "\r\n\x00") {
			return errors.New("логин, домен и пароль RDP не могут содержать переводы строк")
		}
	}
	if in.Username == "" {
		return a.store.Tunnels.SetRDPCredentials(a.ctx, in.TunnelID, "", "", nil, false)
	}
	if in.Password == "" {
		return a.store.Tunnels.SetRDPCredentials(a.ctx, in.TunnelID, in.Username, in.Domain, nil, in.KeepPassword)
	}
	pw := []byte(in.Password)
	defer crypto.ZeroBytes(pw)
	enc, err := a.vault.Encrypt(pw)
	if err != nil {
		return fmt.Errorf("encrypt RDP password: %w", err)
	}
	return a.store.Tunnels.SetRDPCredentials(a.ctx, in.TunnelID, in.Username, in.Domain, enc, false)
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
