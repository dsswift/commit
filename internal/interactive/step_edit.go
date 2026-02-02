package interactive

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// EditModel handles the combined reorder and operations step.
type EditModel struct {
	entries []RebaseEntry
	cursor  int
	styles  Styles
	keys    KeyMap

	// Inline message editing
	editingMessage bool
	messageInput   textinput.Model
	editingIndex   int
}

// EditDoneMsg is sent when the user completes editing.
type EditDoneMsg struct {
	Entries []RebaseEntry
}

// EditBackMsg is sent when the user wants to go back.
type EditBackMsg struct{}

// NewEditModel creates a new edit model.
func NewEditModel(entries []RebaseEntry, styles Styles, keys KeyMap) *EditModel {
	ti := textinput.New()
	ti.Placeholder = "Enter new commit message..."
	ti.CharLimit = 200
	ti.Width = 60

	return &EditModel{
		entries:      entries,
		cursor:       0,
		styles:       styles,
		keys:         keys,
		messageInput: ti,
	}
}

// Update implements tea.Model.
func (m *EditModel) Update(msg tea.Msg) (*EditModel, tea.Cmd) {
	// Handle inline message editing
	if m.editingMessage {
		return m.updateMessageEdit(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keys.MoveUp):
			m.moveUp()

		case key.Matches(msg, m.keys.MoveDown):
			m.moveDown()

		case key.Matches(msg, m.keys.Tab):
			m.cycleOperation()

		case key.Matches(msg, m.keys.Pick):
			m.setOperation(OpPick)

		case key.Matches(msg, m.keys.Squash):
			m.setOperation(OpSquash)

		case key.Matches(msg, m.keys.Reword):
			m.setOperation(OpReword)

		case key.Matches(msg, m.keys.Drop):
			m.setOperation(OpDrop)

		case key.Matches(msg, m.keys.EditMsg):
			m.startMessageEdit()

		case key.Matches(msg, m.keys.Enter):
			return m, func() tea.Msg {
				return EditDoneMsg{Entries: m.entries}
			}

		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg {
				return EditBackMsg{}
			}
		}
	}

	return m, nil
}

// updateMessageEdit handles updates while editing a message.
func (m *EditModel) updateMessageEdit(msg tea.Msg) (*EditModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			// Save the message
			newMsg := strings.TrimSpace(m.messageInput.Value())
			if newMsg != "" {
				m.entries[m.editingIndex].NewMessage = newMsg
				m.entries[m.editingIndex].MessageEdited = true
				m.entries[m.editingIndex].Operation = OpReword
			}
			m.editingMessage = false
			return m, nil

		case tea.KeyEsc:
			// Cancel editing
			m.editingMessage = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.messageInput, cmd = m.messageInput.Update(msg)
	return m, cmd
}

// startMessageEdit begins inline message editing.
func (m *EditModel) startMessageEdit() {
	m.editingMessage = true
	m.editingIndex = m.cursor
	entry := m.entries[m.cursor]

	// Pre-fill with existing message
	if entry.NewMessage != "" {
		m.messageInput.SetValue(entry.NewMessage)
	} else {
		m.messageInput.SetValue(entry.Commit.Message)
	}
	m.messageInput.Focus()
	m.messageInput.CursorEnd()
}

// moveUp moves the current entry up in the list.
func (m *EditModel) moveUp() {
	if m.cursor <= 0 {
		return
	}

	// Swap entries
	m.entries[m.cursor], m.entries[m.cursor-1] = m.entries[m.cursor-1], m.entries[m.cursor]
	m.cursor--
}

// moveDown moves the current entry down in the list.
func (m *EditModel) moveDown() {
	if m.cursor >= len(m.entries)-1 {
		return
	}

	// Swap entries
	m.entries[m.cursor], m.entries[m.cursor+1] = m.entries[m.cursor+1], m.entries[m.cursor]
	m.cursor++
}

// cycleOperation cycles through pick → squash → reword → drop → pick.
func (m *EditModel) cycleOperation() {
	if m.cursor < len(m.entries) {
		m.entries[m.cursor].Operation = m.entries[m.cursor].Operation.Next()
	}
}

// setOperation sets the operation for the current entry.
func (m *EditModel) setOperation(op Operation) {
	if m.cursor < len(m.entries) {
		m.entries[m.cursor].Operation = op
	}
}

// View renders the edit view.
func (m *EditModel) View() string {
	if m.editingMessage {
		return m.renderMessageEdit()
	}

	var s string
	s += m.styles.Title.Render("Edit rebase plan (commits apply top-to-bottom):") + "\n\n"

	// Track which entries are squash parents for indentation
	for i, entry := range m.entries {
		isSelected := i == m.cursor

		// Determine indentation for squash entries
		indent := ""
		if entry.Operation == OpSquash {
			indent = m.styles.SquashIndent.Render("")
		}

		// Cursor
		cursor := "  "
		if isSelected {
			cursor = m.styles.Cursor.Render("")
		}

		// Operation with styling
		opStyle := m.styles.OperationStyle(entry.Operation)
		opStr := opStyle.Render(entry.Operation.String())

		// Commit info
		hash := m.styles.CommitHash.Render(entry.Commit.ShortHash)

		// Use new message if edited, otherwise original
		msg := entry.Commit.Message
		if entry.NewMessage != "" {
			msg = entry.NewMessage
		}
		msgStyled := m.styles.CommitMessage.Render(msg)

		// Mark edited messages
		editMark := ""
		if entry.MessageEdited {
			editMark = m.styles.Success.Render(" (edited)")
		}

		line := fmt.Sprintf("%s%s%s %s %s%s", cursor, indent, opStr, hash, msgStyled, editMark)
		s += line + "\n"
	}

	// Help bar
	s += "\n"
	s += m.styles.HelpKey.Render("↑/↓") + m.styles.HelpDesc.Render(" navigate  ")
	s += m.styles.HelpKey.Render("K/J") + m.styles.HelpDesc.Render(" move  ")
	s += m.styles.HelpKey.Render("tab") + m.styles.HelpDesc.Render(" cycle op  ")
	s += m.styles.HelpKey.Render("e") + m.styles.HelpDesc.Render(" edit msg\n")
	s += m.styles.HelpKey.Render("p") + m.styles.HelpDesc.Render(" pick  ")
	s += m.styles.HelpKey.Render("s") + m.styles.HelpDesc.Render(" squash  ")
	s += m.styles.HelpKey.Render("r") + m.styles.HelpDesc.Render(" reword  ")
	s += m.styles.HelpKey.Render("d") + m.styles.HelpDesc.Render(" drop  ")
	s += m.styles.HelpKey.Render("enter") + m.styles.HelpDesc.Render(" confirm  ")
	s += m.styles.HelpKey.Render("b") + m.styles.HelpDesc.Render(" back")

	return s
}

// renderMessageEdit renders the inline message editing overlay.
func (m *EditModel) renderMessageEdit() string {
	entry := m.entries[m.editingIndex]

	var s string
	s += m.styles.Title.Render(fmt.Sprintf("Edit commit message for %s:", entry.Commit.ShortHash)) + "\n\n"
	s += m.styles.Subtle.Render("Current: ") + entry.Commit.Message + "\n"
	s += m.styles.Subtle.Render("New: ") + m.messageInput.View() + "\n\n"
	s += m.styles.HelpKey.Render("enter") + m.styles.HelpDesc.Render(" save  ")
	s += m.styles.HelpKey.Render("esc") + m.styles.HelpDesc.Render(" cancel")

	return s
}

// Entries returns a copy of the current entries.
func (m *EditModel) Entries() []RebaseEntry {
	result := make([]RebaseEntry, len(m.entries))
	copy(result, m.entries)
	return result
}
