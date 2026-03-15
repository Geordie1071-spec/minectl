package store

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/minectl/minectl/internal/config"
	"github.com/minectl/minectl/internal/domain"
)

type Store struct {
	mu sync.Mutex
}

type serversFile struct {
	Version int            `json:"version"`
	Servers []domain.Server `json:"servers"`
}

func New() *Store {
	return &Store{}
}

func (s *Store) ensureConfigDir() error {
	return os.MkdirAll(config.ConfigDir(), 0755)
}

func (s *Store) ListServers() ([]domain.Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(config.ServersPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var f serversFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Servers, nil
}

func (s *Store) GetServer(name string) (*domain.Server, error) {
	servers, err := s.ListServers()
	if err != nil {
		return nil, err
	}
	for i := range servers {
		if servers[i].Name == name {
			return &servers[i], nil
		}
	}
	return nil, nil
}

func (s *Store) GetServerByID(id string) (*domain.Server, error) {
	servers, err := s.ListServers()
	if err != nil {
		return nil, err
	}
	for i := range servers {
		if servers[i].ID == id {
			return &servers[i], nil
		}
	}
	return nil, nil
}

func (s *Store) SaveServer(server *domain.Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureConfigDir(); err != nil {
		return err
	}

	servers, err := s.readServersUnlocked()
	if err != nil {
		return err
	}

	found := false
	for i := range servers {
		if servers[i].Name == server.Name {
			servers[i] = *server
			found = true
			break
		}
	}
	if !found {
		servers = append(servers, *server)
	}

	return s.writeServersUnlocked(servers)
}

func (s *Store) readServersUnlocked() ([]domain.Server, error) {
	data, err := os.ReadFile(config.ServersPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f serversFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Servers == nil {
		return nil, nil
	}
	return f.Servers, nil
}

func (s *Store) writeServersUnlocked(servers []domain.Server) error {
	f := serversFile{Version: 1, Servers: servers}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.ServersPath(), data, 0644)
}

func (s *Store) DeleteServer(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	servers, err := s.readServersUnlocked()
	if err != nil {
		return err
	}

	var out []domain.Server
	for _, sv := range servers {
		if sv.Name != name {
			out = append(out, sv)
		}
	}
	return s.writeServersUnlocked(out)
}

func (s *Store) GetConfig() (*domain.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(config.ConfigFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return s.defaultConfig(), nil
		}
		return nil, err
	}
	var c domain.Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.DefaultMemoryMB == 0 {
		c.DefaultMemoryMB = config.DefaultMemoryMB
	}
	if c.DefaultDataDir == "" || c.DefaultDataDir == "/opt/minectl/servers" {
		c.DefaultDataDir = config.DefaultDataDir()
	}
	if c.DefaultBackupDir == "" || c.DefaultBackupDir == "/opt/minectl/backups" {
		c.DefaultBackupDir = config.DefaultBackupDir()
	}
	if c.BackupRetention == 0 {
		c.BackupRetention = config.DefaultBackupRetention
	}
	return &c, nil
}

func (s *Store) defaultConfig() *domain.Config {
	return &domain.Config{
		DefaultMemoryMB:         config.DefaultMemoryMB,
		DefaultDataDir:          config.DefaultDataDir(),
		DefaultBackupDir:        config.DefaultBackupDir(),
		BackupRetention:         config.DefaultBackupRetention,
		AutoBackupEnabled:       false,
		AutoBackupIntervalHours: 6,
	}
}

func (s *Store) SaveConfig(cfg *domain.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureConfigDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.ConfigFilePath(), data, 0644)
}

func InitConfigDir() error {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := config.ServersPath()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		f := serversFile{Version: 1, Servers: []domain.Server{}}
		data, _ := json.MarshalIndent(f, "", "  ")
		return os.WriteFile(p, data, 0644)
	}
	cfgPath := config.ConfigFilePath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfg := domain.Config{
			DefaultMemoryMB:         config.DefaultMemoryMB,
			DefaultDataDir:          config.DefaultDataDir(),
			DefaultBackupDir:        config.DefaultBackupDir(),
			BackupRetention:         config.DefaultBackupRetention,
			AutoBackupEnabled:       false,
			AutoBackupIntervalHours: 6,
		}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		return os.WriteFile(cfgPath, data, 0644)
	}
	return nil
}

