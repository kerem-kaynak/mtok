package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// money formats USD for display: cents below $100, whole dollars with
// thousands separators above.
func money(v float64) string {
	switch {
	case v == 0:
		return "$0"
	case v < 0.01:
		return "<$0.01"
	case v < 100:
		return fmt.Sprintf("$%.2f", v)
	default:
		return "$" + comma(int64(v+0.5))
	}
}

// moneyAxis is a compact form for chart axes.
func moneyAxis(v float64) string {
	switch {
	case v >= 1000:
		return fmt.Sprintf("$%.1fk", v/1000)
	case v >= 10:
		return fmt.Sprintf("$%.0f", v)
	default:
		return fmt.Sprintf("$%.1f", v)
	}
}

func comma(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// tok formats token counts: 999 / 45.6k / 1.23M / 4.5B.
func tok(n int64) string {
	f := float64(n)
	switch {
	case n < 1000:
		return strconv.FormatInt(n, 10)
	case n < 1_000_000:
		return trimZero(fmt.Sprintf("%.1f", f/1e3)) + "k"
	case n < 1_000_000_000:
		return trimZero(fmt.Sprintf("%.2f", f/1e6)) + "M"
	default:
		return trimZero(fmt.Sprintf("%.2f", f/1e9)) + "B"
	}
}

// tokAxis is tok for float64 chart axes.
func tokAxis(v float64) string {
	if v < 0 {
		v = 0
	}
	return tok(int64(v + 0.5))
}

func trimZero(s string) string {
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

func pct(f float64) string {
	return fmt.Sprintf("%.0f%%", f*100)
}

func duration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// shortPath abbreviates $HOME to ~ and keeps the tail of long paths.
func shortPath(p, home string, max int) string {
	if home != "" && strings.HasPrefix(p, home) {
		p = "~" + strings.TrimPrefix(p, home)
	}
	if len(p) > max && max > 1 {
		return "…" + p[len(p)-max+1:]
	}
	return p
}
