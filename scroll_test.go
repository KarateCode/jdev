package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func makeIssues(n int) []Issue {
	issues := make([]Issue, n)
	for i := range issues {
		issues[i].Key = fmt.Sprintf("EP-%03d", i)
		issues[i].Fields.Summary = fmt.Sprintf("Summary for issue number %d", i)
		issues[i].Fields.Priority.Name = "Medium"
		issues[i].Fields.Status.Name = "Dev Review"
		issues[i].Fields.IssueType.Name = "Bug"
	}
	return issues
}

func newTestModel(tableView bool, n, height int) model {
	m := initialModel(tableView, filterDevReview)
	m.loading = false
	m.issues = makeIssues(n)
	m.width = 120
	m.height = height
	m.ensureCursorVisible()
	return m
}

func send(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// The rendered view must never be taller than the terminal
func assertFits(t *testing.T, m model, label string) {
	t.Helper()
	h := lipgloss.Height(m.View())
	if h > m.height {
		t.Errorf("%s: view is %d lines, terminal is %d", label, h, m.height)
	}
}

// The focused issue's key must appear in the rendered output
func assertCursorVisible(t *testing.T, m model, label string) {
	t.Helper()
	if len(m.issues) == 0 {
		return
	}
	want := m.issues[m.cursor].Key
	if !strings.Contains(m.View(), want) {
		t.Errorf("%s: focused issue %s (index %d) is off screen; showing %d-%d",
			label, want, m.cursor, m.scrollOffset, m.visibleEnd())
	}
}

func TestScrollingKeepsCursorVisible(t *testing.T) {
	for _, tableView := range []bool{false, true} {
		for _, height := range []int{10, 24, 40, 51} {
			mode := "column"
			if tableView {
				mode = "table"
			}
			label := fmt.Sprintf("%s/h%d", mode, height)

			m := newTestModel(tableView, 45, height)
			assertFits(t, m, label+" initial")
			assertCursorVisible(t, m, label+" initial")

			// Walk all the way down one row at a time
			for i := 0; i < 50; i++ {
				m = send(m, key("down"))
				assertFits(t, m, label+" down")
				assertCursorVisible(t, m, label+" down")
			}
			if m.cursor != 44 {
				t.Errorf("%s: cursor is %d after walking down, want 44", label, m.cursor)
			}

			// And all the way back up
			for i := 0; i < 50; i++ {
				m = send(m, key("up"))
				assertFits(t, m, label+" up")
				assertCursorVisible(t, m, label+" up")
			}
			if m.cursor != 0 || m.scrollOffset != 0 {
				t.Errorf("%s: cursor=%d offset=%d after walking up, want 0/0", label, m.cursor, m.scrollOffset)
			}

			// Paging
			for i := 0; i < 10; i++ {
				m = send(m, key("pgdown"))
				assertFits(t, m, label+" pgdown")
				assertCursorVisible(t, m, label+" pgdown")
			}
			for i := 0; i < 10; i++ {
				m = send(m, key("pgup"))
				assertFits(t, m, label+" pgup")
				assertCursorVisible(t, m, label+" pgup")
			}

			// Jump to bottom and top
			m = send(m, key("G"))
			if m.cursor != 44 {
				t.Errorf("%s: G put cursor at %d, want 44", label, m.cursor)
			}
			assertFits(t, m, label+" G")
			assertCursorVisible(t, m, label+" G")

			m = send(m, key("g"))
			if m.cursor != 0 {
				t.Errorf("%s: g put cursor at %d, want 0", label, m.cursor)
			}
			assertCursorVisible(t, m, label+" g")
		}
	}
}

func TestRefreshFollowsFocusedIssue(t *testing.T) {
	m := newTestModel(false, 45, 40)
	m = send(m, key("G")) // Focus EP-044, scrolled to the bottom

	// Refresh with the first 20 issues dropped: EP-044 is now index 24
	m = send(m, key("r"))
	if m.refreshKey != "EP-044" {
		t.Fatalf("refreshKey is %q, want EP-044", m.refreshKey)
	}
	m = send(m, issuesMsg(makeIssues(45)[20:]))

	if got := m.issues[m.cursor].Key; got != "EP-044" {
		t.Errorf("after refresh focus is %s, want EP-044", got)
	}
	assertFits(t, m, "refresh/followed")
	assertCursorVisible(t, m, "refresh/followed")
}

func TestRefreshWhenFocusedIssueIsGone(t *testing.T) {
	// The reported bug: focused issue leaves Dev Review, selection lands
	// somewhere else and that somewhere else is off screen
	m := newTestModel(false, 45, 40)
	m = send(m, key("G"))

	m = send(m, key("r"))
	m = send(m, issuesMsg(makeIssues(30))) // EP-044 no longer in the list

	if m.cursor > len(m.issues)-1 {
		t.Fatalf("cursor %d out of range for %d issues", m.cursor, len(m.issues))
	}
	assertFits(t, m, "refresh/gone")
	assertCursorVisible(t, m, "refresh/gone")
}

func TestRefreshToShorterAndEmptyLists(t *testing.T) {
	for _, n := range []int{1, 3, 7, 44} {
		m := newTestModel(false, 45, 40)
		m = send(m, key("G"))
		m = send(m, key("r"))
		m = send(m, issuesMsg(makeIssues(n)))

		assertFits(t, m, fmt.Sprintf("shrink to %d", n))
		assertCursorVisible(t, m, fmt.Sprintf("shrink to %d", n))
	}

	m := newTestModel(false, 45, 40)
	m = send(m, key("G"))
	m = send(m, key("r"))
	m = send(m, issuesMsg([]Issue{}))
	if m.cursor != 0 || m.scrollOffset != 0 {
		t.Errorf("empty list: cursor=%d offset=%d, want 0/0", m.cursor, m.scrollOffset)
	}
	m.View() // Must not panic
}

func TestResizeKeepsCursorVisible(t *testing.T) {
	m := newTestModel(false, 45, 50)
	m = send(m, key("G"))

	for _, h := range []int{50, 30, 12, 8, 60, 20} {
		m = send(m, tea.WindowSizeMsg{Width: 120, Height: h})
		assertFits(t, m, fmt.Sprintf("resize h%d", h))
		assertCursorVisible(t, m, fmt.Sprintf("resize h%d", h))
	}
}

func TestStatusMessageDoesNotPushCursorOffScreen(t *testing.T) {
	m := newTestModel(false, 45, 40)
	m = send(m, key("G"))
	m.statusMsg = "Found PR #1234 for EP-044"
	assertFits(t, m, "status shown")
	assertCursorVisible(t, m, "status shown")

	m = send(m, clearStatusMsg{})
	assertFits(t, m, "status cleared")
	assertCursorVisible(t, m, "status cleared")
}

func TestNoScrollIndicatorWhenEverythingFits(t *testing.T) {
	m := newTestModel(false, 4, 40)
	if ind := m.scrollIndicator(); ind != "" {
		t.Errorf("got scroll indicator %q for a list that fits", ind)
	}

	m = newTestModel(false, 45, 40)
	if !strings.Contains(m.View(), "of 45") {
		t.Error("expected a scroll indicator for a list that does not fit")
	}
}
