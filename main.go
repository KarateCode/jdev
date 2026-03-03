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
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

// Filter types
type filterType int

const (
	filterDevReview filterType = iota
	filterAssignedToMe
	filterReleaseTasks
)

func (f filterType) title() string {
	switch f {
	case filterDevReview:
		return "Dev Review"
	case filterAssignedToMe:
		return "Issues Assigned to Me"
	case filterReleaseTasks:
		return "Release Tasks"
	default:
		return "Issues"
	}
}

// Model holds the application state
type model struct {
	issues    []Issue
	cursor    int
	width     int
	height    int
	err       error
	loading   bool
	spinner   spinner.Model
	showHelp  bool
	tableView bool
	filter    filterType
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

	helpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(1, 2).
			Background(lipgloss.Color("235"))

	helpTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true).
			MarginBottom(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Title style
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true).
			MarginBottom(1)

	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Bold(true).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240"))

	tableRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	tableRowSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("236"))

	tableCellKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	tableCellKeySelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("86")).
					Bold(true).
					Background(lipgloss.Color("236"))

	tableCellPriorityStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

	tableCellPrioritySelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("214")).
					Background(lipgloss.Color("236"))

	tableCellStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("114")) // Green

	tableCellStatusSelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("114")).
					Background(lipgloss.Color("236"))

	tableCellFixVersionStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("183"))

	tableCellFixVersionSelectedStyle = lipgloss.NewStyle().
						Foreground(lipgloss.Color("183")).
						Background(lipgloss.Color("236"))

	devOpsIconStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("75")) // Blue
)

// Get icon for issue type
func getIssueTypeIcon(issueType string) string {
	switch issueType {
	case "Story":
		return "📖"
	case "Bug":
		return "🐛"
	case "DevOps":
		return devOpsIconStyle.Render("</>  ")
	default:
		return issueType
	}
}

// Messages
type issuesMsg []Issue
type errMsg error
type viewFinishedMsg struct{ err error }

// Fetch issues from Jira CLI
func fetchIssues(filter filterType) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch filter {
		case filterAssignedToMe:
			// Get current user
			meCmd := exec.Command("jira", "me")
			meOutput, err := meCmd.Output()
			if err != nil {
				return errMsg(err)
			}
			me := strings.TrimSpace(string(meOutput))
			cmd = exec.Command("jira", "issue", "list", "-a"+me, "--raw")
		case filterReleaseTasks:
			cmd = exec.Command("jira", "issue", "list", "-q", "issuetype = 'Release Task' AND resolution = Unresolved", "--raw")
		default:
			cmd = exec.Command("jira", "issue", "list", "-sDev Review", "--raw")
		}

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
	return tea.Batch(m.spinner.Tick, fetchIssues(m.filter))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle help modal toggle
		if m.showHelp {
			switch msg.String() {
			case "?", "esc", "ctrl+g":
				m.showHelp = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "?":
			m.showHelp = true
			return m, nil
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
		case "t":
			// Toggle between list and table view
			m.tableView = !m.tableView
			return m, nil
		case "f":
			// Cycle through filters
			switch m.filter {
			case filterDevReview:
				m.filter = filterAssignedToMe
			case filterAssignedToMe:
				m.filter = filterReleaseTasks
			case filterReleaseTasks:
				m.filter = filterDevReview
			}
			m.loading = true
			m.cursor = 0
			return m, tea.Batch(m.spinner.Tick, fetchIssues(m.filter))
		case "r":
			// Refresh issues
			m.loading = true
			return m, tea.Batch(m.spinner.Tick, fetchIssues(m.filter))
		case "enter", "v":
			// View the selected issue
			if len(m.issues) > 0 {
				key := m.issues[m.cursor].Key
				if os.Getenv("TMUX") != "" {
					// Running inside tmux, use popup
					cmd := exec.Command("tmux", "display-popup", "-E", "-w", "80%", "-h", "80%", "-x", "10", "-y", "5",
						fmt.Sprintf("jira issue view %s --comments 50", key))
					cmd.Run()
					return m, nil
				}
				// Fallback: suspend TUI and run directly
				cmd := exec.Command("jira", "issue", "view", key, "--comments", "50")
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					return viewFinishedMsg{err}
				})
			}
		case "m":
			// Move the selected issue
			if len(m.issues) > 0 {
				key := m.issues[m.cursor].Key
				cmd := exec.Command("jira", "issue", "move", key)
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					return viewFinishedMsg{err}
				})
			}
		case "o":
			// Open the selected issue in browser
			if len(m.issues) > 0 {
				key := m.issues[m.cursor].Key
				url := fmt.Sprintf("https://envoyplatform.atlassian.net/jira/software/c/projects/EP/boards/35?selectedIssue=%s", key)
				exec.Command("open", url).Run()
				return m, nil
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

func (m model) renderListView() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(m.filter.title()))
	b.WriteString("\n\n")

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

		// Get issue type icon
		issueTypeIcon := getIssueTypeIcon(issue.Fields.IssueType.Name)

		if isSelected {
			// Pad lines to full width for background highlight
			lineWidth := m.width
			if lineWidth < 10 {
				lineWidth = 40
			}
			keyLine := fmt.Sprintf("%-*s", lineWidth, "  "+issueTypeIcon+" "+issue.Key)
			summaryLine := fmt.Sprintf("%-*s", lineWidth, "  "+summary)
			priorityLine := fmt.Sprintf("%-*s", lineWidth, "  "+priority)
			fixVersionLine := fmt.Sprintf("%-*s", lineWidth, "  "+fixVersions)

			b.WriteString(keySelectedStyle.Render(keyLine))
			b.WriteString("\n")
			b.WriteString(summarySelectedStyle.Render(summaryLine))
			b.WriteString("\n")
			b.WriteString(prioritySelectedStyle.Render(priorityLine))
			b.WriteString("\n")
			b.WriteString(fixVersionSelectedStyle.Render(fixVersionLine))
		} else {
			b.WriteString("  ")
			b.WriteString(issueTypeIcon + " ")
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

func (m model) renderTableView() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(m.filter.title()))
	b.WriteString("\n\n")

	// Define column widths
	typeCol := 4
	keyCol := 12
	statusCol := 12
	priorityCol := 10
	versionCol := 15
	// Summary takes remaining space
	summaryCol := m.width - typeCol - keyCol - statusCol - priorityCol - versionCol - 7 // 7 for spacing
	if summaryCol < 20 {
		summaryCol = 20
	}

	// Header
	header := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*s",
		typeCol, "Type ",
		keyCol, "Key",
		summaryCol, "Summary",
		statusCol, "Status",
		priorityCol, "Priority",
		versionCol, "Version")
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")

	// Rows
	for i, issue := range m.issues {
		isSelected := i == m.cursor

		// Get issue type icon
		issueTypeIcon := getIssueTypeIcon(issue.Fields.IssueType.Name)

		// Truncate summary
		summary := issue.Fields.Summary
		if len(summary) > summaryCol {
			summary = summary[:summaryCol-3] + "..."
		}

		priority := issue.Fields.Priority.Name
		status := issue.Fields.Status.Name
		if len(status) > statusCol {
			status = status[:statusCol-3] + "..."
		}

		// Join fix version names
		var versionNames []string
		for _, v := range issue.Fields.FixVersions {
			versionNames = append(versionNames, v.Name)
		}
		fixVersions := strings.Join(versionNames, ", ")
		if len(fixVersions) > versionCol {
			fixVersions = fixVersions[:versionCol-3] + "..."
		}

		if isSelected {
			// Build selected row with background
			lineWidth := m.width
			if lineWidth < 10 {
				lineWidth = 80
			}

			typeCell := fmt.Sprintf(" %-*s", typeCol, issueTypeIcon)
			keyCell := fmt.Sprintf("%-*s", keyCol, issue.Key)
			summaryCell := fmt.Sprintf("%-*s", summaryCol, summary)
			statusCell := fmt.Sprintf("%-*s", statusCol, status)
			priorityCell := fmt.Sprintf("%-*s", priorityCol, priority)
			versionCell := fmt.Sprintf("%-*s", versionCol, fixVersions)

			row := tableRowSelectedStyle.Render(typeCell) +
				tableCellKeySelectedStyle.Render(" "+keyCell) +
				tableRowSelectedStyle.Render(" "+summaryCell) +
				tableCellStatusSelectedStyle.Render(" "+statusCell) +
				tableCellPrioritySelectedStyle.Render(" "+priorityCell) +
				tableCellFixVersionSelectedStyle.Render(" "+versionCell)

			// Pad to full width
			rowWidth := lipgloss.Width(row)
			if rowWidth < lineWidth {
				row += tableRowSelectedStyle.Render(strings.Repeat(" ", lineWidth-rowWidth))
			}

			b.WriteString(row)
		} else {
			typeCell := fmt.Sprintf(" %-*s", typeCol, issueTypeIcon)
			keyCell := fmt.Sprintf("%-*s", keyCol, issue.Key)
			summaryCell := fmt.Sprintf("%-*s", summaryCol, summary)
			statusCell := fmt.Sprintf("%-*s", statusCol, status)
			priorityCell := fmt.Sprintf("%-*s", priorityCol, priority)
			versionCell := fmt.Sprintf("%-*s", versionCol, fixVersions)

			row := tableRowStyle.Render(typeCell) +
				tableCellKeyStyle.Render(" "+keyCell) +
				tableRowStyle.Render(" "+summaryCell) +
				tableCellStatusStyle.Render(" "+statusCell) +
				tableCellPriorityStyle.Render(" "+priorityCell) +
				tableCellFixVersionStyle.Render(" "+versionCell)

			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	return b.String()
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

	var content string
	if m.tableView {
		content = m.renderTableView()
	} else {
		content = m.renderListView()
	}

	// Overlay help modal if active
	if m.showHelp {
		bg := lipgloss.Color("235")

		titleStyle := helpTitleStyle.Background(bg)
		keyStyle := helpKeyStyle.Background(bg)
		descStyle := helpDescStyle.Background(bg)

		// Build each line with consistent width
		lines := []string{
			titleStyle.Render("Keyboard Shortcuts"),
			"",
			keyStyle.Render("j/↓/ctrl+n") + descStyle.Render("  Move down      "),
			keyStyle.Render("k/↑/ctrl+p") + descStyle.Render("  Move up        "),
			keyStyle.Render("v/enter   ") + descStyle.Render("  View issue     "),
			keyStyle.Render("o         ") + descStyle.Render("  Open in browser"),
			keyStyle.Render("m         ") + descStyle.Render("  Move issue     "),
			keyStyle.Render("t         ") + descStyle.Render("  Toggle view    "),
			keyStyle.Render("f         ") + descStyle.Render("  Toggle filter  "),
			keyStyle.Render("r         ") + descStyle.Render("  Refresh        "),
			keyStyle.Render("q/ctrl+c  ") + descStyle.Render("  Quit           "),
			"",
			descStyle.Render("Press ?, Esc, or Ctrl+g to close"),
		}

		help := strings.Join(lines, "\n")
		helpBox := helpBoxStyle.Render(help)

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpBox)
	}

	return content
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
