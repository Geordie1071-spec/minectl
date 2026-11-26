package store

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Geordie1071-spec/minectl/internal/domain"
)

func TestUsersSessionsTokens(t *testing.T) {
	useTempConfig(t)
	st := New()
	defer st.Close()

	admin, err := st.CreateFirstAdmin("owner", "h1")
	if err != nil || !admin.IsAdmin {
		t.Fatalf("first admin: %+v %v", admin, err)
	}
	if _, err := st.CreateFirstAdmin("intruder", "h2"); !errors.Is(err, ErrExists) {
		t.Fatalf("second first-admin should fail, got %v", err)
	}
	if _, err := st.CreateUser("OWNER", "h3", false); !errors.Is(err, ErrExists) {
		t.Fatalf("usernames should be case-insensitive unique, got %v", err)
	}
	friend, err := st.CreateUser("friend", "h4", false)
	if err != nil {
		t.Fatal(err)
	}
	u, hash, err := st.GetUserByName("Friend")
	if err != nil || u == nil || u.ID != friend.ID || hash != "h4" {
		t.Fatalf("lookup: %+v %q %v", u, hash, err)
	}
	if n, _ := st.CountAdmins(); n != 1 {
		t.Fatalf("admins = %d", n)
	}

	if err := st.CreateSession("live", friend.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession("old", friend.ID, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.SessionUser("live"); u == nil || u.ID != friend.ID {
		t.Fatal("live session not found")
	}
	if u, _ := st.SessionUser("old"); u != nil {
		t.Fatal("expired session accepted")
	}

	tok, err := st.CreateAPIToken(friend.ID, "bot", "th")
	if err != nil {
		t.Fatal(err)
	}
	if u, _ := st.APITokenUser("th"); u == nil || u.ID != friend.ID {
		t.Fatal("token user not found")
	}
	if list, _ := st.ListAPITokens(friend.ID); len(list) != 1 || list[0].LastUsedAt == nil {
		t.Fatalf("tokens = %+v", list)
	}
	if ok, _ := st.DeleteAPIToken(admin.ID, tok.ID); ok {
		t.Fatal("deleted another user's token")
	}

	if err := st.SetGrant(friend.ID, "survival", domain.RoleViewer); err != nil {
		t.Fatal(err)
	}
	if err := st.SetGrant(friend.ID, "survival", domain.RoleOperator); err != nil {
		t.Fatal(err)
	}
	if role, _ := st.GrantRole(friend.ID, "survival"); role != domain.RoleOperator {
		t.Fatalf("role = %q", role)
	}
	if err := st.SaveServer(&domain.Server{Name: "survival"}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteServer("survival"); err != nil {
		t.Fatal(err)
	}
	if role, _ := st.GrantRole(friend.ID, "survival"); role != "" {
		t.Fatal("grant survived server delete")
	}

	_ = st.SetGrant(friend.ID, "creative", domain.RoleViewer)
	if err := st.DeleteUser(friend.ID); err != nil {
		t.Fatal(err)
	}
	if u, _ := st.SessionUser("live"); u != nil {
		t.Fatal("session survived user delete")
	}
	if u, _ := st.APITokenUser("th"); u != nil {
		t.Fatal("token survived user delete")
	}
	if g, _ := st.ListGrants(friend.ID); len(g) != 0 {
		t.Fatal("grants survived user delete")
	}
}

func TestConcurrentFirstAdmin(t *testing.T) {
	useTempConfig(t)
	st := New()
	defer st.Close()
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := st.CreateFirstAdmin("admin"+string(rune('a'+i)), "h"); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if n, _ := st.CountUsers(); wins != 1 || n != 1 {
		t.Fatalf("wins=%d users=%d", wins, n)
	}
}

func TestJobsAndAudit(t *testing.T) {
	useTempConfig(t)
	st := New()
	defer st.Close()
	for i, name := range []string{"a", "b", "a"} {
		j := &domain.Job{ID: string(rune('1' + i)), Kind: "start", Server: name, Status: domain.JobRunning, CreatedAt: time.Now().Add(time.Duration(i) * time.Second)}
		if err := st.InsertJob(j); err != nil {
			t.Fatal(err)
		}
	}
	j := &domain.Job{ID: "1", Status: domain.JobSucceeded, Events: []domain.JobEvent{{Seq: 1, Type: "info", Message: "hi"}}}
	if err := st.FinishJob(j); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetJob("1")
	if got.Status != domain.JobSucceeded || len(got.Events) != 1 || got.FinishedAt == nil {
		t.Fatalf("job = %+v", got)
	}
	if list, _ := st.ListJobs("a", 10); len(list) != 2 || list[0].ID != "3" {
		t.Fatalf("jobs for a = %+v", list)
	}
	if list, _ := st.ListJobs("", 10); len(list) != 3 {
		t.Fatalf("all jobs = %d", len(list))
	}

	for i := 0; i < 5; i++ {
		_ = st.AddAudit(domain.AuditEntry{Username: "owner", Action: "server.start", Server: "a"})
	}
	page, _ := st.ListAudit(3, 0)
	if len(page) != 3 || page[0].ID != 5 {
		t.Fatalf("audit page = %+v", page)
	}
	if rest, _ := st.ListAudit(10, page[2].ID); len(rest) != 2 {
		t.Fatalf("audit rest = %d", len(rest))
	}
}
