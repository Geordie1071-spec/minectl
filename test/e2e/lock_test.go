//go:build e2e

package e2e

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// holdLock takes a server's lock the way a running operation does, by
// writing its row in the locks table, so lock checks can be tested without
// waiting for a real restart. The returned func releases it.
func holdLock(t *testing.T, configDir, server, holder string) func() {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(configDir, "minectl.db")+"?_pragma=busy_timeout(10000)")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_, err = db.Exec(`INSERT INTO locks (server, holder, token, acquired_at, expires_at) VALUES (?, ?, 'e2e', ?, ?)
		ON CONFLICT(server) DO UPDATE SET holder = excluded.holder, token = excluded.token,
		acquired_at = excluded.acquired_at, expires_at = excluded.expires_at`,
		server, holder, now.Format(time.RFC3339Nano), now.Add(time.Minute).Format(time.RFC3339Nano))
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	return func() {
		if _, err := db.Exec(`DELETE FROM locks WHERE server = ? AND token = 'e2e'`, server); err != nil {
			t.Errorf("release lock: %v", err)
		}
		db.Close()
	}
}
