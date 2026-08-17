package tui

// Terminal chart primitives. Rules (dataviz method): single hue for
// magnitude, categorical hues only for identity, thin marks with gaps,
// recessive solid axes, selective direct labels, visible legends with values
// (the relief channel for low-contrast fills).

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var eighths = []rune(" ▁▂▃▄▅▆▇█")

// sparkline renders values as a single line scaled to the max, in the accent
// hue. Values are right-aligned: the most recent point is the last cell.
func sparkline(vals []float64, width int) string {
	if width <= 0 || len(vals) == 0 {
		return ""
	}
	if len(vals) > width {
		vals = vals[len(vals)-width:]
	}
	max := 0.0
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	for i := 0; i < width-len(vals); i++ {
		b.WriteByte(' ')
	}
	for _, v := range vals {
		idx := 0
		if max > 0 {
			idx = int(v/max*8 + 0.5)
			if idx > 8 {
				idx = 8
			}
			if v > 0 && idx == 0 {
				idx = 1
			}
		}
		b.WriteRune(eighths[idx])
	}
	return sAccent.Render(b.String())
}

// barChart renders a single-series column chart with a left y-axis (max and
// zero labels), block-eighth bar tops, 1-cell gaps between bars, and sparse
// x labels (first and last). fmtY formats axis values.
func barChart(vals []float64, xlabels []string, width, height int, fmtY func(float64) string) string {
	if height < 2 || width < 10 || len(vals) == 0 {
		return sMuted.Render("no data")
	}
	max := 0.0
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	yTop := fmtY(max)
	axisW := len(yTop)
	if axisW < 2 {
		axisW = 2
	}
	plotW := width - axisW - 1 // 1 for the axis rule
	if plotW < 3 {
		return ""
	}
	barW, gap := 2, 1
	n := (plotW + gap) / (barW + gap)
	if n < 1 {
		n = 1
	}
	if len(vals) > n {
		vals = vals[len(vals)-n:]
		if len(xlabels) > n {
			xlabels = xlabels[len(xlabels)-n:]
		}
	}
	n = len(vals)

	// Bar heights in eighths of a row.
	tot := make([]int, n)
	for i, v := range vals {
		if max > 0 {
			tot[i] = int(v/max*float64(height*8) + 0.5)
			if v > 0 && tot[i] == 0 {
				tot[i] = 1
			}
		}
	}

	pad := strings.Repeat(" ", axisW)
	var lines []string
	for row := 0; row < height; row++ {
		var b strings.Builder
		for i := 0; i < n; i++ {
			rem := tot[i] - (height-1-row)*8
			if rem < 0 {
				rem = 0
			}
			if rem > 8 {
				rem = 8
			}
			b.WriteString(strings.Repeat(string(eighths[rem]), barW))
			if i != n-1 {
				b.WriteByte(' ')
			}
		}
		label := pad
		if row == 0 {
			label = pads(yTop, axisW)
		}
		lines = append(lines, sMuted.Render(label)+sAxis.Render("│")+sAccent.Render(b.String()))
	}
	// Baseline.
	usedW := n*barW + (n-1)*gap
	lines = append(lines, sMuted.Render(pads(fmtY(0), axisW))+sAxis.Render("┴"+strings.Repeat("─", usedW)))
	// Sparse x labels: first and last.
	if len(xlabels) > 0 {
		first, last := xlabels[0], xlabels[len(xlabels)-1]
		gapW := usedW - len(first) - len(last)
		if gapW < 1 {
			first = ""
			gapW = usedW - len(last)
		}
		if gapW >= 0 {
			lines = append(lines, pad+" "+sMuted.Render(first+strings.Repeat(" ", gapW)+last))
		}
	}
	return strings.Join(lines, "\n")
}

func pads(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

// segment is one entity in a share bar.
type segment struct {
	Label string
	Value float64
	Color lipgloss.AdaptiveColor
}

// shareBar renders a proportional horizontal bar with a 1-cell surface gap
// between segments (the spacer rule). Callers render a legend with values —
// identity is never color-alone.
func shareBar(segs []segment, width int) string {
	total := 0.0
	for _, s := range segs {
		total += s.Value
	}
	if total <= 0 || width < len(segs)*2 {
		return sMuted.Render(strings.Repeat("░", maxInt(width, 0)))
	}
	// Give every nonzero segment at least one cell, distribute the rest.
	gaps := len(segs) - 1
	avail := width - gaps
	widths := make([]int, len(segs))
	used := 0
	for i, s := range segs {
		w := int(s.Value/total*float64(avail) + 0.5)
		if s.Value > 0 && w == 0 {
			w = 1
		}
		widths[i] = w
		used += w
	}
	for i := 0; used > avail && i < len(widths); i++ { // trim rounding overflow
		for j := range widths {
			if used <= avail {
				break
			}
			if widths[j] > 1 {
				widths[j]--
				used--
			}
		}
	}
	var parts []string
	for i, s := range segs {
		if widths[i] <= 0 {
			continue
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(s.Color).Render(strings.Repeat("█", widths[i])))
	}
	return strings.Join(parts, " ")
}

// meter renders a single ratio as fill over a same-hue recessive track.
func meter(frac float64, width int) string {
	if width <= 0 {
		return ""
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	fill := int(frac*float64(width) + 0.5)
	return sAccent.Render(strings.Repeat("█", fill)) + sMuted.Render(strings.Repeat("░", width-fill))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
