package domain

import "time"

const (
	StatusCreating = "creating"
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusError    = "error"
	StatusDeleted  = "deleted"
)

const (
	TypeVanilla  = "vanilla"
	TypePaper    = "paper"
	TypeSpigot   = "spigot"
	TypeFabric   = "fabric"
	TypeForge    = "forge"
	TypeNeoForge = "neoforge"
	TypeQuilt    = "quilt"
)

type Server struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	ContainerID      string    `json:"container_id"`
	MCType           string    `json:"mc_type"`
	MCVersion        string    `json:"mc_version"`
	MemoryMB         int       `json:"memory_mb"`
	CPUCores         float64   `json:"cpu_cores"`
	Port             int       `json:"port"`
	Status           string    `json:"status"`
	ModLoader        string    `json:"mod_loader"`
	ModpackSource    *string   `json:"modpack_source,omitempty"`
	ModpackID        *string   `json:"modpack_id,omitempty"`
	ModpackVersion   *string   `json:"modpack_version,omitempty"`
	ModsLocked       bool      `json:"mods_locked"`
	JavaFlags        string    `json:"java_flags"`
	ImageTag         string    `json:"image_tag,omitempty"`
	DataDir          string    `json:"data_dir"`
	BackupDir        string    `json:"backup_dir"`
	CreatedAt        time.Time `json:"created_at"`
	LastStartedAt    time.Time `json:"last_started_at"`
	Mods             []Mod     `json:"mods,omitempty"`
	Backups          []Backup  `json:"backups,omitempty"`
	AutoBackupCron   string    `json:"auto_backup_cron,omitempty"`
	AutoBackupNextAt *time.Time `json:"auto_backup_next_at,omitempty"`
}

type Mod struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	ModID       string    `json:"mod_id"`
	ModName     string    `json:"mod_name"`
	VersionID   string    `json:"version_id"`
	DownloadURL string    `json:"download_url"`
	MCVersions  []string  `json:"mc_versions"`
	Loaders     []string  `json:"loaders"`
	Enabled     bool      `json:"enabled"`
	AddedAt     time.Time `json:"added_at"`
}

type Backup struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type Config struct {
	DefaultMemoryMB          int  `json:"default_memory_mb"`
	DefaultDataDir           string `json:"default_data_dir"`
	DefaultBackupDir         string `json:"default_backup_dir"`
	BackupRetention          int  `json:"backup_retention"`
	AutoBackupEnabled        bool `json:"auto_backup_enabled"`
	AutoBackupIntervalHours  int  `json:"auto_backup_interval_hours"`
}

func (s *Server) EnabledModURLs() []string {
	var urls []string
	for _, m := range s.Mods {
		if m.Enabled && m.DownloadURL != "" {
			urls = append(urls, m.DownloadURL)
		}
	}
	return urls
}

func (s *Server) ResolvedModpackURL() string {
	if s.ModpackSource == nil || *s.ModpackSource != "modrinth" || s.ModpackID == nil {
		return ""
	}
	id := *s.ModpackID
	ver := ""
	if s.ModpackVersion != nil {
		ver = *s.ModpackVersion
	}
	if ver != "" {
		return "https://api.modrinth.com/v2/project/" + id + "/version/" + ver
	}
	return "https://api.modrinth.com/v2/project/" + id
}
