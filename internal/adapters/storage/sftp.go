package storage

import (
	"context"
	"fmt"
	"io"
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
	Host     string
	Port     int
	User     string
	Password string
	KeyPath  string
	BasePath string
}

type SFTPStorage struct {
	cfg       SFTPConfig
	client    *sftp.Client
	sshClient *ssh.Client
	mu        sync.Mutex
}

func NewSFTPStorage(cfg SFTPConfig) (*SFTPStorage, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	s := &SFTPStorage{cfg: cfg}
	if err := s.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to SFTP: %w", err)
	}
	return s, nil
}

func (s *SFTPStorage) connect() error {
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

	sshConfig := &ssh.ClientConfig{
		User:            s.cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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

func (s *SFTPStorage) ensureConnection() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		if _, err := s.client.Getwd(); err == nil {
			return nil
		}
	}
	return s.connect()
}

func (s *SFTPStorage) remotePath(key string) string {
	cleanKey := filepath.ToSlash(key)
	if s.cfg.BasePath != "" {
		return path.Join(s.cfg.BasePath, cleanKey)
	}
	return cleanKey
}

func (s *SFTPStorage) Save(_ context.Context, key string, r io.Reader) error {
	if err := s.ensureConnection(); err != nil {
		return err
	}

	dest := s.remotePath(key)
	dir := path.Dir(dest)

	if err := s.client.MkdirAll(dir); err != nil {
		return err
	}

	f, err := s.client.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	return err
}

func (s *SFTPStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if err := s.ensureConnection(); err != nil {
		return nil, err
	}
	return s.client.Open(s.remotePath(key))
}

func (s *SFTPStorage) Delete(_ context.Context, key string) error {
	if err := s.ensureConnection(); err != nil {
		return err
	}
	return s.client.Remove(s.remotePath(key))
}
