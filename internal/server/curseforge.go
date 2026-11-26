package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/config"
	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

const curseForgeSetupImageTag = "java21"

type CurseForgePack struct {
	Name        string
	PackVersion string
	MCType      string
	MCVersion   string
	Mods        int
}

type SetupError struct {
	Logs []string
}

func (e *SetupError) Error() string { return "modpack install failed" }

func DataDirFor(st *store.Store, name, override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	cfg, err := st.GetConfig()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg.DefaultDataDir, name), nil
}

func InstallCurseForgePack(ctx context.Context, d *docker.Client, st *store.Store, name, dataDir, pack, fileID string, onPercent func(int), onLine func(string)) (*CurseForgePack, error) {
	if existing, _ := st.GetServer(name); existing != nil && (existing.Status != domain.StatusCreating || existing.ContainerID != "") {
		return nil, fmt.Errorf("server already exists: %s", name)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, CurseForgeDownloadsDir), 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	imageRef := config.MinecraftImage + ":" + curseForgeSetupImageTag
	if err := ensureImageRef(ctx, d, imageRef, onPercent); err != nil {
		return nil, err
	}

	source := "curseforge"
	tmp := &domain.Server{MCType: domain.TypeFabric, MemoryMB: config.DefaultMemoryMB, ModpackSource: &source, ModpackID: &pack}
	if fileID != "" {
		tmp.ModpackVersion = &fileID
	}
	if id, err := d.ContainerByName(ctx, name+"-setup"); err == nil {
		_ = d.RemoveContainer(ctx, id)
	}
	id, err := d.CreateSetupContainer(ctx, name, imageRef, dataDir, BuildEnvVars(tmp))
	if err != nil {
		return nil, fmt.Errorf("create setup container: %w", err)
	}
	defer func() { _ = d.RemoveContainer(context.Background(), id) }()
	if err := d.StartContainer(ctx, id); err != nil {
		return nil, fmt.Errorf("start setup container: %w", err)
	}

	var recent []string
	lines := make(chan string, 100)
	go func() {
		_ = d.StreamLogs(ctx, id, true, lines)
		close(lines)
	}()
	for line := range lines {
		recent = append(recent, line)
		if len(recent) > keepLogLines {
			recent = recent[1:]
		}
		if onLine != nil {
			onLine(line)
		}
	}
	code, err := d.WaitContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, &SetupError{Logs: recent}
	}

	info, err := readInstallEnv(filepath.Join(dataDir, ".install-curseforge.env"))
	if err != nil {
		return nil, &SetupError{Logs: append(recent, "minectl: "+err.Error())}
	}
	p := &CurseForgePack{
		Name:        info["MODPACK_NAME"],
		PackVersion: info["MODPACK_VERSION"],
		MCType:      strings.ToLower(info["TYPE"]),
		MCVersion:   info["VERSION"],
		Mods:        countJars(filepath.Join(dataDir, "mods")),
	}
	switch p.MCType {
	case domain.TypeFabric, domain.TypeForge, domain.TypeNeoForge, domain.TypeQuilt:
	default:
		return nil, fmt.Errorf("modpack uses unsupported server type %q", info["TYPE"])
	}
	if p.MCVersion == "" {
		return nil, errors.New("modpack install did not report a Minecraft version")
	}
	return p, nil
}

func readInstallEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read install result: %w", err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if ok {
			out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return out, sc.Err()
}

func countJars(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jar") {
			n++
		}
	}
	return n
}
