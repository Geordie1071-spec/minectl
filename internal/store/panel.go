package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
)

var ErrExists = errors.New("already exists")

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func parseNullTS(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t := parseTS(s.String)
	return &t
}

func isUniqueErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

const userCols = `id, username, is_admin, created_at`

func scanUser(row interface{ Scan(...any) error }, extra ...any) (*domain.User, error) {
	var u domain.User
	var created string
	if err := row.Scan(append([]any{&u.ID, &u.Username, &u.IsAdmin, &created}, extra...)...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	u.CreatedAt = parseTS(created)
	return &u, nil
}

func (s *Store) CountUsers() (int, error) {
	db, err := s.open()
	if err != nil {
		return 0, err
	}
	var n int
	err = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CountAdmins() (int, error) {
	db, err := s.open()
	if err != nil {
		return 0, err
	}
	var n int
	err = db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(username, passwordHash string, admin bool) (*domain.User, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	res, err := db.Exec(`INSERT INTO users (username, password_hash, is_admin, created_at) VALUES (?, ?, ?, ?)`,
		username, passwordHash, admin, ts(now))
	if isUniqueErr(err) {
		return nil, ErrExists
	}
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &domain.User{ID: id, Username: username, IsAdmin: admin, CreatedAt: now.UTC()}, nil
}

// CreateFirstAdmin creates the admin account only if no users exist yet, so
// two racing setup requests can't both succeed.
func (s *Store) CreateFirstAdmin(username, passwordHash string) (*domain.User, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	res, err := db.Exec(`INSERT INTO users (username, password_hash, is_admin, created_at)
		SELECT ?, ?, 1, ? WHERE NOT EXISTS (SELECT 1 FROM users)`, username, passwordHash, ts(now))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrExists
	}
	id, _ := res.LastInsertId()
	return &domain.User{ID: id, Username: username, IsAdmin: true, CreatedAt: now.UTC()}, nil
}

func (s *Store) GetUser(id int64) (*domain.User, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	return scanUser(db.QueryRow(`SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// GetUserByName returns the user and their password hash.
func (s *Store) GetUserByName(username string) (*domain.User, string, error) {
	db, err := s.open()
	if err != nil {
		return nil, "", err
	}
	var hash string
	u, err := scanUser(db.QueryRow(`SELECT `+userCols+`, password_hash FROM users WHERE username = ?`, username), &hash)
	return u, hash, err
}

func (s *Store) ListUsers() ([]domain.User, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT ` + userCols + ` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (s *Store) SetUserPassword(id int64, passwordHash string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	return err
}

func (s *Store) SetUserAdmin(id int64, admin bool) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE users SET is_admin = ? WHERE id = ?`, admin, id)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) CreateSession(tokenHash string, userID int64, expires time.Time) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, ts(time.Now()), ts(expires))
	return err
}

func (s *Store) SessionUser(tokenHash string) (*domain.User, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	return scanUser(db.QueryRow(`SELECT u.id, u.username, u.is_admin, u.created_at FROM sessions s
		JOIN users u ON u.id = s.user_id WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, ts(time.Now())))
}

func (s *Store) DeleteSession(tokenHash string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteUserSessions logs a user out everywhere except the session in keep.
func (s *Store) DeleteUserSessions(userID int64, keep string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, keep)
	return err
}

func (s *Store) PruneSessions() error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, ts(time.Now()))
	return err
}

func (s *Store) CreateAPIToken(userID int64, name, tokenHash string) (*domain.APIToken, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	res, err := db.Exec(`INSERT INTO api_tokens (user_id, name, token_hash, created_at) VALUES (?, ?, ?, ?)`,
		userID, name, tokenHash, ts(now))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &domain.APIToken{ID: id, UserID: userID, Name: name, CreatedAt: now.UTC()}, nil
}

func (s *Store) ListAPITokens(userID int64) ([]domain.APIToken, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, user_id, name, created_at, last_used_at FROM api_tokens WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.APIToken{}
	for rows.Next() {
		var t domain.APIToken
		var created string
		var used sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &created, &used); err != nil {
			return nil, err
		}
		t.CreatedAt, t.LastUsedAt = parseTS(created), parseNullTS(used)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DeleteAPIToken(userID, id int64) (bool, error) {
	db, err := s.open()
	if err != nil {
		return false, err
	}
	res, err := db.Exec(`DELETE FROM api_tokens WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) APITokenUser(tokenHash string) (*domain.User, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	u, err := scanUser(db.QueryRow(`SELECT u.id, u.username, u.is_admin, u.created_at FROM api_tokens t
		JOIN users u ON u.id = t.user_id WHERE t.token_hash = ?`, tokenHash))
	if u != nil {
		_, _ = db.Exec(`UPDATE api_tokens SET last_used_at = ? WHERE token_hash = ?`, ts(time.Now()), tokenHash)
	}
	return u, err
}

func (s *Store) SetGrant(userID int64, server, role string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO server_grants (user_id, server, role) VALUES (?, ?, ?)
		ON CONFLICT(user_id, server) DO UPDATE SET role = excluded.role`, userID, server, role)
	return err
}

func (s *Store) DeleteGrant(userID int64, server string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM server_grants WHERE user_id = ? AND server = ?`, userID, server)
	return err
}

func (s *Store) ListGrants(userID int64) ([]domain.Grant, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT user_id, server, role FROM server_grants WHERE user_id = ? ORDER BY server`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Grant{}
	for rows.Next() {
		var g domain.Grant
		if err := rows.Scan(&g.UserID, &g.Server, &g.Role); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) GrantRole(userID int64, server string) (string, error) {
	db, err := s.open()
	if err != nil {
		return "", err
	}
	var role string
	err = db.QueryRow(`SELECT role FROM server_grants WHERE user_id = ? AND server = ?`, userID, server).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

func (s *Store) InsertJob(j *domain.Job) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO jobs (id, kind, server, user_id, status, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		j.ID, j.Kind, j.Server, j.UserID, j.Status, ts(j.CreatedAt))
	return err
}

func (s *Store) FinishJob(j *domain.Job) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	events, err := json.Marshal(j.Events)
	if err != nil {
		return err
	}
	finished := time.Now()
	if j.FinishedAt != nil {
		finished = *j.FinishedAt
	}
	_, err = db.Exec(`UPDATE jobs SET status = ?, error = ?, events = ?, finished_at = ? WHERE id = ?`,
		j.Status, j.Error, string(events), ts(finished), j.ID)
	return err
}

// FailRunningJobs marks jobs left running by a previous process as failed.
func (s *Store) FailRunningJobs(msg string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE jobs SET status = ?, error = ?, finished_at = ? WHERE status = ?`,
		domain.JobFailed, msg, ts(time.Now()), domain.JobRunning)
	return err
}

const jobCols = `id, kind, server, COALESCE(user_id, 0), status, error, events, created_at, finished_at`

func scanJob(row interface{ Scan(...any) error }, withEvents bool) (*domain.Job, error) {
	var j domain.Job
	var events, created string
	var finished sql.NullString
	if err := row.Scan(&j.ID, &j.Kind, &j.Server, &j.UserID, &j.Status, &j.Error, &events, &created, &finished); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	j.CreatedAt, j.FinishedAt = parseTS(created), parseNullTS(finished)
	if withEvents {
		_ = json.Unmarshal([]byte(events), &j.Events)
	}
	return &j, nil
}

func (s *Store) GetJob(id string) (*domain.Job, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	return scanJob(db.QueryRow(`SELECT `+jobCols+` FROM jobs WHERE id = ?`, id), true)
}

// ListJobs returns the newest jobs first, without their events. An empty
// server lists jobs for every server.
func (s *Store) ListJobs(server string, limit int) ([]domain.Job, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT `+jobCols+` FROM jobs WHERE ? = '' OR server = ? ORDER BY created_at DESC LIMIT ?`, server, server, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Job{}
	for rows.Next() {
		j, err := scanJob(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

func (s *Store) AddAudit(e domain.AuditEntry) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	var uid any
	if e.UserID != 0 {
		uid = e.UserID
	}
	_, err = db.Exec(`INSERT INTO audit_log (time, user_id, username, action, server, detail, ip) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ts(e.Time), uid, e.Username, e.Action, e.Server, e.Detail, e.IP)
	return err
}

// ListAudit returns entries newest first. A non-zero before pages backwards.
func (s *Store) ListAudit(limit int, before int64) ([]domain.AuditEntry, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, time, COALESCE(user_id, 0), username, action, server, detail, ip FROM audit_log
		WHERE ? = 0 OR id < ? ORDER BY id DESC LIMIT ?`, before, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AuditEntry{}
	for rows.Next() {
		var e domain.AuditEntry
		var t string
		if err := rows.Scan(&e.ID, &t, &e.UserID, &e.Username, &e.Action, &e.Server, &e.Detail, &e.IP); err != nil {
			return nil, err
		}
		e.Time = parseTS(t)
		out = append(out, e)
	}
	return out, rows.Err()
}

// AuditPage returns one page of the audit log, newest first, and how many
// entries there are in all.
func (s *Store) AuditPage(limit, offset int) ([]domain.AuditEntry, int, error) {
	db, err := s.open()
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.Query(`SELECT id, time, COALESCE(user_id, 0), username, action, server, detail, ip FROM audit_log
		ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.AuditEntry{}
	for rows.Next() {
		var e domain.AuditEntry
		var t string
		if err := rows.Scan(&e.ID, &t, &e.UserID, &e.Username, &e.Action, &e.Server, &e.Detail, &e.IP); err != nil {
			return nil, 0, err
		}
		e.Time = parseTS(t)
		out = append(out, e)
	}
	return out, total, rows.Err()
}
