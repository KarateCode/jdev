package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Jira issue structure
type Issue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary  string `json:"summary"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		FixVersions []struct {
			Name string `json:"name"`
		} `json:"fixVersions"`
	} `json:"fields"`
}

// Model holds the application state
type model struct {
	issues  []Issue
	cursor  int
	width   int
	height  int
	err     error
	loading bool
	spinner spinner.Model
}

// Styles
var (
	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")). // Cyan
			Bold(true)

	keySelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true).
				Background(lipgloss.Color("236"))

	summaryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")). // Light gray
			PaddingLeft(1)

	summarySelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")). // White
				PaddingLeft(1).
				Background(lipgloss.Color("236"))

	priorityStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")). // Orange
			PaddingLeft(1)

	prioritySelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")). // Orange
				PaddingLeft(1).
				Background(lipgloss.Color("236"))

	fixVersionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("183")). // Purple
			PaddingLeft(1)

	fixVersionSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("183")). // Purple
				PaddingLeft(1).
				Background(lipgloss.Color("236"))

	selectedIndicator = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")). // Pink
				Bold(true)
)

// Messages
type issuesMsg []Issue
type errMsg error
type viewFinishedMsg struct{ err error }

// Fetch issues from Jira CLI
func fetchIssues() tea.Msg {
	cmd := exec.Command("jira", "issue", "list", "-sDev Review", "--raw")
	output, err := cmd.Output()
	if err != nil {
		return errMsg(err)
	}

	var issues []Issue
	if err := json.Unmarshal(output, &issues); err != nil {
		return errMsg(err)
	}

	return issuesMsg(issues)
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		loading: true,
		spinner: s,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchIssues)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "ctrl+p", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "ctrl+n", "j":
			if m.cursor < len(m.issues)-1 {
				m.cursor++
			}
		case "r":
			// Refresh issues
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, fetchIssues)
		case "enter", "v":
			// View the selected issue
			if len(m.issues) > 0 {
				key := m.issues[m.cursor].Key
				cmd := exec.Command("jira", "issue", "view", key, "--comments", "50")
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					return viewFinishedMsg{err}
				})
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case issuesMsg:
		m.issues = msg
		m.loading = false

	case errMsg:
		m.err = msg
		m.loading = false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case viewFinishedMsg:
		// Return from viewing issue, could handle error if needed
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.loading {
		return m.spinner.View() + " Loading issues..."
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'r' to retry, 'q' to quit.", m.err)
	}

	if len(m.issues) == 0 {
		return "No issues found.\n\nPress 'r' to refresh, 'q' to quit."
	}

	var b strings.Builder

	for i, issue := range m.issues {
		isSelected := i == m.cursor

		// Truncate summary to fit narrow width
		summary := issue.Fields.Summary
		maxWidth := m.width - 3
		if maxWidth < 10 {
			maxWidth = 20
		}
		if len(summary) > maxWidth {
			summary = summary[:maxWidth-3] + "..."
		}

		priority := issue.Fields.Priority.Name

		// Join fix version names with ", "
		var versionNames []string
		for _, v := range issue.Fields.FixVersions {
			versionNames = append(versionNames, v.Name)
		}
		fixVersions := strings.Join(versionNames, ", ")

		if isSelected {
			b.WriteString(selectedIndicator.Render("> "))
			b.WriteString(keySelectedStyle.Render(issue.Key))
			b.WriteString("\n")
			b.WriteString("  ")
			b.WriteString(summarySelectedStyle.Render(summary))
			b.WriteString("\n")
			b.WriteString("  ")
			b.WriteString(prioritySelectedStyle.Render(priority))
			b.WriteString("\n")
			b.WriteString("  ")
			b.WriteString(fixVersionSelectedStyle.Render(fixVersions))
		} else {
			b.WriteString("  ")
			b.WriteString(keyStyle.Render(issue.Key))
			b.WriteString("\n")
			b.WriteString(summaryStyle.Render("  " + summary))
			b.WriteString("\n")
			b.WriteString(priorityStyle.Render("  " + priority))
			b.WriteString("\n")
			b.WriteString(fixVersionStyle.Render("  " + fixVersions))
		}
		b.WriteString("\n\n")
	}

	return b.String()
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
