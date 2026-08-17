package tui

import "github.com/charmbracelet/lipgloss"

// Palette: validated reference palette (dataviz skill), light/dark steps per
// mode. Categorical slots are assigned to entities in a fixed order and never
// re-ranked while the app runs; past the last slot entities fold into "Other".
var (
	slotColors = []lipgloss.AdaptiveColor{
		{Light: "#2a78d6", Dark: "#3987e5"}, // blue
		{Light: "#eb6834", Dark: "#d95926"}, // orange
		{Light: "#1baf7a", Dark: "#199e70"}, // aqua
		{Light: "#eda100", Dark: "#c98500"}, // yellow
		{Light: "#e87ba4", Dark: "#d55181"}, // magenta
		{Light: "#008300", Dark: "#008300"}, // green
	}
	cAccent = slotColors[0]

	cInk   = lipgloss.AdaptiveColor{Light: "#0b0b0b", Dark: "#ffffff"}
	cInk2  = lipgloss.AdaptiveColor{Light: "#52514e", Dark: "#c3c2b7"}
	cMuted = lipgloss.AdaptiveColor{Light: "#898781", Dark: "#898781"}
	cGrid  = lipgloss.AdaptiveColor{Light: "#e1e0d9", Dark: "#2c2c2a"}
	cAxis  = lipgloss.AdaptiveColor{Light: "#c3c2b7", Dark: "#383835"}
	// Selection background: same warm gray as the grid, stepped further from
	// the terminal background so the cursor row is clearly visible.
	cSel   = lipgloss.AdaptiveColor{Light: "#c3c2b7", Dark: "#4a4a45"}
	cOther = cMuted // fold-to-"Other" swatch
)

var (
	sInk    = lipgloss.NewStyle().Foreground(cInk)
	sInk2   = lipgloss.NewStyle().Foreground(cInk2)
	sMuted  = lipgloss.NewStyle().Foreground(cMuted)
	sAxis   = lipgloss.NewStyle().Foreground(cAxis)
	sAccent = lipgloss.NewStyle().Foreground(cAccent)

	sTitle    = lipgloss.NewStyle().Bold(true).Foreground(cInk)
	sBrand    = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sValueBig = lipgloss.NewStyle().Bold(true).Foreground(cInk)

	sTab       = lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2)
	sTabActive = lipgloss.NewStyle().Bold(true).Foreground(cAccent).Padding(0, 2).
			Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(cAccent)

	sTile = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cGrid).Padding(0, 2)

	sPanel      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cGrid).Padding(0, 1)
	sPanelTitle = lipgloss.NewStyle().Bold(true).Foreground(cInk2)

	sFooter = lipgloss.NewStyle().Foreground(cMuted)

	sSelected = lipgloss.NewStyle().Bold(true).Foreground(cInk).Background(cSel)
	sHeader   = lipgloss.NewStyle().Bold(true).Foreground(cInk2).
			Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(cGrid)
)

// slotColor returns the categorical color for a fixed slot index; entities
// past the last slot get the neutral "Other" color.
func slotColor(i int) lipgloss.AdaptiveColor {
	if i >= 0 && i < len(slotColors) {
		return slotColors[i]
	}
	return cOther
}

func swatch(c lipgloss.AdaptiveColor) string {
	return lipgloss.NewStyle().Foreground(c).Render("■")
}
