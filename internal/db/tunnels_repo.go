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
		SELECT id, host_id, name, direction, local_addr, remote_addr, auto_start
		FROM tunnels WHERE host_id = ? ORDER BY name ASC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Tunnel
	for rows.Next() {
		var t Tunnel
		var remote sql.NullString
		var auto int
		if err := rows.Scan(&t.ID, &t.HostID, &t.Name, &t.Direction, &t.LocalAddr, &remote, &auto); err != nil {
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

func (r *TunnelsRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tunnels WHERE id = ?`, id)
	return err
}
