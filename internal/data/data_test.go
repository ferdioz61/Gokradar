package data

import (
	"testing"
	"time"

	"gokradar/internal/model"
)

func TestLoadWorldData(t *testing.T) {
	wd, err := LoadWorldData()
	if err != nil {
		t.Fatalf("LoadWorldData failed: %v", err)
	}
	if len(wd.Coastlines) < 50 {
		t.Fatalf("expected coastlines, got %d", len(wd.Coastlines))
	}
	if len(wd.Borders) < 100 {
		t.Fatalf("expected borders, got %d", len(wd.Borders))
	}
	if len(wd.Airports) < 30 {
		t.Fatalf("expected airports, got %d", len(wd.Airports))
	}
}

func TestParseOpenSkyStates(t *testing.T) {
	states := [][]any{
		{"4bc8c9", "THY1831 ", "Turkey", 1700000000.0, 1700000000.0, 28.74, 41.26, 10000.0, false, 230.0, 310.0, 5.0, nil, 10100.0, "2415", false, 0.0},
	}
	fl := ParseOpenSkyStates(states)
	if len(fl) != 1 {
		t.Fatalf("expected 1 flight, got %d", len(fl))
	}
	if fl[0].Callsign != "THY1831" || fl[0].Hex != "4bc8c9" || fl[0].AltFeet < 32000 {
		t.Fatalf("unexpected parsed flight: %+v", fl[0])
	}
}

func TestParseADSBLolAC(t *testing.T) {
	list := []adsbLolAC{
		{Hex: "4b9c62", Flight: "THY6652 ", Reg: "TC-GCB", Type: "A321", AltBaro: 28600.0, GS: 383.9, Track: 273.8, BaroRate: 960, Squawk: "2361", Lat: 40.92, Lon: 26.93, DBFlags: 1},
		{Hex: "4b0001", Flight: "GND1", AltBaro: "ground", Lat: 41.26, Lon: 28.74},
	}
	fl := ParseADSBLolAC(list, "ADSB", false)
	if len(fl) != 2 {
		t.Fatalf("expected 2 flights, got %d", len(fl))
	}
	if !fl[0].IsMilitary || fl[0].AircraftType != "A321" || fl[0].Registration != "TC-GCB" {
		t.Fatalf("unexpected rich flight: %+v", fl[0])
	}
	if !fl[1].OnGround || fl[1].AltFeet != 0 {
		t.Fatalf("expected ground aircraft, got %+v", fl[1])
	}
}

func TestRouteCache(t *testing.T) {
	rc := NewRouteCache()
	rc.Set("thy1", &model.RouteInfo{
		Callsign:   "THY1",
		OriginIATA: "IST",
		DestIATA:   "JFK",
		FetchedAt:  time.Now(),
	})
	r, ok := rc.Get("THY1")
	if !ok || r.ShortRoute() != "IST → JFK" {
		t.Fatalf("unexpected route cache result: ok=%v, r=%+v", ok, r)
	}
}
