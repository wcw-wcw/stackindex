package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	badgeStyle    = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
)
