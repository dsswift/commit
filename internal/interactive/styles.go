package interactive

import "github.com/charmbracelet/lipgloss"

// Styles defines the visual styling for the interactive wizard.
type Styles struct {
	// Step indicator styles
	StepIndicator    lipgloss.Style
	StepActive       lipgloss.Style
	StepCompleted    lipgloss.Style
	StepPending      lipgloss.Style
	StepArrow        lipgloss.Style

	// List styles
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style
	ListItemDimmed   lipgloss.Style

	// Operation styles
	OpPick   lipgloss.Style
	OpSquash lipgloss.Style
	OpReword lipgloss.Style
	OpDrop   lipgloss.Style

	// Commit styles
	CommitHash    lipgloss.Style
	CommitMessage lipgloss.Style
	CommitMeta    lipgloss.Style
	CommitPushed  lipgloss.Style

	// Squash indentation
	SquashIndent lipgloss.Style

	// Input styles
	InputLabel lipgloss.Style
	InputField lipgloss.Style
	InputHint  lipgloss.Style

	// Message styles
	Title   lipgloss.Style
	Subtle  lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Success lipgloss.Style

	// Help bar
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
	HelpBar  lipgloss.Style

	// Cursor
	Cursor lipgloss.Style
}

// DefaultStyles returns the default styling.
func DefaultStyles() Styles {
	return Styles{
		// Step indicator
		StepIndicator: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		StepActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),
		StepCompleted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")),
		StepPending: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		StepArrow: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),

		// List items
		ListItem: lipgloss.NewStyle().
			PaddingLeft(2),
		ListItemSelected: lipgloss.NewStyle().
			PaddingLeft(0).
			Foreground(lipgloss.Color("212")),
		ListItemDimmed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),

		// Operations
		OpPick: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Width(7),
		OpSquash: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Width(7),
		OpReword: lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")).
			Width(7),
		OpDrop: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Width(7),

		// Commits
		CommitHash: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")),
		CommitMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		CommitMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		CommitPushed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")),

		// Squash indentation
		SquashIndent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			SetString("  └─ "),

		// Input
		InputLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		InputField: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("241")).
			Padding(0, 1),
		InputHint: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true),

		// Messages
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("252")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")),

		// Help bar
		HelpKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		HelpDesc: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")),
		HelpBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),

		// Cursor
		Cursor: lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			SetString("> "),
	}
}

// OperationStyle returns the appropriate style for an operation.
func (s Styles) OperationStyle(op Operation) lipgloss.Style {
	switch op {
	case OpPick:
		return s.OpPick
	case OpSquash:
		return s.OpSquash
	case OpReword:
		return s.OpReword
	case OpDrop:
		return s.OpDrop
	default:
		return s.OpPick
	}
}

// RenderStepIndicator renders the step progress indicator.
func (s Styles) RenderStepIndicator(current WizardStep) string {
	steps := []struct {
		step WizardStep
		name string
	}{
		{StepSelect, "Select Commit"},
		{StepEdit, "Edit"},
		{StepConfirm, "Confirm"},
	}

	var parts []string
	parts = append(parts, s.StepArrow.Render("←"))

	for i, step := range steps {
		var marker, name string

		switch {
		case step.step < current || (current == StepPushWarning && step.step == StepSelect):
			// Completed
			marker = s.StepCompleted.Render("✓")
			name = s.StepCompleted.Render(step.name)
		case step.step == current ||
			(current == StepPushWarning && step.step == StepSelect) ||
			(current == StepSquashMessage && step.step == StepConfirm):
			// Active
			marker = s.StepActive.Render("■")
			name = s.StepActive.Render(step.name)
		default:
			// Pending
			marker = s.StepPending.Render("□")
			name = s.StepPending.Render(step.name)
		}

		parts = append(parts, marker, name)
		if i < len(steps)-1 {
			parts = append(parts, s.StepPending.Render("□"))
		}
	}

	parts = append(parts, s.StepArrow.Render("→"))

	result := ""
	for _, p := range parts {
		result += p + " "
	}
	return result
}
