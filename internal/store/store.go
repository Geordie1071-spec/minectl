package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	_ "modernc.org/sqlite"
)

const (
	lockTTL       = 2 * time.Minute
	lockHeartbeat = 30 * time.Second
)

var migrations = []string{
	`CREATE TABLE servers (
		name TEXT PRIMARY KEY,
		data TEXT NOT NULL
	);
	CREATE TABLE config (
		id   INTEGER PRIMARY KEY CHECK (id = 1),
		data TEXT NOT NULL
	);
	CREATE TABLE locks (
		server      TEXT PRIMARY KEY,
		holder      TEXT NOT NULL,
		token       TEXT NOT NULL,
		acquired_at TEXT NOT NULL,
		expires_at  TEXT NOT NULL
	);`,
	`CREATE TABLE users (
		id            INTEGER PRIMARY KEY,
		username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
		password_hash TEXT NOT NULL,
		is_admin      INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT NOT NULL
	);
	CREATE TABLE sessions (
		token_hash TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL
	);
	CREATE TABLE api_tokens (
		id           INTEGER PRIMARY KEY,
		user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name         TEXT NOT NULL,
		token_hash   TEXT NOT NULL UNIQUE,
		created_at   TEXT NOT NULL,
		last_used_at TEXT
	);
	CREATE TABLE server_grants (
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		server  TEXT NOT NULL,
		role    TEXT NOT NULL,
		PRIMARY KEY (user_id, server)
	);
	CREATE TABLE jobs (
		id          TEXT PRIMARY KEY,
		kind        TEXT NOT NULL,
		server      TEXT NOT NULL,
		user_id     INTEGER,
		status      TEXT NOT NULL,
		error       TEXT NOT NULL DEFAULT '',
		events      TEXT NOT NULL DEFAULT '[]',
		created_at  TEXT NOT NULL,
		finished_at TEXT
	);
	CREATE INDEX jobs_server ON jobs(server, created_at);
	CREATE TABLE audit_log (
		id       INTEGER PRIMARY KEY,
		time     TEXT NOT NULL,
		user_id  INTEGER,
		username TEXT NOT NULL,
		action   TEXT NOT NULL,
		server   TEXT NOT NULL DEFAULT '',
		detail   TEXT NOT NULL DEFAULT '',
		ip       TEXT NOT NULL DEFAULT ''
	);`,
}

type Store struct {
	once sync.Once
	db   *sql.DB
	err  error
}

type LockedError struct {
	Server string
	Holder string
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("%s is busy: %s", e.Server, e.Holder)
}

func New() *Store {
	return &Store{}
}

func Path() string {
	return filepath.Join(config.ConfigDir(), "minectl.db")
}

func (s *Store) open() (*sql.DB, error) {
	s.once.Do(func() {
		s.db, s.err = openDB(Path())
	})
	return s.db, s.err
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func openDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version >= len(migrations) {
		return nil
	}
	for i := version; i < len(migrations); i++ {
		if _, err := tx.Exec(migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	if version == 0 {
		if err := importJSON(tx); err != nil {
			return fmt.Errorf("import json state: %w", err)
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations))); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, p := range []string{legacyServersPath(), legacyConfigPath()} {
		if _, err := os.Stat(p); err == nil {
			_ = os.Rename(p, p+".migrated")
		}
	}
	return nil
}

func legacyServersPath() string { return filepath.Join(config.ConfigDir(), "servers.json") }
func legacyConfigPath() string  { return filepath.Join(config.ConfigDir(), "config.json") }

func importJSON(tx *sql.Tx) error {
	if data, err := os.ReadFile(legacyServersPath()); err == nil {
		var f struct {
			Servers []domain.Server `json:"servers"`
		}
		if err := json.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("servers.json: %w", err)
		}
		for i := range f.Servers {
			if err := upsertServer(tx, &f.Servers[i]); err != nil {
				return err
			}
		}
	}
	if data, err := os.ReadFile(legacyConfigPath()); err == nil {
		var c domain.Config
		if err := json.Unmarshal(data, &c); err != nil {
			return fmt.Errorf("config.json: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO config (id, data) VALUES (1, ?)`, string(data)); err != nil {
			return err
		}
	}
	return nil
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func upsertServer(db execer, s *domain.Server) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO servers (name, data) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET data = excluded.data`, s.Name, string(data))
	return err
}

func (s *Store) ListServers() ([]domain.Server, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT data FROM servers ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Server
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var sv domain.Server
		if err := json.Unmarshal([]byte(data), &sv); err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, rows.Err()
}

func (s *Store) GetServer(name string) (*domain.Server, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	var data string
	err = db.QueryRow(`SELECT data FROM servers WHERE name = ?`, name).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sv domain.Server
	if err := json.Unmarshal([]byte(data), &sv); err != nil {
		return nil, err
	}
	return &sv, nil
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
	db, err := s.open()
	if err != nil {
		return err
	}
	return upsertServer(db, server)
}

func (s *Store) DeleteServer(name string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	if _, err = db.Exec(`DELETE FROM servers WHERE name = ?`, name); err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM server_grants WHERE server = ?`, name)
	return err
}

func (s *Store) GetConfig() (*domain.Config, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	var data string
	err = db.QueryRow(`SELECT data FROM config WHERE id = 1`).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultConfig(), nil
	}
	if err != nil {
		return nil, err
	}
	var c domain.Config
	if err := json.Unmarshal([]byte(data), &c); err != nil {
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

func defaultConfig() *domain.Config {
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
	db, err := s.open()
	if err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO config (id, data) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data`, string(data))
	return err
}

func (s *Store) Lock(ctx context.Context, server, holder string) (release func(), err error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	token := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	now := time.Now().UTC()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var curHolder, expires string
	err = tx.QueryRow(`SELECT holder, expires_at FROM locks WHERE server = ?`, server).Scan(&curHolder, &expires)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return nil, err
	default:
		if exp, perr := time.Parse(time.RFC3339Nano, expires); perr == nil && exp.After(now) {
			return nil, &LockedError{Server: server, Holder: curHolder}
		}
	}
	_, err = tx.Exec(`INSERT INTO locks (server, holder, token, acquired_at, expires_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(server) DO UPDATE SET holder = excluded.holder, token = excluded.token,
		acquired_at = excluded.acquired_at, expires_at = excluded.expires_at`,
		server, holder, token, now.Format(time.RFC3339Nano), now.Add(lockTTL).Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(lockHeartbeat)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				_, _ = db.Exec(`UPDATE locks SET expires_at = ? WHERE server = ? AND token = ?`,
					time.Now().UTC().Add(lockTTL).Format(time.RFC3339Nano), server, token)
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(stop)
			<-done
			_, _ = db.Exec(`DELETE FROM locks WHERE server = ? AND token = ?`, server, token)
		})
	}, nil
}

func InitConfigDir() error {
	if err := os.MkdirAll(config.ConfigDir(), 0755); err != nil {
		return err
	}
	db, err := openDB(Path())
	if err != nil {
		return err
	}
	return db.Close()
}
