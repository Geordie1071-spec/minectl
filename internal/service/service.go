package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/docker"
	"github.com/Geordie1071-spec/minectl/internal/domain"
	"github.com/Geordie1071-spec/minectl/internal/store"
)

type Events interface {
	Progress(percent int, msg string)
	Stage(label string)
	Log(line string)
	Info(msg string)
	Warn(msg string)
}

type NopEvents struct{}

func (NopEvents) Progress(int, string) {}
func (NopEvents) Stage(string)         {}
func (NopEvents) Log(string)           {}
func (NopEvents) Info(string)          {}
func (NopEvents) Warn(string)          {}

type Manager struct {
	D      *docker.Client
	St     *store.Store
	Holder string
	Wait   bool
	// CLIHints adds command-line flag suggestions to messages.
	CLIHints bool
}

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid server name %q: use 1-32 lowercase letters, digits, or dashes, starting with a letter or digit", name)
	}
	return nil
}

func ValidateType(mcType string) error {
	switch mcType {
	case domain.TypeVanilla, domain.TypePaper, domain.TypeSpigot, domain.TypeFabric, domain.TypeForge, domain.TypeNeoForge, domain.TypeQuilt:
		return nil
	}
	return fmt.Errorf("invalid type %q; use: vanilla, paper, spigot, fabric, forge, neoforge, quilt", mcType)
}

func (m *Manager) lock(ctx context.Context, name, action string) (func(), error) {
	holder := action
	if m.Holder != "" {
		holder = m.Holder + ": " + action
	}
	return m.St.Lock(ctx, name, holder)
}

func (m *Manager) get(name string) (*domain.Server, error) {
	s, err := m.St.GetServer(name)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("server not found: %s", name)
	}
	return s, nil
}

type StartFailedError struct {
	Server  string
	Hint    string
	Summary []string
	LogPath string
	Cleanup string
}

func (e *StartFailedError) Error() string {
	msg := fmt.Sprintf("%s failed to start", e.Server)
	if e.Hint != "" {
		msg += ": " + e.Hint
	}
	if len(e.Summary) > 0 {
		msg += "\n\nKey log lines:\n  " + strings.Join(e.Summary, "\n  ")
	}
	if e.LogPath != "" {
		msg += "\n\nFull log: " + e.LogPath
	}
	if e.Cleanup != "" {
		msg += "\n\n" + e.Cleanup
	}
	return msg
}
