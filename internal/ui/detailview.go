package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"gokradar/internal/geo"
	"gokradar/internal/model"
)

var (
	detailLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	detailEmergStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	detailMilStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
	helpBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
)

// RouteProgress holds computed flight progress along Origin -> Destination.
type RouteProgress struct {
	Percent    float64 // 0..100
	DistDoneNM float64
	DistRemNM  float64
	ETAMinutes int // -1 if unknown (e.g. stationary / on ground)
	Valid      bool
}

// ComputeRouteProgress calculates remaining distance, percentage, and ETA for a flight.
func ComputeRouteProgress(sel *model.Flight, route *model.RouteInfo) RouteProgress {
	if sel == nil || route == nil || !route.HasCoords() {
		return RouteProgress{}
	}
	done := geo.HaversineNM(route.OriginLat, route.OriginLon, sel.Lat, sel.Lon)
	rem := geo.HaversineNM(sel.Lat, sel.Lon, route.DestLat, route.DestLon)
	total := done + rem
	if total < 1 {
		return RouteProgress{}
	}
	pct := (done / total) * 100.0
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	eta := -1
	if !sel.OnGround && sel.SpeedKt > 50 {
		eta = int(math.Round((rem / sel.SpeedKt) * 60.0))
	}
	return RouteProgress{
		Percent:    pct,
		DistDoneNM: done,
		DistRemNM:  rem,
		ETAMinutes: eta,
		Valid:      true,
	}
}

func formatProgressBar(pct float64, width int, ascii bool) string {
	if width < 4 {
		width = 4
	}
	pos := int(math.Round((pct / 100.0) * float64(width-1)))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}
	doneCh, remCh, planeCh := "━", "─", "✈"
	if ascii {
		doneCh, remCh, planeCh = "=", "-", ">"
	}
	var sb strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i < pos:
			sb.WriteString(doneCh)
		case i == pos:
			sb.WriteString(planeCh)
		default:
			sb.WriteString(remCh)
		}
	}
	return sb.String()
}

func formatETA(minutes int) string {
	if minutes < 0 {
		return ""
	}
	if minutes < 60 {
		return fmt.Sprintf("ETA ~%d dk", minutes)
	}
	h := minutes / 60
	m := minutes % 60
	return fmt.Sprintf("ETA ~%ds %02ddk", h, m)
}

// RenderDetail renders the bottom detail panel for the currently selected aircraft.
func RenderDetail(
	width int,
	sel *model.Flight,
	route *model.RouteInfo,
	routeLoading bool,
	isFav bool,
	metric bool,
	ascii bool,
	statusMsg string,
	theme Theme,
) string {
	if width < 20 {
		return ""
	}

	var line1, line2 string
	if sel == nil {
		line1 = detailLabelStyle.Render("SEÇİLİ UÇAK: Yok — Haritada bir uçağa tıklayın veya listeden seçin (↑/↓).")
		line2 = detailLabelStyle.Render(statusMsg)
	} else {
		badges := ""
		if isFav {
			badges += " ★FAV"
		}
		if sel.IsMilitary {
			badges += " " + detailMilStyle.Render("[ASKERİ]")
		}
		if sel.IsEmergency() {
			badges += " " + detailEmergStyle.Render("["+sel.EmergencyLabel()+"]")
		}

		meta := []string{strings.ToUpper(sel.Hex)}
		if sel.AircraftType != "" {
			meta = append(meta, sel.AircraftType)
		}
		if sel.Registration != "" {
			meta = append(meta, sel.Registration)
		}
		if sel.Country != "" {
			meta = append(meta, sel.Country)
		}

		routeStr := detailLabelStyle.Render("Rota: -")
		if routeLoading {
			routeStr = detailLabelStyle.Render("Rota aranıyor...")
		} else if route != nil && !route.NotFound {
			orig := route.OriginIATA
			if orig == "" {
				orig = route.OriginICAO
			}
			dest := route.DestIATA
			if dest == "" {
				dest = route.DestICAO
			}
			airline := ""
			if route.AirlineName != "" {
				airline = route.AirlineName + " | "
			}

			prog := ComputeRouteProgress(sel, route)
			if prog.Valid {
				bar := formatProgressBar(prog.Percent, 9, ascii)
				var remStr string
				if metric {
					remStr = fmt.Sprintf("%d km", int(math.Round(prog.DistRemNM*1.852)))
				} else {
					remStr = fmt.Sprintf("%d NM", int(math.Round(prog.DistRemNM)))
				}
				etaStr := formatETA(prog.ETAMinutes)
				extra := fmt.Sprintf("%%%.0f • Kalan: %s", prog.Percent, remStr)
				if etaStr != "" {
					extra += " • " + etaStr
				}
				routeStr = theme.DetailRoute.Render(fmt.Sprintf("%s%s %s %s (%s)", airline, orig, bar, dest, extra))
			} else if route.OriginCity != "" && route.DestCity != "" {
				routeStr = theme.DetailRoute.Render(fmt.Sprintf("%s%s (%s) → %s (%s)", airline, orig, route.OriginCity, dest, route.DestCity))
			} else {
				routeStr = theme.DetailRoute.Render(fmt.Sprintf("%s%s → %s", airline, orig, dest))
			}
		} else if route != nil && route.NotFound {
			routeStr = detailLabelStyle.Render("Rota bilgisi yok")
		}

		line1 = fmt.Sprintf("%s (%s)%s  |  %s",
			theme.DetailTitle.Render(sel.DisplayID()),
			strings.Join(meta, " / "),
			badges,
			routeStr,
		)

		squawk := sel.Squawk
		if squawk == "" {
			squawk = "-"
		}
		latDir := "K"
		if sel.Lat < 0 {
			latDir = "G"
		}
		lonDir := "D"
		if sel.Lon < 0 {
			lonDir = "B"
		}

		line2 = fmt.Sprintf(
			"İrtifa: %s (%s) | Hız: %s | Yön: %03.0f° | Squawk: %s | Konum: %.2f°%s %.2f°%s | Kaynak: %s",
			model.FormatAlt(sel.AltFeet, sel.OnGround, metric),
			model.FormatVertRate(sel.VertRateFPM, metric),
			model.FormatSpeed(sel.SpeedKt, metric),
			sel.TrackDeg,
			squawk,
			math.Abs(sel.Lat), latDir,
			math.Abs(sel.Lon), lonDir,
			sel.Source,
		)
	}

	help := "Fare: Sürükle/Tekerlek/Tıkla  |  [1-5] Bölge  [L] Kilit  [b] Etiket  [t] İz  [v] Vektör  [g] Halka  [T] Tema  [i] İstat  [p] Duraklat  [?] Yardım  [q] Çık"
	return line1 + "\n" + truncate(line2, width) + "\n" + helpBarStyle.Render(truncate(help, width))
}
