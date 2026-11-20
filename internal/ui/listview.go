package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"gokradar/internal/data"
	"gokradar/internal/model"
)

var (
	listHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	listSelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Background(lipgloss.Color("24")).Bold(true)
	listEmergStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	listMilStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	listFavStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	listDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

// RenderList renders the right-hand scrollable live flight table.
func RenderList(
	width, height int,
	flights []model.Flight,
	cursor int,
	routes *data.RouteCache,
	isFav func(callsign, hex string) bool,
	metric bool,
	ascii bool,
	searching bool,
	searchQuery string,
	filterTags string,
) string {
	if width < 16 || height < 4 {
		return ""
	}

	var lines []string

	// Title / Search line
	if searching {
		lines = append(lines, listHeaderStyle.Render(truncate("ARA: "+searchQuery+"_", width)))
	} else if searchQuery != "" {
		lines = append(lines, listHeaderStyle.Render(truncate(fmt.Sprintf("UÇUŞLAR (%d) /%s", len(flights), searchQuery), width)))
	} else {
		title := fmt.Sprintf("UÇUŞLAR (%d)", len(flights))
		if filterTags != "" {
			title += " " + filterTags
		}
		lines = append(lines, listHeaderStyle.Render(truncate(title, width)))
	}

	// Column header
	hdr := fmt.Sprintf("  %-8s %7s %5s", "ÇAĞRI", "İRTİFA", "HIZ")
	if width >= 33 {
		hdr += " ROTA"
	}
	lines = append(lines, listDimStyle.Render(truncate(hdr, width)))

	rowsAvail := height - 2
	if rowsAvail < 1 {
		return strings.Join(lines, "\n")
	}

	if len(flights) == 0 {
		lines = append(lines, listDimStyle.Render("  Eşleşen uçuş yok"))
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}

	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(flights) {
		cursor = len(flights) - 1
	}

	start := 0
	if cursor >= rowsAvail {
		start = cursor - rowsAvail + 1
	}
	end := start + rowsAvail
	if end > len(flights) {
		end = len(flights)
	}

	for i := start; i < end; i++ {
		f := flights[i]
		icon := model.HeadingArrow(f.TrackDeg, ascii)
		if f.IsEmergency() {
			icon = "!"
		} else if isFav(f.Callsign, f.Hex) {
			if ascii {
				icon = "*"
			} else {
				icon = "★"
			}
		} else if f.IsMilitary {
			icon = "M"
		}

		altStr := formatCompactAlt(f.AltFeet, f.OnGround, metric)
		spdStr := formatCompactSpd(f.SpeedKt, metric)
		id := f.DisplayID()
		if len(id) > 8 {
			id = id[:8]
		}

		row := fmt.Sprintf("%s %-8s %7s %5s", icon, id, altStr, spdStr)
		if width >= 33 && f.Callsign != "" {
			if r, ok := routes.Get(f.Callsign); ok && !r.NotFound {
				sr := r.ShortRoute()
				if sr != "" {
					row += " " + sr
				}
			}
		}
		row = padRight(truncate(row, width), width)

		switch {
		case i == cursor:
			lines = append(lines, listSelStyle.Render(row))
		case f.IsEmergency():
			lines = append(lines, listEmergStyle.Render(row))
		case f.IsMilitary:
			lines = append(lines, listMilStyle.Render(row))
		case isFav(f.Callsign, f.Hex):
			lines = append(lines, listFavStyle.Render(row))
		default:
			lines = append(lines, row)
		}
	}

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func formatCompactAlt(feet int, onGround, metric bool) string {
	if onGround {
		return "YER"
	}
	if feet <= 0 {
		return "-"
	}
	if metric {
		return fmt.Sprintf("%dm", int(float64(feet)*0.3048))
	}
	return fmt.Sprintf("%dft", feet)
}

func formatCompactSpd(kt float64, metric bool) string {
	if kt <= 0 {
		return "-"
	}
	if metric {
		return fmt.Sprintf("%dkm", int(kt*1.852))
	}
	return fmt.Sprintf("%dkt", int(kt))
}

func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxW {
		return s
	}
	if maxW == 1 {
		return string(r[:1])
	}
	return string(r[:maxW-1]) + "…"
}

func padRight(s string, width int) string {
	n := len([]rune(s))
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}
