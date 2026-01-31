package interactive

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// ConfirmModel handles the confirm and execute step.
type ConfirmModel struct {
	entries    []RebaseEntry
	baseCommit string
	gitRoot    string
	cursor     int
	styles     Styles
	keys       KeyMap

	// Squash message editing state
	editingSquashMsg bool
	squashTextArea   textarea.Model
	squashParentIdx  int

	// State
	executing bool
}

// ConfirmDoneMsg is sent when execution is complete or cancelled.
type ConfirmDoneMsg struct {
	Executed bool
	Err      error
}

// ConfirmBackMsg is sent when the user wants to go back.
type ConfirmBackMsg struct{}

// NewConfirmModel creates a new confirm model.
func NewConfirmModel(entries []RebaseEntry, baseCommit, gitRoot string, styles Styles, keys KeyMap) *ConfirmModel {
	ta := textarea.New()
	ta.Placeholder = "Enter combined commit message..."
	ta.ShowLineNumbers = false
	ta.SetWidth(60)
	ta.SetHeight(6)

	return &ConfirmModel{
		entries:        entries,
		baseCommit:     baseCommit,
		gitRoot:        gitRoot,
		cursor:         0,
		styles:         styles,
		keys:           keys,
		squashTextArea: ta,
	}
}

// Update implements tea.Model.
func (m *ConfirmModel) Update(msg tea.Msg) (*ConfirmModel, tea.Cmd) {
	if m.editingSquashMsg {
		return m.updateSquashMsgEdit(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < 2 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.Enter):
			switch m.cursor {
			case 0: // Execute
				// Check if we need to prompt for squash messages
				if needsSquashPrompt, parentIdx := m.needsSquashMessagePrompt(); needsSquashPrompt {
					return m, m.startSquashMsgEdit(parentIdx)
				}
				return m, m.executeRebase()
			case 1: // Go back
				return m, func() tea.Msg {
					return ConfirmBackMsg{}
				}
			case 2: // Cancel
				return m, func() tea.Msg {
					return ConfirmDoneMsg{Executed: false}
				}
			}

		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg {
				return ConfirmBackMsg{}
			}
		}
	}

	return m, nil
}

// needsSquashMessagePrompt checks if any squash group needs a combined message.
// Returns true and the index of the first parent that needs prompting.
func (m *ConfirmModel) needsSquashMessagePrompt() (bool, int) {
	for i, entry := range m.entries {
		if entry.Operation != OpPick {
			continue
		}

		// Check if this pick has squash children
		hasSquashChildren := false
		for j := i + 1; j < len(m.entries); j++ {
			if m.entries[j].Operation == OpSquash {
				hasSquashChildren = true
			} else {
				break
			}
		}

		// If it has squash children and the parent message wasn't edited, prompt
		if hasSquashChildren && !entry.MessageEdited {
			return true, i
		}
	}

	return false, -1
}

// startSquashMsgEdit begins editing a squash message.
func (m *ConfirmModel) startSquashMsgEdit(parentIdx int) tea.Cmd {
	m.editingSquashMsg = true
	m.squashParentIdx = parentIdx

	// Build the combined message
	var msgs []string
	msgs = append(msgs, m.entries[parentIdx].GetEffectiveMessage())

	for j := parentIdx + 1; j < len(m.entries); j++ {
		if m.entries[j].Operation == OpSquash {
			msgs = append(msgs, m.entries[j].GetEffectiveMessage())
		} else {
			break
		}
	}

	combined := strings.Join(msgs, "\n\n")
	m.squashTextArea.SetValue(combined)
	m.squashTextArea.Focus()

	return nil
}

// updateSquashMsgEdit handles updates while editing a squash message.
func (m *ConfirmModel) updateSquashMsgEdit(msg tea.Msg) (*ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if msg.Alt {
				// Alt+Enter to insert newline
				var cmd tea.Cmd
				m.squashTextArea, cmd = m.squashTextArea.Update(msg)
				return m, cmd
			}
			// Regular Enter saves
			newMsg := strings.TrimSpace(m.squashTextArea.Value())
			if newMsg != "" {
				m.entries[m.squashParentIdx].NewMessage = newMsg
				m.entries[m.squashParentIdx].MessageEdited = true
			}
			m.editingSquashMsg = false

			// Check if there are more squash groups needing prompts
			if needsMore, nextIdx := m.needsSquashMessagePrompt(); needsMore {
				return m, m.startSquashMsgEdit(nextIdx)
			}

			// All done, execute
			return m, m.executeRebase()

		case tea.KeyEsc:
			m.editingSquashMsg = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.squashTextArea, cmd = m.squashTextArea.Update(msg)
	return m, cmd
}

// executeRebase executes the rebase operation.
func (m *ConfirmModel) executeRebase() tea.Cmd {
	entries := m.entries
	baseCommit := m.baseCommit
	gitRoot := m.gitRoot

	return func() tea.Msg {
		rebaser := NewRebaser(gitRoot)
		err := rebaser.Execute(entries, baseCommit)
		if err != nil {
			return ConfirmDoneMsg{Executed: false, Err: err}
		}
		return ConfirmDoneMsg{Executed: true}
	}
}

// View renders the confirm view.
func (m *ConfirmModel) View() string {
	if m.editingSquashMsg {
		return m.renderSquashMsgEdit()
	}

	var s string
	s += m.styles.Title.Render("Review rebase plan:") + "\n\n"

	// Show the plan
	for i, entry := range m.entries {
		indent := ""
		if entry.Operation == OpSquash {
			indent = m.styles.SquashIndent.Render("")
		}

		opStyle := m.styles.OperationStyle(entry.Operation)
		opStr := opStyle.Render(entry.Operation.String())
		hash := m.styles.CommitHash.Render(entry.Commit.ShortHash)

		msg := entry.GetEffectiveMessage()
		msgStyled := m.styles.CommitMessage.Render(msg)

		// Show reword indicator
		suffix := ""
		if entry.Operation == OpReword && entry.NewMessage != "" && entry.NewMessage != entry.Commit.Message {
			suffix = m.styles.Success.Render(fmt.Sprintf(" -> %q", entry.NewMessage))
		}

		// Show squash parent indicator
		if entry.Operation == OpPick {
			hasSquashChildren := false
			for j := i + 1; j < len(m.entries); j++ {
				if m.entries[j].Operation == OpSquash {
					hasSquashChildren = true
				} else {
					break
				}
			}
			if hasSquashChildren && entry.MessageEdited {
				suffix = m.styles.Success.Render(" (squash message set)")
			}
		}

		s += fmt.Sprintf("  %s%s %s %s%s\n", indent, opStr, hash, msgStyled, suffix)
	}

	// Options
	s += "\n"
	options := []string{"Execute rebase", "Go back and make changes", "Cancel"}
	for i, opt := range options {
		cursor := "  "
		if i == m.cursor {
			cursor = m.styles.Cursor.Render("")
		}
		s += fmt.Sprintf("%s%d. %s\n", cursor, i+1, opt)
	}

	// Help bar
	s += "\n"
	s += m.styles.HelpKey.Render("↑/↓") + m.styles.HelpDesc.Render(" navigate  ")
	s += m.styles.HelpKey.Render("enter") + m.styles.HelpDesc.Render(" select  ")
	s += m.styles.HelpKey.Render("b") + m.styles.HelpDesc.Render(" back")

	return s
}

// renderSquashMsgEdit renders the squash message editing view.
func (m *ConfirmModel) renderSquashMsgEdit() string {
	parent := m.entries[m.squashParentIdx]

	// Count squash children
	squashCount := 1 // Include parent
	for j := m.squashParentIdx + 1; j < len(m.entries); j++ {
		if m.entries[j].Operation == OpSquash {
			squashCount++
		} else {
			break
		}
	}

	var s string
	s += m.styles.Title.Render(fmt.Sprintf("Squash message needed for %s (%d commits):", parent.Commit.ShortHash, squashCount)) + "\n\n"
	s += m.squashTextArea.View() + "\n\n"
	s += m.styles.Subtle.Render("Edit the combined message above, or replace entirely.") + "\n\n"
	s += m.styles.HelpKey.Render("enter") + m.styles.HelpDesc.Render(" confirm  ")
	s += m.styles.HelpKey.Render("esc") + m.styles.HelpDesc.Render(" cancel")

	return s
}
