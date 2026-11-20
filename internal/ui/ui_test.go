package ui

import (
	"math"
	"strings"
	"testing"

	"gokradar/internal/geo"
	"gokradar/internal/model"
)

func TestComputeRouteProgress(t *testing.T) {
	route := &model.RouteInfo{
		OriginIATA: "IST",
		OriginLat:  41.26,
		OriginLon:  28.74,
		DestIATA:   "FRA",
		DestLat:    50.03,
		DestLon:    8.56,
	}
	// Aircraft roughly halfway along IST -> FRA
	flight := &model.Flight{
		Callsign: "THY1591",
		Lat:      46.0,
		Lon:      18.5,
		SpeedKt:  450,
	}
	prog := ComputeRouteProgress(flight, route)
	if !prog.Valid {
		t.Fatalf("expected valid route progress")
	}
	if prog.Percent < 35 || prog.Percent > 65 {
		t.Fatalf("expected midpoint percent ~50%%, got %.1f%%", prog.Percent)
	}
	if prog.ETAMinutes <= 0 {
		t.Fatalf("expected positive ETA minutes, got %d", prog.ETAMinutes)
	}
}

func TestFindNearestFlightAt(t *testing.T) {
	vp := geo.Presets[1].Viewport // TÜRKİYE
	const w, h = 80, 24
	flights := []model.Flight{
		{Hex: "aaa", Callsign: "THY1", Lat: 41.26, Lon: 28.74}, // Istanbul
		{Hex: "bbb", Callsign: "PGT2", Lat: 39.95, Lon: 32.99}, // Ankara
	}
	x, y, vis := vp.Project(39.95, 32.99, w, h, float64(w-1), float64(h-1))
	if !vis {
		t.Fatalf("expected Ankara visible")
	}
	idx := FindNearestFlightAt(flights, vp, w, h, int(math.Round(x))+1, int(math.Round(y)), 3.0)
	if idx != 1 {
		t.Fatalf("expected Ankara flight index 1, got %d", idx)
	}
	// Clicking far away should return -1
	miss := FindNearestFlightAt(flights, vp, w, h, 0, 0, 2.0)
	if miss != -1 {
		t.Fatalf("expected -1 for far click, got %d", miss)
	}
}

func TestRenderMapAndStatsAllThemes(t *testing.T) {
	vp := geo.Presets[1].Viewport
	flights := []model.Flight{
		{Hex: "4b1800", Callsign: "THY1831", Country: "Turkey", Lat: 40.5, Lon: 30.0, AltFeet: 35000, SpeedKt: 460, TrackDeg: 295},
		{Hex: "4b1801", Callsign: "PGT404", Country: "Turkey", Lat: 39.2, Lon: 33.5, AltFeet: 18000, SpeedKt: 380, TrackDeg: 110},
	}
	trails := map[string][]model.Point{
		"4b1800": {
			{Lat: 40.1, Lon: 31.0, AltFeet: 12000},
			{Lat: 40.3, Lon: 30.5, AltFeet: 25000},
			{Lat: 40.5, Lon: 30.0, AltFeet: 35000},
		},
	}
	route := &model.RouteInfo{
		OriginIATA: "IST", OriginLat: 41.26, OriginLon: 28.74,
		DestIATA: "ESB", DestLat: 40.12, DestLon: 32.99,
	}
	allMap := map[string]model.Flight{"4b1800": flights[0], "4b1801": flights[1]}

	for _, th := range Themes {
		out := RenderMap(80, 24, vp, nil, flights, "4b1800", &flights[0], route, trails, func(_, _ string) bool { return false }, MapRenderOptions{
			LabelMode:   LabelDetailed,
			TrailMode:   TrailAll,
			ShowRings:   true,
			ShowVectors: true,
			Theme:       th,
		})
		if !strings.Contains(out, "THY1831") {
			t.Fatalf("theme %s: expected selected callsign in map output", th.Name)
		}
		stats := RenderStats(100, 30, flights, allMap, false, false, th)
		if !strings.Contains(stats, "CANLI TRAFİK İSTATİSTİKLERİ") {
			t.Fatalf("theme %s: expected stats header", th.Name)
		}
	}
}

func TestSelectionPersistsWhenPannedOffscreen(t *testing.T) {
	m := NewAppModel(nil, Options{Region: "tr"})
	m.flightsByHex["sel01"] = model.Flight{Hex: "sel01", Callsign: "THY1831", Lat: 41.0, Lon: 29.0}
	m.flightsByHex["other"] = model.Flight{Hex: "other", Callsign: "DLH99", Lat: 50.0, Lon: 8.5}
	m.selectedHex = "sel01"
	m.rebuildFiltered()

	// Pan camera far away (to Europe) so sel01 is temporarily outside viewport
	m.vp = geo.Presets[2].Viewport // AVRUPA (centered ~49, 11)
	m.vp.SpanLon = 10.0            // tight zoom on Germany, excluding Istanbul
	m.rebuildFiltered()

	if m.selectedHex != "sel01" {
		t.Fatalf("expected selectedHex to remain sel01 when panned offscreen, got %q", m.selectedHex)
	}
	sel := m.selectedFlight()
	if sel == nil || sel.Hex != "sel01" {
		t.Fatalf("expected selectedFlight() to still return sel01, got %+v", sel)
	}
}
