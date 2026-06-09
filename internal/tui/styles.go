package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	appStyle = lipgloss.NewStyle().
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A78BFA"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D4D4D8"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E4E4E7"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22C55E"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#52525B"))

	dimmedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#71717A"))

	activeProjectStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7C3AED")).
				Bold(true)

	inactiveProjectStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A1A1AA"))

	secretKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D4D4D8"))

	secretValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A1A1AA"))

	secretUnsetStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#52525B")).
				Italic(true)

	inputStyle = lipgloss.NewStyle().
			BorderForeground(lipgloss.Color("#7C3AED")).
			BorderStyle(lipgloss.NormalBorder()).
			Padding(0, 1)

	focusedInputStyle = lipgloss.NewStyle().
				BorderForeground(lipgloss.Color("#A78BFA")).
				BorderStyle(lipgloss.NormalBorder()).
				Padding(0, 1)

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#27272A"))

	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#7C3AED")).
				Padding(0, 1)

	unfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#27272A")).
				Padding(0, 1)

	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)
)
