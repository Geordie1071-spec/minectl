package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	Version               = "0.1.0"
	DefaultMemoryMB       = 2048
	DefaultBackupRetention = 7
	MinecraftImage        = "itzg/minecraft-server"
	MinecraftPort         = 25565
	ContainerNamePrefix   = "minectl-"
)

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
		return "java8"
	case minor == 17:
		return "java16"
	case minor <= 20:
		return "java17"
	default:
		return "java21"
	}
}

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

func ServersPath() string {
	return filepath.Join(ConfigDir(), "servers.json")
}

func ConfigFilePath() string {
	return filepath.Join(ConfigDir(), "config.json")
}
