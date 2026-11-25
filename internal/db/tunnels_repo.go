package db

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type Tunnel struct {
	ID         string `json:"id"`
	HostID     string `json:"host_id"`
	Name       string `json:"name"`
	Direction  string `json:"direction"`
	LocalAddr  string `json:"local_addr"`
	RemoteAddr string `json:"remote_addr"`
	AutoStart  bool   `json:"auto_start"`

	// Учётные данные RDP: пароль наружу не отдаётся, только признак наличия.
	RDPUsername    string `json:"rdp_username"`
	RDPDomain      string `json:"rdp_domain"`
	HasRDPPassword bool   `json:"has_rdp_password"`
}

type TunnelsRepo struct {
	db *sql.DB
}

func (r *TunnelsRepo) Upsert(ctx context.Context, t *Tunnel) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}

	var remote sql.NullString
	if t.RemoteAddr != "" {
		remote = sql.NullString{String: t.RemoteAddr, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tunnels (id, host_id, name, direction, local_addr, remote_addr, auto_start)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			direction = excluded.direction,
			local_addr = excluded.local_addr,
			remote_addr = excluded.remote_addr,
			auto_start = excluded.auto_start`,
		t.ID, t.HostID, t.Name, t.Direction, t.LocalAddr, remote, boolToInt(t.AutoStart),
	)
	return err
}

func (r *TunnelsRepo) ListByHost(ctx context.Context, hostID string) ([]Tunnel, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, host_id, name, direction, local_addr, remote_addr, auto_start,
		       rdp_username, rdp_domain, length(coalesce(rdp_password_enc, '')) > 0
		FROM tunnels WHERE host_id = ? ORDER BY name ASC`, hostID)
	if err != nil {
		return nil, err
	}
	return scanTunnels(rows)
}

func (r *TunnelsRepo) ListAll(ctx context.Context) ([]Tunnel, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, host_id, name, direction, local_addr, remote_addr, auto_start,
		       rdp_username, rdp_domain, length(coalesce(rdp_password_enc, '')) > 0
		FROM tunnels ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	return scanTunnels(rows)
}

func (r *TunnelsRepo) GetByID(ctx context.Context, id string) (*Tunnel, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, host_id, name, direction, local_addr, remote_addr, auto_start,
		       rdp_username, rdp_domain, length(coalesce(rdp_password_enc, '')) > 0
		FROM tunnels WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	list, err := scanTunnels(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, sql.ErrNoRows
	}
	return &list[0], nil
}

func scanTunnels(rows *sql.Rows) ([]Tunnel, error) {
	defer rows.Close()

	var list []Tunnel
	for rows.Next() {
		var t Tunnel
		var remote sql.NullString
		var auto int
		if err := rows.Scan(&t.ID, &t.HostID, &t.Name, &t.Direction, &t.LocalAddr, &remote, &auto,
			&t.RDPUsername, &t.RDPDomain, &t.HasRDPPassword); err != nil {
			return nil, err
		}
		if remote.Valid {
			t.RemoteAddr = remote.String
		}
		t.AutoStart = auto == 1
		list = append(list, t)
	}
	return list, rows.Err()
}

// SetRDPCredentials сохраняет логин и домен; passwordEnc == nil при
// keepPassword оставляет прежний пароль, иначе заменяет (nil — удаляет).
func (r *TunnelsRepo) SetRDPCredentials(ctx context.Context, id, username, domain string, passwordEnc []byte, keepPassword bool) error {
	var res sql.Result
	var err error
	if keepPassword {
		res, err = r.db.ExecContext(ctx, `UPDATE tunnels SET rdp_username = ?, rdp_domain = ? WHERE id = ?`, username, domain, id)
	} else {
		res, err = r.db.ExecContext(ctx, `UPDATE tunnels SET rdp_username = ?, rdp_domain = ?, rdp_password_enc = ? WHERE id = ?`,
			username, domain, passwordEnc, id)
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RDPPasswordEnc возвращает зашифрованный пароль RDP (nil — не сохранён).
func (r *TunnelsRepo) RDPPasswordEnc(ctx context.Context, id string) ([]byte, error) {
	var enc []byte
	err := r.db.QueryRowContext(ctx, `SELECT rdp_password_enc FROM tunnels WHERE id = ?`, id).Scan(&enc)
	return enc, err
}

func (r *TunnelsRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tunnels WHERE id = ?`, id)
	return err
}
