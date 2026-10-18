package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPConfig struct {
	Host               string
	Port               int
	User               string
	Password           string
	KeyPath            string
	BasePath           string
	HostKeyFingerprint string
}

// SFTP stores blobs on a remote SFTP server, reconnecting lazily when the
// underlying session drops.
type SFTP struct {
	cfg       SFTPConfig
	client    *sftp.Client
	sshClient *ssh.Client
	mu        sync.Mutex
}

func NewSFTP(cfg SFTPConfig) (*SFTP, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	s := &SFTP{cfg: cfg}
	if err := s.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to SFTP: %w", err)
	}
	return s, nil
}

func (s *SFTP) connect() error {
	var authMethods []ssh.AuthMethod

	if s.cfg.Password != "" {
		// 1. Standard Password Auth
		authMethods = append(authMethods, ssh.Password(s.cfg.Password))

		// 2. Keyboard-Interactive Auth (Handles PAM / Modern OpenSSH servers)
		authMethods = append(authMethods, ssh.KeyboardInteractive(
			func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = s.cfg.Password
				}
				return answers, nil
			},
		))
	}

	// 3. SSH Key Auth (if KeyPath is provided)
	if s.cfg.KeyPath != "" {
		keyPath := s.cfg.KeyPath
		if strings.HasPrefix(keyPath, "~/") {
			home, _ := os.UserHomeDir()
			keyPath = filepath.Join(home, keyPath[2:])
		}
		keyBytes, err := os.ReadFile(keyPath)
		if err == nil {
			signer, err := ssh.ParsePrivateKey(keyBytes)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if s.cfg.HostKeyFingerprint != "" {
		expected := s.cfg.HostKeyFingerprint
		if !strings.HasPrefix(expected, "SHA256:") {
			expected = "SHA256:" + expected
		}
		hostKeyCallback = func(_ string, _ net.Addr, key ssh.PublicKey) error {
			actual := ssh.FingerprintSHA256(key)
			if actual != expected {
				return fmt.Errorf("SFTP host key mismatch: got %s", actual)
			}
			return nil
		}
	}

	sshConfig := &ssh.ClientConfig{
		User:            s.cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	conn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return err
	}

	s.sshClient = conn
	s.client = client
	return nil
}

func (s *SFTP) ensureConnection() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		if _, err := s.client.Getwd(); err == nil {
			return nil
		}
	}
	return s.connect()
}

func (s *SFTP) remotePath(key string) string {
	cleanKey := filepath.ToSlash(key)
	if s.cfg.BasePath != "" {
		return path.Join(s.cfg.BasePath, cleanKey)
	}
	return cleanKey
}

func (s *SFTP) Save(_ context.Context, key string, r io.Reader) error {
	if err := s.ensureConnection(); err != nil {
		return err
	}

	dest := s.remotePath(key)
	dir := path.Dir(dest)

	if err := s.client.MkdirAll(dir); err != nil {
		return err
	}

	// Write to a temporary name and rename, so an interrupted upload never
	// leaves a truncated backup at the final key.
	tmp := fmt.Sprintf("%s.tmp-%d", dest, time.Now().UnixNano())
	f, err := s.client.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = s.client.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = s.client.Remove(tmp)
		return err
	}
	if err := s.client.PosixRename(tmp, dest); err == nil {
		return nil
	}
	_ = s.client.Remove(dest)
	if err := s.client.Rename(tmp, dest); err != nil {
		_ = s.client.Remove(tmp)
		return err
	}
	return nil
}

func (s *SFTP) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if err := s.ensureConnection(); err != nil {
		return nil, err
	}
	return s.client.Open(s.remotePath(key))
}

func (s *SFTP) Delete(_ context.Context, key string) error {
	if err := s.ensureConnection(); err != nil {
		return err
	}
	err := s.client.Remove(s.remotePath(key))
	if err == nil || errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
		return nil
	}
	return err
}

// Close releases the SFTP and SSH connections. It is safe to call more than
// once.
func (s *SFTP) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	if s.sshClient != nil {
		_ = s.sshClient.Close()
		s.sshClient = nil
	}
	return nil
}
