package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"

	"ssh-launcher/internal/db"
	"ssh-launcher/internal/ipc"
	sshsvc "ssh-launcher/internal/ssh"
)

type ConnectConfig struct {
	HostID     string
	SessionID  string
	Hostname   string
	Port       int
	Username   string
	PubKeyPath string
	JumpHost   string
}

// Run загружает хост из БД и запускает системный ssh в текущем терминале.
// Используется и отдельным ssh-launcher-cli.exe (Windows), и GUI-бинарником
// с флагом --connect (macOS / Linux).
func Run(hostID, sessionID string) int {
	dbPath, err := db.DefaultDBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve db path: %v\n", err)
		return 1
	}

	store, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		return 1
	}

	cfg, err := buildConfig(store, hostID, sessionID, filepath.Join(filepath.Dir(dbPath), db.PubKeyDirName))
	// БД не держим открытой на всё время SSH-сессии.
	_ = store.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return RunConnect(cfg)
}

func buildConfig(store *db.Store, hostID, sessionID, pubKeyDir string) (ConnectConfig, error) {
	ctx := context.Background()
	host, err := store.Hosts.GetByID(ctx, hostID)
	if err != nil {
		return ConnectConfig{}, fmt.Errorf("host %q not found: %w", hostID, err)
	}

	cfg := ConnectConfig{
		HostID:    host.ID,
		SessionID: sessionID,
		Hostname:  host.Hostname,
		Port:      host.Port,
		Username:  host.Username,
	}

	if host.KeyID != nil && *host.KeyID != "" {
		pubPath := filepath.Join(pubKeyDir, *host.KeyID+".pub")
		if _, statErr := os.Stat(pubPath); statErr == nil {
			cfg.PubKeyPath = pubPath
		}
	}

	if host.JumpHostID != nil && *host.JumpHostID != "" && *host.JumpHostID != host.ID {
		jump, err := store.Hosts.GetByID(ctx, *host.JumpHostID)
		if err != nil {
			return ConnectConfig{}, fmt.Errorf("jump host %q not found: %w", *host.JumpHostID, err)
		}
		if err := validateTarget(jump.Hostname, jump.Username, jump.Port); err != nil {
			return ConnectConfig{}, fmt.Errorf("jump host: %w", err)
		}
		cfg.JumpHost = fmt.Sprintf("%s@%s:%d", jump.Username, jump.Hostname, jump.Port)
	}
	return cfg, nil
}

func validateTarget(hostname, username string, port int) error {
	if err := sshsvc.ValidateHostname(hostname); err != nil {
		return err
	}
	if err := sshsvc.ValidateUsername(username); err != nil {
		return err
	}
	return sshsvc.ValidatePort(port)
}

// BuildSSHArgs формирует аргументы системного ssh. Цель передаётся после "--",
// чтобы её нельзя было интерпретировать как опцию.
func BuildSSHArgs(cfg ConnectConfig) ([]string, error) {
	if err := validateTarget(cfg.Hostname, cfg.Username, cfg.Port); err != nil {
		return nil, err
	}

	args := []string{
		"-p", strconv.Itoa(cfg.Port),
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
	}
	if cfg.PubKeyPath != "" {
		args = append(args, "-o", "IdentitiesOnly=yes", "-i", cfg.PubKeyPath)
	}
	if cfg.JumpHost != "" {
		args = append(args, "-J", cfg.JumpHost)
	}
	return append(args, "--", fmt.Sprintf("%s@%s", cfg.Username, cfg.Hostname)), nil
}

func RunConnect(cfg ConnectConfig) int {
	args, err := BuildSSHArgs(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crystal-core-ssh: %v\n", err)
		return 1
	}

	// Ctrl+C должен доходить до удалённой стороны через ssh, а не убивать обёртку.
	signal.Ignore(os.Interrupt)

	_ = ipc.Send(ipc.Message{
		Type:      ipc.MsgSessionStarted,
		SessionID: cfg.SessionID,
		HostID:    cfg.HostID,
		PID:       os.Getpid(),
	})

	exitCode := 0
	defer func() {
		_ = ipc.Send(ipc.Message{
			Type:      ipc.MsgSessionEnded,
			SessionID: cfg.SessionID,
			HostID:    cfg.HostID,
			ExitCode:  exitCode,
		})
	}()

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "crystal-core-ssh error: %v\n", err)
			exitCode = 1
		}
	}

	return exitCode
}
