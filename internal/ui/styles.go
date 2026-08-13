package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent  = lipgloss.AdaptiveColor{Light: "63", Dark: "111"}
	colorFaint   = lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
	colorAdd     = lipgloss.AdaptiveColor{Light: "28", Dark: "78"}
	colorDel     = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	colorHunk    = lipgloss.AdaptiveColor{Light: "31", Dark: "80"}
	colorDone    = lipgloss.AdaptiveColor{Light: "28", Dark: "78"}
	colorWarn    = lipgloss.AdaptiveColor{Light: "130", Dark: "215"}
	colorTitleFg = lipgloss.AdaptiveColor{Light: "231", Dark: "231"}

	styleHeader = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "63", Dark: "60"}).
			Foreground(colorTitleFg).
			Bold(true).
			Padding(0, 1)

	styleHeaderInfo = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "63", Dark: "60"}).
			Foreground(lipgloss.AdaptiveColor{Light: "255", Dark: "252"})

	stylePane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorFaint)

	stylePaneFocus = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent)

	stylePaneTitle = lipgloss.NewStyle().
			Foreground(colorFaint).
			Bold(true)

	stylePaneTitleFocus = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	styleStep = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleCursor = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "254", Dark: "237"}).
			Bold(true)

	styleReviewed = lipgloss.NewStyle().Foreground(colorDone)
	styleFaint    = lipgloss.NewStyle().Foreground(colorFaint)

	styleDiffAdd  = lipgloss.NewStyle().Foreground(colorAdd)
	styleDiffDel  = lipgloss.NewStyle().Foreground(colorDel)
	styleDiffHunk = lipgloss.NewStyle().Foreground(colorHunk).Bold(true)
	styleDiffMeta = lipgloss.NewStyle().Foreground(colorFaint)

	styleGuideHead = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleGuideWarn = lipgloss.NewStyle().Foreground(colorWarn)

	styleStatusA = lipgloss.NewStyle().Foreground(colorAdd).Bold(true)
	styleStatusM = lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
	styleStatusD = lipgloss.NewStyle().Foreground(colorDel).Bold(true)

	styleFooter = lipgloss.NewStyle().Foreground(colorFaint).Padding(0, 1)
)

func plainStatus(status string) string {
	switch status {
	case "added":
		return "A"
	case "deleted":
		return "D"
	case "renamed":
		return "R"
	case "modified":
		return "M"
	default:
		return "·"
	}
}

func statusIcon(status string) string {
	switch status {
	case "added":
		return styleStatusA.Render("A")
	case "deleted":
		return styleStatusD.Render("D")
	case "renamed":
		return styleStatusM.Render("R")
	case "modified":
		return styleStatusM.Render("M")
	default:
		return styleFaint.Render("·")
	}
}
