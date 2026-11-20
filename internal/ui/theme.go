package ui

import (
	"github.com/charmbracelet/lipgloss"

	"gokradar/internal/geo"
)

// Theme defines a complete color palette for the map canvas and TUI chrome.
type Theme struct {
	Name         string
	HeaderStyle  lipgloss.Style
	FocusBorder  lipgloss.Color
	IdleBorder   lipgloss.Color
	ListHeader   lipgloss.Style
	ListSelected lipgloss.Style
	DetailTitle  lipgloss.Style
	DetailRoute  lipgloss.Style
	CanvasStyles map[geo.Layer]lipgloss.Style
}

// Themes lists all built-in color palettes cycled via the 'T' key.
var Themes = []Theme{
	{
		Name:         "KLASİK",
		HeaderStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")),
		FocusBorder:  lipgloss.Color("39"),
		IdleBorder:   lipgloss.Color("240"),
		ListHeader:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
		ListSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Background(lipgloss.Color("24")).Bold(true),
		DetailTitle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226")),
		DetailRoute:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		CanvasStyles: geo.DefaultLayerStyles(),
	},
	{
		Name:         "ATC YEŞİL",
		HeaderStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("120")).Background(lipgloss.Color("22")),
		FocusBorder:  lipgloss.Color("46"),
		IdleBorder:   lipgloss.Color("28"),
		ListHeader:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82")),
		ListSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("154")).Background(lipgloss.Color("22")).Bold(true),
		DetailTitle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("118")),
		DetailRoute:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")),
		CanvasStyles: map[geo.Layer]lipgloss.Style{
			geo.LayerGrid:          lipgloss.NewStyle().Foreground(lipgloss.Color("22")),
			geo.LayerRangeRing:     lipgloss.NewStyle().Foreground(lipgloss.Color("28")),
			geo.LayerBorder:        lipgloss.NewStyle().Foreground(lipgloss.Color("22")),
			geo.LayerCoast:         lipgloss.NewStyle().Foreground(lipgloss.Color("34")),
			geo.LayerVector:        lipgloss.NewStyle().Foreground(lipgloss.Color("40")),
			geo.LayerRouteRem:      lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
			geo.LayerRouteDone:     lipgloss.NewStyle().Foreground(lipgloss.Color("118")),
			geo.LayerTrailLow:      lipgloss.NewStyle().Foreground(lipgloss.Color("34")),
			geo.LayerTrailMid:      lipgloss.NewStyle().Foreground(lipgloss.Color("46")),
			geo.LayerTrailHigh:     lipgloss.NewStyle().Foreground(lipgloss.Color("154")),
			geo.LayerAirport:       lipgloss.NewStyle().Foreground(lipgloss.Color("76")),
			geo.LayerRouteEndpoint: lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true),
			geo.LayerScaleBar:      lipgloss.NewStyle().Foreground(lipgloss.Color("120")).Background(lipgloss.Color("22")),
			geo.LayerPlaneLow:      lipgloss.NewStyle().Foreground(lipgloss.Color("40")),
			geo.LayerPlaneMid:      lipgloss.NewStyle().Foreground(lipgloss.Color("82")),
			geo.LayerPlaneHigh:     lipgloss.NewStyle().Foreground(lipgloss.Color("154")),
			geo.LayerPlaneFav:      lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true),
			geo.LayerPlaneMil:      lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true),
			geo.LayerPlaneEmerg:    lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
			geo.LayerPlaneSel:      lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(lipgloss.Color("118")).Bold(true),
		},
	},
	{
		Name:         "YÜKSEK KONTRAST",
		HeaderStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("19")),
		FocusBorder:  lipgloss.Color("51"),
		IdleBorder:   lipgloss.Color("247"),
		ListHeader:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")),
		ListSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("226")).Bold(true),
		DetailTitle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226")),
		DetailRoute:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")),
		CanvasStyles: map[geo.Layer]lipgloss.Style{
			geo.LayerGrid:          lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
			geo.LayerRangeRing:     lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
			geo.LayerBorder:        lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
			geo.LayerCoast:         lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
			geo.LayerVector:        lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
			geo.LayerRouteRem:      lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
			geo.LayerRouteDone:     lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true),
			geo.LayerTrailLow:      lipgloss.NewStyle().Foreground(lipgloss.Color("46")),
			geo.LayerTrailMid:      lipgloss.NewStyle().Foreground(lipgloss.Color("51")),
			geo.LayerTrailHigh:     lipgloss.NewStyle().Foreground(lipgloss.Color("201")),
			geo.LayerAirport:       lipgloss.NewStyle().Foreground(lipgloss.Color("255")),
			geo.LayerRouteEndpoint: lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("226")).Bold(true),
			geo.LayerScaleBar:      lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("19")),
			geo.LayerPlaneLow:      lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true),
			geo.LayerPlaneMid:      lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true),
			geo.LayerPlaneHigh:     lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true),
			geo.LayerPlaneFav:      lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true),
			geo.LayerPlaneMil:      lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true),
			geo.LayerPlaneEmerg:    lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("196")).Bold(true),
			geo.LayerPlaneSel:      lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("226")).Bold(true),
		},
	},
}
