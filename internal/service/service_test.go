package service

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"survival", "e2e-123", "a", "bettermc-2", strings.Repeat("a", 32)} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("ValidateName(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Survival", "-lead", "has space", "../etc", "a_b", "x/y", strings.Repeat("a", 33)} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) should fail", bad)
		}
	}
}

func TestStartFailedErrorMessage(t *testing.T) {
	err := &StartFailedError{
		Server:  "backrooms",
		Hint:    `the mod "spb-revamped" uses client-only code`,
		Summary: []string{"[main/ERROR]: Minecraft has crashed!"},
		LogPath: "/srv/backrooms/logs/latest.log",
		Cleanup: "Stopped backrooms so it doesn't keep restarting.",
	}
	msg := err.Error()
	for _, want := range []string{"backrooms failed to start: the mod", "Key log lines:\n  [main/ERROR]", "Full log: /srv/backrooms/logs/latest.log", "Stopped backrooms"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestChoosePort(t *testing.T) {
	t.Setenv("MINECTL_CONFIG_DIR", t.TempDir())
	st := store.New()
	defer st.Close()
	m := &Manager{St: st}
	ctx := context.Background()

	// Hold a port the way another program would, and start the search
	// just below it so the test doesn't depend on 25565 being free.
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	held := l.Addr().(*net.TCPAddr).Port

	if _, _, err := m.choosePort(ctx, held); err == nil || !strings.Contains(err.Error(), "another program") {
		t.Fatalf("requested port held by another program: %v", err)
	}
	if err := st.SaveServer(&domain.Server{Name: "taken", Port: held + 1}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.choosePort(ctx, held+1); err == nil || !strings.Contains(err.Error(), "minectl server taken") {
		t.Fatalf("requested port used by a server: %v", err)
	}

	// Creates running at the same time get different ports until released.
	var got []int
	var releases []func()
	for i := 0; i < 3; i++ {
		p, release, err := m.choosePort(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
		releases = append(releases, release)
	}
	if got[0] == got[1] || got[1] == got[2] || got[0] == got[2] {
		t.Fatalf("concurrent creates got overlapping ports %v", got)
	}
	for _, r := range releases {
		r()
	}
	if p, release, _ := m.choosePort(ctx, 0); p != got[0] {
		t.Fatalf("after release, want %d again, got %d", got[0], p)
	} else {
		release()
	}
}
