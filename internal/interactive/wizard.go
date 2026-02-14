package interactive

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dsswift/commit/internal/git"
)

// WizardModel is the main model for the interactive rebase wizard.
type WizardModel struct {
	// Configuration
	gitRoot string
	force   bool

	// State
	step      WizardStep
	cancelled bool
	completed bool
	err       error

	// Sub-models
	selectModel  *SelectModel
	editModel    *EditModel
	confirmModel *ConfirmModel

	// Collected data
	baseCommit string // The commit to rebase onto
	entries    []RebaseEntry

	// Styling
	styles Styles
	keys   KeyMap

	// Terminal dimensions
	width  int
	height int
}

// Config holds configuration for the wizard.
type Config struct {
	GitRoot string
	Force   bool
}

// NewWizard creates a new interactive rebase wizard.
func NewWizard(cfg Config) *WizardModel {
	collector := git.NewCollector(cfg.GitRoot)

	return &WizardModel{
		gitRoot:     cfg.GitRoot,
		force:       cfg.Force,
		step:        StepSelect,
		selectModel: NewSelectModel(collector),
		styles:      DefaultStyles(),
		keys:        DefaultKeyMap(),
		width:       80,
		height:      24,
	}
}

// Init implements tea.Model.
func (m *WizardModel) Init() tea.Cmd {
	return m.selectModel.Init()
}

// Update implements tea.Model.
func (m *WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global cancel handling
		if key.Matches(msg, m.keys.Cancel) {
			m.cancelled = true
			return m, tea.Quit
		}
	}

	// Delegate to current step
	var cmd tea.Cmd
	switch m.step {
	case StepSelect:
		cmd = m.updateSelect(msg)
	case StepPushWarning:
		cmd = m.updatePushWarning(msg)
	case StepEdit:
		cmd = m.updateEdit(msg)
	case StepSquashMessage:
		cmd = m.updateSquashMessage(msg)
	case StepConfirm:
		cmd = m.updateConfirm(msg)
	}

	return m, cmd
}

// updateSelect handles the commit selection step.
func (m *WizardModel) updateSelect(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case SelectDoneMsg:
		// User selected a commit
		m.baseCommit = msg.BaseCommit
		m.entries = msg.Entries

		// Check for pushed commits
		pushedCount := 0
		for _, e := range m.entries {
			if e.Commit.IsPushed {
				pushedCount++
			}
		}

		if pushedCount > 0 && !m.force {
			m.step = StepPushWarning
			return nil
		}

		// Move to edit step
		m.step = StepEdit
		m.editModel = NewEditModel(m.entries, m.styles, m.keys)
		return nil

	default:
		var cmd tea.Cmd
		m.selectModel, cmd = m.selectModel.Update(msg)
		return cmd
	}
}

// updatePushWarning handles the push warning step.
func (m *WizardModel) updatePushWarning(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Enter):
			// User chose to continue (would need --force)
			m.err = &PushedCommitError{}
			return tea.Quit
		case key.Matches(msg, m.keys.Back):
			// Go back to selection
			m.step = StepSelect
			return nil
		}
	}
	return nil
}

// updateEdit handles the edit step.
func (m *WizardModel) updateEdit(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case EditDoneMsg:
		m.entries = msg.Entries
		m.step = StepConfirm
		m.confirmModel = NewConfirmModel(m.entries, m.baseCommit, m.gitRoot, m.styles, m.keys)
		return nil

	case EditBackMsg:
		m.step = StepSelect
		return nil

	default:
		var cmd tea.Cmd
		m.editModel, cmd = m.editModel.Update(msg)
		return cmd
	}
}

// updateSquashMessage handles the squash message editing step.
func (m *WizardModel) updateSquashMessage(msg tea.Msg) tea.Cmd {
	// This is handled inline in confirmModel
	return nil
}

// updateConfirm handles the confirm step.
func (m *WizardModel) updateConfirm(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case ConfirmDoneMsg:
		if msg.Executed {
			m.completed = true
		}
		if msg.Err != nil {
			m.err = msg.Err
		}
		return tea.Quit

	case ConfirmBackMsg:
		m.step = StepEdit
		m.editModel = NewEditModel(m.entries, m.styles, m.keys)
		return nil

	default:
		var cmd tea.Cmd
		m.confirmModel, cmd = m.confirmModel.Update(msg)
		return cmd
	}
}

// View implements tea.Model.
func (m *WizardModel) View() string {
	if m.cancelled {
		return "Cancelled.\n"
	}

	if m.err != nil {
		return m.styles.Error.Render("Error: "+m.err.Error()) + "\n"
	}

	var content string

	// Step indicator
	header := m.styles.RenderStepIndicator(m.step)
	header = lipgloss.PlaceHorizontal(m.width, lipgloss.Center, header)

	// Step content
	switch m.step {
	case StepSelect:
		content = m.selectModel.View()
	case StepPushWarning:
		content = m.renderPushWarning()
	case StepEdit:
		content = m.editModel.View()
	case StepConfirm:
		content = m.confirmModel.View()
	}

	return header + "\n\n" + content
}

// renderPushWarning renders the push warning step.
func (m *WizardModel) renderPushWarning() string {
	pushedCount := 0
	for _, e := range m.entries {
		if e.Commit.IsPushed {
			pushedCount++
		}
	}

	var s string
	s += m.styles.Warning.Render("Warning: ") + m.styles.Title.Render("Pushed commits detected\n\n")
	s += m.styles.Subtle.Render("This rebase includes commits that have been pushed to origin.\n")
	s += m.styles.Subtle.Render("Rebasing will require force-push to sync with remote.\n\n")
	s += m.styles.Subtle.Render("Re-run with --force to proceed, or press 'b' to go back.\n\n")
	s += m.styles.HelpKey.Render("b") + m.styles.HelpDesc.Render(" back  ")
	s += m.styles.HelpKey.Render("q") + m.styles.HelpDesc.Render(" cancel")

	return s
}

// Result returns the result of the wizard after it completes.
func (m *WizardModel) Result() (completed bool, cancelled bool, err error) {
	return m.completed, m.cancelled, m.err
}

// PushedCommitError is returned when rebase includes pushed commits without --force.
type PushedCommitError struct{}

func (e *PushedCommitError) Error() string {
	return "rebase includes pushed commits; use --force to proceed"
}

// Run starts the interactive wizard and returns when complete.
func Run(cfg Config) (bool, error) {
	model := NewWizard(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	wizard := finalModel.(*WizardModel)
	completed, cancelled, wizardErr := wizard.Result()

	if cancelled {
		return false, nil
	}
	if wizardErr != nil {
		return false, wizardErr
	}

	return completed, nil
}
