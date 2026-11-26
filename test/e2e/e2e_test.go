//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "minectl-e2e-bin")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "minectl")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/minectl")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type env struct {
	t         *testing.T
	configDir string
	dataDir   string
	name      string
	port      int
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	e := &env{t: t, configDir: filepath.Join(root, "config")}
	if err := os.MkdirAll(e.configDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := fmt.Sprintf(`{"default_data_dir": %q, "default_backup_dir": %q}`, filepath.Join(root, "servers"), filepath.Join(root, "backups"))
	if err := os.WriteFile(filepath.Join(e.configDir, "config.json"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) run(args ...string) (string, error) {
	e.t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), "MINECTL_CONFIG_DIR="+e.configDir)
	out, err := cmd.CombinedOutput()
	e.t.Logf("$ minectl %s\n%s", strings.Join(args, " "), out)
	return string(out), err
}

func (e *env) mustRun(args ...string) string {
	e.t.Helper()
	out, err := e.run(args...)
	if err != nil {
		e.t.Fatalf("minectl %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func (e *env) rcon(command string) string {
	e.t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		out, err := e.run("exec", e.name, command)
		if err == nil {
			return strings.TrimSpace(out)
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("exec %q never succeeded: %v", command, err)
		}
		time.Sleep(2 * time.Second)
	}
}

type serverJSON struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	MCVersion string `json:"mc_version"`
	MemoryMB  int    `json:"memory_mb"`
	Backups   []struct {
		ID string `json:"id"`
	} `json:"backups"`
}

func (e *env) server() *serverJSON {
	e.t.Helper()
	var servers []serverJSON
	if err := json.Unmarshal([]byte(e.mustRun("list", "--json")), &servers); err != nil {
		e.t.Fatalf("parse list --json: %v", err)
	}
	for i := range servers {
		if servers[i].Name == e.name {
			return &servers[i]
		}
	}
	return nil
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestServerLifecycle(t *testing.T) {
	e := newEnv(t)
	e.dataDir = filepath.Join(t.TempDir(), "data")
	e.name = fmt.Sprintf("e2e-%d", rand.Intn(1_000_000))
	e.port = freePort(t)
	t.Cleanup(func() {
		_, _ = e.run("delete", e.name, "--purge")
		_ = exec.Command("docker", "rm", "-f", "minectl-"+e.name).Run()
	})

	if out := e.mustRun("versions", "paper"); !strings.Contains(out, "26.3") {
		t.Fatalf("paper versions missing 26.3:\n%s", out)
	}
	if out, err := e.run("create", "-n", e.name, "-v", "1.26.3"); err == nil || !strings.Contains(out, "Did you mean 26.3?") {
		t.Fatalf("expected a did-you-mean error for 1.26.3, got err=%v", err)
	}

	out := e.mustRun("create", "-n", e.name, "-t", "paper", "-v", "26.2", "-m", "1G", "--cpu", "2",
		"-p", fmt.Sprint(e.port), "--data-dir", e.dataDir)
	if !strings.Contains(out, "Ready in") {
		t.Fatalf("create did not report ready:\n%s", out)
	}
	if s := e.server(); s == nil || s.Status != "running" || s.MCVersion != "26.2" {
		t.Fatalf("after create: %+v", s)
	}

	t.Run("backup and restore", func(t *testing.T) {
		e.rcon("forceload add 0 0")
		e.rcon("setblock 0 100 0 minecraft:gold_block")
		e.rcon("save-all flush")

		out := e.mustRun("backup", "create", e.name)
		id := regexp.MustCompile(`bak_[0-9a-f]{8}`).FindString(out)
		if id == "" || strings.Contains(out, " 0 B") {
			t.Fatalf("bad backup output: %s", out)
		}

		e.rcon("setblock 0 100 0 minecraft:air")
		if got := e.rcon("execute if block 0 100 0 minecraft:air"); !strings.Contains(got, "passed") {
			t.Fatalf("block not cleared: %s", got)
		}

		e.mustRun("backup", "restore", e.name, id)
		e.rcon("forceload add 0 0")
		if got := e.rcon("execute if block 0 100 0 minecraft:gold_block"); !strings.Contains(got, "passed") {
			t.Fatalf("gold block not restored: %s", got)
		}
	})

	t.Run("status follows docker", func(t *testing.T) {
		if out, err := exec.Command("docker", "stop", "minectl-"+e.name).CombinedOutput(); err != nil {
			t.Fatalf("docker stop: %v\n%s", err, out)
		}
		if s := e.server(); s.Status != "stopped" {
			t.Fatalf("status after docker stop = %s, want stopped", s.Status)
		}
		if out := e.mustRun("start", e.name); !strings.Contains(out, "Ready in") {
			t.Fatalf("start did not report ready:\n%s", out)
		}
	})

	t.Run("logs and stats", func(t *testing.T) {
		if out := e.mustRun("logs", e.name, "--tail", "5"); strings.TrimSpace(out) == "" {
			t.Fatal("no log output")
		}
		if out := e.mustRun("stats", e.name); !strings.Contains(out, "CPU") {
			t.Fatalf("unexpected stats output: %s", out)
		}
	})

	t.Run("resize", func(t *testing.T) {
		e.mustRun("resize", e.name, "--memory", "1536M")
		if s := e.server(); s.MemoryMB != 1536 || s.Status != "running" {
			t.Fatalf("after resize: %+v", s)
		}
	})

	t.Run("upgrade", func(t *testing.T) {
		before := len(e.server().Backups)
		e.mustRun("upgrade", e.name, "-v", "26.3")
		s := e.server()
		if s.MCVersion != "26.3" || s.Status != "running" {
			t.Fatalf("after upgrade: %+v", s)
		}
		if len(s.Backups) != before+1 {
			t.Fatalf("expected a pre-upgrade backup, backups %d -> %d", before, len(s.Backups))
		}
	})

	t.Run("mods need a mod loader", func(t *testing.T) {
		if out, err := e.run("mods", "add", e.name, "lithium"); err == nil || !strings.Contains(out, "does not support mods") {
			t.Fatalf("expected mods add to fail on paper, err=%v", err)
		}
	})

	t.Run("delete purge", func(t *testing.T) {
		e.mustRun("delete", e.name, "--purge")
		if s := e.server(); s != nil {
			t.Fatalf("server still listed: %+v", s)
		}
		if _, err := os.Stat(e.dataDir); !os.IsNotExist(err) {
			t.Fatalf("data dir still exists: %v", err)
		}
	})
}

func TestDeleteAll(t *testing.T) {
	e := newEnv(t)
	base := t.TempDir()
	var names []string
	for i := 0; i < 2; i++ {
		name := fmt.Sprintf("e2e-all-%d-%d", rand.Intn(1_000_000), i)
		names = append(names, name)
		t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", "minectl-"+name).Run() })
		e.mustRun("create", "-n", name, "-t", "paper", "-v", "26.3", "--no-start",
			"-p", fmt.Sprint(freePort(t)), "--data-dir", filepath.Join(base, name))
	}

	if out, err := e.run("delete", "--all"); err == nil || !strings.Contains(out, "--yes") {
		t.Fatalf("delete --all without a terminal should refuse, err=%v", err)
	}
	var servers []serverJSON
	_ = json.Unmarshal([]byte(e.mustRun("list", "--json")), &servers)
	if len(servers) != 2 {
		t.Fatalf("servers deleted without confirmation: %d left", len(servers))
	}

	e.mustRun("delete", "--all", "--yes", "--purge")
	servers = nil
	_ = json.Unmarshal([]byte(e.mustRun("list", "--json")), &servers)
	if len(servers) != 0 {
		t.Fatalf("servers left after delete --all: %+v", servers)
	}
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(base, name)); !os.IsNotExist(err) {
			t.Errorf("data dir for %s still exists", name)
		}
		if err := exec.Command("docker", "inspect", "minectl-"+name).Run(); err == nil {
			t.Errorf("container minectl-%s still exists", name)
		}
	}
}
