package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// PubKeyDirName — подкаталог рядом с vault.db, где лежат только публичные ключи
// (по ним системный ssh выбирает нужный ключ из ssh-agent).
const PubKeyDirName = "pubkeys"

type Store struct {
	DB      *sql.DB
	Keys    *KeysRepo
	Hosts   *HostsRepo
	Tunnels *TunnelsRepo
}

func DefaultDBPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config dir: %w", err)
	}
	appDir := filepath.Join(dir, "CrystalCoreSSHManager")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", fmt.Errorf("create app dir: %w", err)
	}
	pubKeyDir := filepath.Join(appDir, PubKeyDirName)
	if err := os.MkdirAll(pubKeyDir, 0700); err != nil {
		return "", fmt.Errorf("create pubkeys dir: %w", err)
	}
	return filepath.Join(appDir, "vault.db"), nil
}

func Open(dbPath string) (*Store, error) {
	// Без префикса "file:": иначе путь разбирается как URI и ломается на
	// Windows-путях с обратными слэшами, пробелами, '#' или '%'.
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := runMigrations(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{
		DB:      sqlDB,
		Keys:    &KeysRepo{db: sqlDB},
		Hosts:   &HostsRepo{db: sqlDB},
		Tunnels: &TunnelsRepo{db: sqlDB},
	}, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func runMigrations(db *sql.DB) error {
	var currentVersion int
	if err := db.QueryRow("PRAGMA user_version").Scan(&currentVersion); err != nil {
		return err
	}

	migrations := []struct {
		version int
		file    string
	}{
		{1, "migrations/001_init.sql"},
	}

	for _, m := range migrations {
		if currentVersion >= m.version {
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile(m.file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", m.file, err)
		}

		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("exec migration %s: %w", m.file, err)
		}

		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}
