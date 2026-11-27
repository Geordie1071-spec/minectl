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
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	ContainerID    string    `json:"container_id"`
	MCType         string    `json:"mc_type"`
	MCVersion      string    `json:"mc_version"`
	MemoryMB       int       `json:"memory_mb"`
	CPUCores       float64   `json:"cpu_cores"`
	Port           int       `json:"port"`
	Status         string    `json:"status"`
	ModLoader      string    `json:"mod_loader"`
	ModpackSource  *string   `json:"modpack_source,omitempty"`
	ModpackID      *string   `json:"modpack_id,omitempty"`
	ModpackVersion *string   `json:"modpack_version,omitempty"`
	ModsLocked     bool      `json:"mods_locked"`
	JavaFlags      string    `json:"java_flags"`
	ImageTag       string    `json:"image_tag,omitempty"`
	DataDir        string    `json:"data_dir"`
	BackupDir      string    `json:"backup_dir"`
	CreatedAt      time.Time `json:"created_at"`
	LastStartedAt  time.Time `json:"last_started_at"`
	Mods           []Mod     `json:"mods,omitempty"`
	Backups        []Backup  `json:"backups,omitempty"`
	// BackupSchedule is one of the Schedule* presets, or empty for none.
	BackupSchedule   string      `json:"auto_backup_cron,omitempty"`
	AutoBackupNextAt *time.Time  `json:"auto_backup_next_at,omitempty"`
	Properties       *Properties `json:"properties,omitempty"`
	LastCrashAt      *time.Time  `json:"last_crash_at,omitempty"`
	LastCrashReason  string      `json:"last_crash_reason,omitempty"`
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
	DefaultMemoryMB         int    `json:"default_memory_mb"`
	DefaultDataDir          string `json:"default_data_dir"`
	DefaultBackupDir        string `json:"default_backup_dir"`
	BackupRetention         int    `json:"backup_retention"`
	AutoBackupEnabled       bool   `json:"auto_backup_enabled"`
	AutoBackupIntervalHours int    `json:"auto_backup_interval_hours"`
	// DiscordWebhook receives crash and backup failure alerts from the panel.
	DiscordWebhook string `json:"discord_webhook,omitempty"`
}

const (
	ScheduleHourly = "hourly"
	Schedule6Hours = "6h"
	ScheduleDaily  = "daily"
)

// Properties are the server.properties settings minectl manages. Unset
// fields keep Minecraft's defaults.
type Properties struct {
	MOTD         string `json:"motd,omitempty"`
	MaxPlayers   int    `json:"max_players,omitempty"`
	Difficulty   string `json:"difficulty,omitempty"`
	Gamemode     string `json:"gamemode,omitempty"`
	ViewDistance int    `json:"view_distance,omitempty"`
	PVP          *bool  `json:"pvp,omitempty"`
	Hardcore     *bool  `json:"hardcore,omitempty"`
	OnlineMode   *bool  `json:"online_mode,omitempty"`
	Whitelist    *bool  `json:"whitelist,omitempty"`
	AllowFlight  *bool  `json:"allow_flight,omitempty"`
}

func (s *Server) EnabledModrinthProjects() []string {
	var projects []string
	for _, m := range s.Mods {
		if m.Enabled && m.Source == "modrinth" && m.ModID != "" {
			p := m.ModID
			if m.VersionID != "" {
				p += ":" + m.VersionID
			}
			projects = append(projects, p)
		}
	}
	return projects
}

func (s *Server) HasCurseForgeModpack() bool {
	return s.ModpackSource != nil && *s.ModpackSource == "curseforge" && s.ModpackID != nil && *s.ModpackID != ""
}

func (s *Server) HasModpack() bool {
	return s.HasModrinthModpack() || s.HasCurseForgeModpack()
}

func (s *Server) HasModrinthModpack() bool {
	return s.ModpackSource != nil && *s.ModpackSource == "modrinth" && s.ModpackID != nil && *s.ModpackID != ""
}
