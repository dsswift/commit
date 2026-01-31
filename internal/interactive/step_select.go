package interactive

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dsswift/commit/internal/git"
)

const (
	initialCommitCount = 20
	loadMoreCount      = 20
)

// SelectModel handles the commit selection step.
type SelectModel struct {
	collector *git.Collector
	commits   []RebaseCommit
	cursor    int
	loading   bool
	hasMore   bool
	err       error
	styles    Styles
	keys      KeyMap
}

// SelectDoneMsg is sent when the user selects a commit.
type SelectDoneMsg struct {
	BaseCommit string        // The commit hash to rebase onto
	Entries    []RebaseEntry // Commits to be rebased (in chronological order)
}

// NewSelectModel creates a new commit selection model.
func NewSelectModel(collector *git.Collector) *SelectModel {
	return &SelectModel{
		collector: collector,
		cursor:    0,
		loading:   true,
		hasMore:   true,
		styles:    DefaultStyles(),
		keys:      DefaultKeyMap(),
	}
}

// Init implements tea.Model.
func (m *SelectModel) Init() tea.Cmd {
	return m.loadCommits(initialCommitCount)
}

// loadCommitsMsg carries loaded commits.
type loadCommitsMsg struct {
	commits []RebaseCommit
	err     error
}

// loadCommits returns a command that loads commits from git.
func (m *SelectModel) loadCommits(count int) tea.Cmd {
	collector := m.collector
	existingCount := len(m.commits)

	return func() tea.Msg {
		// Load more commits than we currently have
		totalCount := existingCount + count
		gitCommits, err := collector.GetCommitLog(totalCount)
		if err != nil {
			return loadCommitsMsg{err: err}
		}

		// Convert to RebaseCommit
		var commits []RebaseCommit
		for _, gc := range gitCommits {
			commits = append(commits, RebaseCommit{
				Hash:      gc.Hash,
				ShortHash: gc.ShortHash,
				Message:   gc.Message,
				Author:    gc.Author,
				Date:      gc.Date,
				IsPushed:  gc.IsPushed,
			})
		}

		return loadCommitsMsg{commits: commits}
	}
}

// Update implements tea.Model.
func (m *SelectModel) Update(msg tea.Msg) (*SelectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case loadCommitsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		prevCount := len(m.commits)
		m.commits = msg.commits
		// If we didn't get more commits, there are no more to load
		m.hasMore = len(m.commits) > prevCount
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.commits)-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.Enter):
			if len(m.commits) > 0 && m.cursor < len(m.commits) {
				return m, m.selectCommit()
			}

		case key.Matches(msg, m.keys.LoadMore):
			if m.hasMore && !m.loading {
				m.loading = true
				return m, m.loadCommits(loadMoreCount)
			}
		}
	}

	return m, nil
}

// selectCommit creates the entries for rebasing and returns the done message.
func (m *SelectModel) selectCommit() tea.Cmd {
	selectedIdx := m.cursor
	selectedCommit := m.commits[selectedIdx]

	// All commits BEFORE the selected one will be rebased
	// (commits are in reverse chronological order, so index 0 is most recent)
	// We need commits from index 0 to selectedIdx-1
	var entries []RebaseEntry
	for i := selectedIdx - 1; i >= 0; i-- {
		entries = append(entries, RebaseEntry{
			Commit:    m.commits[i],
			Operation: OpPick,
		})
	}

	return func() tea.Msg {
		return SelectDoneMsg{
			BaseCommit: selectedCommit.Hash,
			Entries:    entries,
		}
	}
}

// View renders the commit selection view.
func (m *SelectModel) View() string {
	if m.err != nil {
		return m.styles.Error.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.loading && len(m.commits) == 0 {
		return m.styles.Subtle.Render("Loading commits...")
	}

	var s string
	s += m.styles.Title.Render("Which commit should the rebase start from?") + "\n"
	s += m.styles.Subtle.Render("All commits AFTER this one will be included in the rebase.") + "\n\n"

	for i, commit := range m.commits {
		cursor := "  "
		if i == m.cursor {
			cursor = m.styles.Cursor.Render("")
		}

		// Format the commit line
		hash := m.styles.CommitHash.Render(commit.ShortHash)
		msg := m.styles.CommitMessage.Render(commit.Message)
		age := m.styles.CommitMeta.Render(formatAge(commit.Date))

		line := fmt.Sprintf("%s%d. %s %s %s", cursor, i+1, hash, msg, age)

		if commit.IsPushed {
			line += " " + m.styles.CommitPushed.Render("(pushed)")
		}

		s += line + "\n"
	}

	if m.hasMore {
		if m.loading {
			s += "\n" + m.styles.Subtle.Render("   Loading more...")
		} else {
			s += "\n" + m.styles.Subtle.Render("   [Press 'l' to load more commits...]")
		}
	}

	// Help bar
	s += "\n\n"
	s += m.styles.HelpKey.Render("↑/↓") + m.styles.HelpDesc.Render(" navigate  ")
	s += m.styles.HelpKey.Render("enter") + m.styles.HelpDesc.Render(" select  ")
	if m.hasMore {
		s += m.styles.HelpKey.Render("l") + m.styles.HelpDesc.Render(" load more  ")
	}
	s += m.styles.HelpKey.Render("esc") + m.styles.HelpDesc.Render(" cancel")

	return s
}

// formatAge returns a human-readable age string.
func formatAge(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}
