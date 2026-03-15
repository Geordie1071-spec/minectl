package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type ProgressModel struct {
	Spinner  spinner.Model
	Message  string
	Start    time.Time
	QuitChan chan struct{}
}

type ProgressMsg string

type ProgressDone struct{}

func NewProgressModel(message string) ProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle
	return ProgressModel{
		Spinner: s,
		Message: message,
		Start:   time.Now(),
	}
}

func (m ProgressModel) Init() tea.Cmd {
	return m.Spinner.Tick
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case ProgressMsg:
		m.Message = string(msg)
		return m, nil
	case ProgressDone:
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.Spinner, cmd = m.Spinner.Update(msg)
	return m, cmd
}

func (m ProgressModel) View() string {
	elapsed := time.Since(m.Start).Round(time.Second)
	return fmt.Sprintf("  %s %s  (%s)\n", m.Spinner.View(), m.Message, elapsed)
}

func RunProgress(initialMsg string, work func(update func(string)) error) error {
	m := NewProgressModel(initialMsg)
	p := tea.NewProgram(m)
	errCh := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			p.Send(ProgressDone{})
			errCh <- err
		}()
		update := func(msg string) { p.Send(ProgressMsg(msg)) }
		err = work(update)
	}()
	if _, err := p.Run(); err != nil {
		return err
	}
	return <-errCh
}
