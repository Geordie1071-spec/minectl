package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConsoleModel is the interactive server console (log view + command input)
type ConsoleModel struct {
	Viewport   viewport.Model
	Input      textinput.Model
	LogCh      <-chan string
	SendCmd    func(string)
	History    []string
	HistoryIdx int
	LogLines   []string
	Ready      bool
	Title      string
	InputFocused bool // true = typing in input; false = scrolling viewport
}

type logLineMsg string

func NewConsoleModel(title string, logCh <-chan string, sendCmd func(string)) ConsoleModel {
	vp := viewport.New(80, 20)
	vp.Style = PanelStyle
	ti := textinput.New()
	ti.Placeholder = "Enter command..."
	ti.CharLimit = 256
	return ConsoleModel{
		Viewport:     vp,
		Input:        ti,
		LogCh:        logCh,
		SendCmd:      sendCmd,
		Title:        title,
		History:      []string{},
		HistoryIdx:   -1,
		InputFocused: true,
	}
}

func (m ConsoleModel) Init() tea.Cmd {
	return tea.Batch(
		m.Input.Focus(),
		textinput.Blink,
		m.waitForLogLine,
	)
}

func (m ConsoleModel) waitForLogLine() tea.Msg {
	if m.LogCh == nil {
		return nil
	}
	select {
	case line := <-m.LogCh:
		return logLineMsg(line)
	default:
		return nil
	}
}

func (m ConsoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		// Tab: switch focus between input and viewport
		if key == "tab" {
			m.InputFocused = !m.InputFocused
			if m.InputFocused {
				return m, m.Input.Focus()
			}
			m.Input.Blur()
			return m, nil
		}
		if m.InputFocused {
			// All keys go to input only
			switch key {
			case "enter":
				cmd := strings.TrimSpace(m.Input.Value())
				if cmd != "" {
					if m.SendCmd != nil {
						m.SendCmd(cmd)
					}
					m.History = append(m.History, cmd)
					m.HistoryIdx = len(m.History)
					m.Input.SetValue("")
				}
				var tiCmd tea.Cmd
				m.Input, tiCmd = m.Input.Update(msg)
				return m, tiCmd
			case "up":
				if len(m.History) > 0 {
					if m.HistoryIdx <= 0 {
						m.HistoryIdx = 0
					} else {
						m.HistoryIdx--
					}
					m.Input.SetValue(m.History[m.HistoryIdx])
				}
				return m, nil
			case "down":
				if m.HistoryIdx >= len(m.History)-1 {
					m.HistoryIdx = len(m.History)
					m.Input.SetValue("")
				} else {
					m.HistoryIdx++
					m.Input.SetValue(m.History[m.HistoryIdx])
				}
				return m, nil
			}
			var tiCmd tea.Cmd
			m.Input, tiCmd = m.Input.Update(msg)
			return m, tiCmd
		}
		// Viewport focused: only viewport gets keys (scroll)
		var vpCmd tea.Cmd
		m.Viewport, vpCmd = m.Viewport.Update(msg)
		return m, vpCmd
	case logLineMsg:
		m.LogLines = append(m.LogLines, string(msg))
		m.Viewport.SetContent(strings.Join(m.LogLines, "\n"))
		m.Viewport.GotoBottom()
		return m, m.waitForLogLine
	}

	// Non-key messages: only update focused component
	if m.InputFocused {
		var tiCmd tea.Cmd
		m.Input, tiCmd = m.Input.Update(msg)
		return m, tiCmd
	}
	var vpCmd tea.Cmd
	m.Viewport, vpCmd = m.Viewport.Update(msg)
	return m, vpCmd
}

func (m ConsoleModel) View() string {
	title := TitleStyle.Render("minectl console — " + m.Title)
	logArea := PanelStyle.Render(m.Viewport.View())
	hint := "Tab: scroll log | "
	if m.InputFocused {
		hint += "type here, Enter to send"
	} else {
		hint += "Tab: back to input"
	}
	inputArea := PanelStyle.Render("command (" + hint + ")\n" + m.Input.View())
	return lipgloss.JoinVertical(lipgloss.Left, title, logArea, inputArea)
}

// RunConsole runs the console TUI; ctx can be used to cancel (e.g. when logs stop)
func RunConsole(ctx context.Context, title string, logCh <-chan string, sendCmd func(string)) error {
	p := tea.NewProgram(NewConsoleModel(title, logCh, sendCmd))
	_, err := p.Run()
	_ = ctx
	return err
}
