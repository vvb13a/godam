package service

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
	"golang.org/x/crypto/ssh"
)

func startTestSFTP(t *testing.T) (int, func()) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if meta.User() == "test" && string(pass) == "test" {
				return nil, nil
			}
			return nil, fmt.Errorf("invalid credentials")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveTestSSH(conn, config)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, func() { _ = listener.Close() }
}

func serveTestSSH(conn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range requests {
				if req.Type == "subsystem" && len(req.Payload) >= 4 && string(req.Payload[4:]) == "sftp" {
					_ = req.Reply(true, nil)
					server, err := sftp.NewServer(channel)
					if err != nil {
						return
					}
					_ = server.Serve()
					return
				}
				_ = req.Reply(false, nil)
			}
		}()
	}
}

func TestBackupSFTPFlow(t *testing.T) {
	port, stop := startTestSFTP(t)
	defer stop()

	remoteDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.StoragePath = t.TempDir()
	local, err := storage.NewLocal(cfg.StoragePath)
	if err != nil {
		t.Fatal(err)
	}
	registry := storage.NewRegistry("local")
	registry.Register("local", local)

	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := NewSecretBox(filepath.Join(t.TempDir(), "backup.key"))
	if err != nil {
		t.Fatal(err)
	}
	backup := NewBackupService(db, registry, secret)
	assets := NewAssetService(cfg, db, registry, local, NewCompositeExtractor(), NewPreviewGenerator(128, 85), NewTagService(db), backup)

	libraryID := uuid.NewString()
	now := time.Now().UTC()
	if err := db.Create(&store.Library{ID: libraryID, Name: "Backup test", Color: "8b5cf6", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := backup.SetSettings(context.Background(), libraryID, BackupSettingsRequest{
		Enabled:  true,
		Driver:   "sftp",
		Host:     "127.0.0.1",
		Port:     port,
		User:     "test",
		Password: "test",
		BasePath: remoteDir,
	}); err != nil {
		t.Fatal(err)
	}
	if err := backup.TestConnection(context.Background(), libraryID, BackupSettingsRequest{}); err != nil {
		t.Fatalf("test connection: %v", err)
	}

	content := []byte("hello from the backup test")
	asset, err := assets.Upload(context.Background(), "", libraryID, "", "hello.txt", "text/plain", int64(len(content)), bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	backup.processJobs(context.Background())

	remotePath := filepath.Join(remoteDir, filepath.FromSlash(backupKey(libraryID, asset.ID, asset.StorageKey)))
	got, err := os.ReadFile(remotePath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("backup content mismatch: %q", got)
	}

	status, err := backup.Status(context.Background(), libraryID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Synced != 1 || status.Total != 1 || status.Pending != 0 || status.Failed != 0 {
		t.Fatalf("unexpected status: %+v", status)
	}

	if err := assets.Delete(context.Background(), asset.ID); err != nil {
		t.Fatal(err)
	}
	backup.processJobs(context.Background())
	if _, err := os.Stat(remotePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backup still exists after delete: %v", err)
	}
}
