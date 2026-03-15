package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// Version is the minectl CLI version
	Version = "0.1.0"

	// DefaultMemoryMB is the default RAM allocation per server
	DefaultMemoryMB = 2048
	// DefaultBackupRetention is how many backups to keep
	DefaultBackupRetention = 7
	// MinecraftImage is the Docker image for MC servers
	MinecraftImage = "itzg/minecraft-server"
	// MinecraftPort is the default MC server port
	MinecraftPort = 25565
	// ContainerNamePrefix is prepended to all minectl containers
	ContainerNamePrefix = "minectl-"
)

// ImageTagForMCVersion returns the itzg/minecraft-server image tag (Java version) for the given MC version and type.
// Picks the correct Java so any modpack works: 1.12–1.16 → java8, 1.17 → java16, 1.18–1.20 → java17, 1.21+ → java21.
func ImageTagForMCVersion(mcVersion, mcType string) string {
	var major, minor int
	_, _ = fmt.Sscanf(mcVersion, "%d.%d", &major, &minor)
	if major == 0 {
		return "latest"
	}
	if major != 1 {
		return "java17"
	}
	switch {
	case minor <= 16:
		return "java8"  // Forge 1.12–1.16 (LaunchWrapper)
	case minor == 17:
		return "java16"
	case minor <= 20:
		return "java17"
	default:
		return "java21" // 1.21+
	}
}

// DefaultDataDir returns the default base path for server data (OS-specific).
// On Windows uses %USERPROFILE%\minectl\servers; on Linux /opt/minectl/servers.
func DefaultDataDir() string {
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		if home == "" {
			home = "."
		}
		return filepath.Join(home, "minectl", "servers")
	}
	return "/opt/minectl/servers"
}

// DefaultBackupDir returns the default base path for backups (OS-specific).
func DefaultBackupDir() string {
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		if home == "" {
			home = "."
		}
		return filepath.Join(home, "minectl", "backups")
	}
	return "/opt/minectl/backups"
}

// ConfigDir returns the minectl config directory.
// Respects MINECTL_CONFIG_DIR env var, else ~/.minectl (or $HOME/.minectl on Windows).
func ConfigDir() string {
	if d := os.Getenv("MINECTL_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".minectl")
}

// ServersPath returns the path to servers.json
func ServersPath() string {
	return filepath.Join(ConfigDir(), "servers.json")
}

// ConfigFilePath returns the path to config.json
func ConfigFilePath() string {
	return filepath.Join(ConfigDir(), "config.json")
}
