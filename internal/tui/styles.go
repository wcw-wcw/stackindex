package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selectedStyle     = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
	headerStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1, 2).MarginBottom(1)
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2).MarginBottom(1)
	methodStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	okStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	highStyle         = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("160"))
	mediumStyle       = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("172"))
	lowStyle          = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("60"))
	infoStyle         = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("238"))
)
