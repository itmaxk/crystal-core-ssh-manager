package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Host struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Hostname        string     `json:"hostname"`
	Port            int        `json:"port"`
	Username        string     `json:"username"`
	KeyID           *string    `json:"key_id"`
	JumpHostID      *string    `json:"jump_host_id"`
	EnvTag          string     `json:"env_tag"`
	TabColor        string     `json:"tab_color"`
	IsTemporary     bool       `json:"is_temporary"`
	LastConnectedAt *time.Time `json:"last_connected_at" ts_type:"string"`
	CreatedAt       time.Time  `json:"created_at" ts_type:"string"`
}

type HostsRepo struct {
	db *sql.DB
}

func (r *HostsRepo) Upsert(ctx context.Context, h *Host) error {
	if h.ID == "" {
		h.ID = uuid.NewString()
	}
	if h.Port == 0 {
		h.Port = 22
	}
	if h.EnvTag == "" {
		h.EnvTag = "dev"
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO hosts (id, name, hostname, port, username, key_id, jump_host_id, env_tag, tab_color, is_temporary)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			hostname = excluded.hostname,
			port = excluded.port,
			username = excluded.username,
			key_id = excluded.key_id,
			jump_host_id = excluded.jump_host_id,
			env_tag = excluded.env_tag,
			tab_color = excluded.tab_color,
			is_temporary = excluded.is_temporary`,
		h.ID, h.Name, h.Hostname, h.Port, h.Username, h.KeyID, h.JumpHostID, h.EnvTag, h.TabColor, boolToInt(h.IsTemporary),
	)
	return err
}

func (r *HostsRepo) List(ctx context.Context) ([]Host, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, hostname, port, username, key_id, jump_host_id, env_tag, tab_color, is_temporary, last_connected_at, created_at
		FROM hosts
		ORDER BY is_temporary ASC, last_connected_at DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []Host
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (r *HostsRepo) GetByID(ctx context.Context, id string) (*Host, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, hostname, port, username, key_id, jump_host_id, env_tag, tab_color, is_temporary, last_connected_at, created_at
		FROM hosts WHERE id = ?`, id)
	h, err := scanHost(row)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *HostsRepo) TouchLastConnected(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE hosts SET last_connected_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (r *HostsRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM hosts WHERE id = ?`, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanHost(s scanner) (Host, error) {
	var h Host
	var keyID, jumpID sql.NullString
	var lastConn sql.NullTime
	var isTemp int

	err := s.Scan(
		&h.ID, &h.Name, &h.Hostname, &h.Port, &h.Username,
		&keyID, &jumpID, &h.EnvTag, &h.TabColor, &isTemp, &lastConn, &h.CreatedAt,
	)
	if err != nil {
		return h, err
	}
	if keyID.Valid {
		h.KeyID = &keyID.String
	}
	if jumpID.Valid {
		h.JumpHostID = &jumpID.String
	}
	if lastConn.Valid {
		h.LastConnectedAt = &lastConn.Time
	}
	h.IsTemporary = isTemp == 1
	return h, nil
}
