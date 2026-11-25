package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"ssh-launcher/internal/crypto"
)

type UploadProgress struct {
	HostID     string `json:"host_id"`
	FileName   string `json:"file_name"`
	BytesSent  int64  `json:"bytes_sent"`
	TotalBytes int64  `json:"total_bytes"`
	Percent    int    `json:"percent"`
	RemotePath string `json:"remote_path,omitempty"`
	Done       bool   `json:"done"`
	Error      string `json:"error,omitempty"`
}

type UploadRequest struct {
	HostID          string
	Hostname        string
	Port            int
	Username        string
	PrivateKeyPEM   []byte
	Passphrase      []byte
	HostKeyCallback ssh.HostKeyCallback
	LocalPath       string
	RemoteDir       string
	Timeout         time.Duration
	OnProgress      func(UploadProgress)
}

func UploadFile(ctx context.Context, req UploadRequest) (string, error) {
	defer crypto.ZeroBytes(req.PrivateKeyPEM)
	defer crypto.ZeroBytes(req.Passphrase)

	localFile, err := os.Open(req.LocalPath)
	if err != nil {
		return "", fmt.Errorf("open local file: %w", err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return "", fmt.Errorf("stat local file: %w", err)
	}
	if stat.IsDir() {
		return "", errors.New("directory upload is not supported in quick dropzone")
	}

	signer, err := parseSigner(req.PrivateKeyPEM, req.Passphrase)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	addr := net.JoinHostPort(req.Hostname, strconv.Itoa(req.Port))
	sshCfg := &ssh.ClientConfig{
		User:              req.Username,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback:   req.HostKeyCallback,
		HostKeyAlgorithms: knownHostKeyAlgorithms(addr),
		Timeout:           timeout,
	}
	if sshCfg.HostKeyCallback == nil {
		sshCfg.HostKeyCallback = StrictHostKeyCallback()
	}

	sshClient, err := dialSSHContext(ctx, addr, sshCfg, timeout)
	if err != nil {
		return "", fmt.Errorf("ssh dial: %w", err)
	}
	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return "", fmt.Errorf("init sftp client: %w", err)
	}
	defer sftpClient.Close()

	remoteDir := req.RemoteDir
	if remoteDir == "" {
		remoteDir = "/tmp"
	}

	fileName := filepath.Base(req.LocalPath)
	remotePath := path.Join(remoteDir, fileName)

	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return "", fmt.Errorf("create remote file %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	reportErr := func(err error) {
		if req.OnProgress != nil {
			req.OnProgress(UploadProgress{
				HostID:     req.HostID,
				FileName:   fileName,
				TotalBytes: stat.Size(),
				Error:      err.Error(),
			})
		}
	}

	pr := &progressReader{
		ctx:      ctx,
		reader:   localFile,
		hostID:   req.HostID,
		fileName: fileName,
		total:    stat.Size(),
		onUpdate: req.OnProgress,
	}

	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(remoteFile, pr, buf); err != nil {
		if req.OnProgress != nil {
			req.OnProgress(UploadProgress{
				HostID:     req.HostID,
				FileName:   fileName,
				BytesSent:  pr.sent,
				TotalBytes: stat.Size(),
				Percent:    pr.lastPct,
				Error:      err.Error(),
			})
		}
		return "", fmt.Errorf("sftp copy: %w", err)
	}
	// Close на SFTP-файле сбрасывает последние записи — его ошибку нельзя терять.
	if err := remoteFile.Close(); err != nil {
		reportErr(err)
		return "", fmt.Errorf("finalize remote file: %w", err)
	}

	if req.OnProgress != nil {
		req.OnProgress(UploadProgress{
			HostID:     req.HostID,
			FileName:   fileName,
			BytesSent:  stat.Size(),
			TotalBytes: stat.Size(),
			Percent:    100,
			RemotePath: remotePath,
			Done:       true,
		})
	}

	return remotePath, nil
}

type progressReader struct {
	ctx      context.Context
	reader   io.Reader
	hostID   string
	fileName string
	total    int64
	sent     int64
	lastPct  int
	lastEmit time.Time
	onUpdate func(UploadProgress)
}

func (p *progressReader) Read(b []byte) (int, error) {
	if err := p.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := p.reader.Read(b)
	if n > 0 {
		p.sent += int64(n)
		p.emitIfNeeded()
	}
	return n, err
}

func (p *progressReader) emitIfNeeded() {
	if p.onUpdate == nil {
		return
	}
	pct := 100
	if p.total > 0 {
		pct = int((p.sent * 100) / p.total)
	}
	now := time.Now()
	if pct != p.lastPct && (now.Sub(p.lastEmit) >= 80*time.Millisecond || pct == 100) {
		p.lastPct = pct
		p.lastEmit = now
		p.onUpdate(UploadProgress{
			HostID:     p.hostID,
			FileName:   p.fileName,
			BytesSent:  p.sent,
			TotalBytes: p.total,
			Percent:    pct,
		})
	}
}
