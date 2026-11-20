package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"gokradar/internal/model"
)

var (
	detailTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226"))
	detailRouteStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	detailLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	detailEmergStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	detailMilStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
	helpBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
)

// RenderDetail renders the bottom detail panel for the currently selected aircraft.
func RenderDetail(
	width int,
	sel *model.Flight,
	route *model.RouteInfo,
	routeLoading bool,
	isFav bool,
	metric bool,
	statusMsg string,
) string {
	if width < 20 {
		return ""
	}

	var line1, line2 string
	if sel == nil {
		line1 = detailLabelStyle.Render("SEÇİLİ UÇAK: Yok — Listeden bir uçuş seçin (↑/↓) veya Enter ile haritada odaklanın.")
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
			origCity := route.OriginCity
			destCity := route.DestCity
			airline := ""
			if route.AirlineName != "" {
				airline = route.AirlineName + " | "
			}
			if origCity != "" && destCity != "" {
				routeStr = detailRouteStyle.Render(fmt.Sprintf("%s%s (%s) → %s (%s)", airline, orig, origCity, dest, destCity))
			} else {
				routeStr = detailRouteStyle.Render(fmt.Sprintf("%s%s → %s", airline, orig, dest))
			}
		} else if route != nil && route.NotFound {
			routeStr = detailLabelStyle.Render("Rota bilgisi yok")
		}

		line1 = fmt.Sprintf("%s (%s)%s  |  %s",
			detailTitleStyle.Render(sel.DisplayID()),
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

	help := "[1-5] Bölge  [Tab] Odak  [+/-] Yakınlaş  [c] Ortala  [f] Tam Ekran  [/] Ara  [a] İrtifa  [M] Askeri  [E] Acil  [s/S] Fav  [m] ASCII  [u] Birim  [r] Yenile  [?] Yardım  [q] Çık"
	return line1 + "\n" + truncate(line2, width) + "\n" + helpBarStyle.Render(truncate(help, width))
}
