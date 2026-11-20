package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gokradar/internal/model"
)

type adsbLolResp struct {
	AC []adsbLolAC `json:"ac"`
}

type adsbLolAC struct {
	Hex      string  `json:"hex"`
	Flight   string  `json:"flight"`
	Reg      string  `json:"r"`
	Type     string  `json:"t"`
	AltBaro  any     `json:"alt_baro"`
	AltGeom  any     `json:"alt_geom"`
	GS       float64 `json:"gs"`
	Track    float64 `json:"track"`
	BaroRate int     `json:"baro_rate"`
	GeomRate int     `json:"geom_rate"`
	Squawk   string  `json:"squawk"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	DBFlags  int     `json:"dbFlags"`
}

// FetchADSBLolPoint fetches rich ADS-B aircraft within distNM (max 250) of (lat, lon).
// Falls back to airplanes.live if adsb.lol fails.
func FetchADSBLolPoint(ctx context.Context, client *http.Client, lat, lon float64, distNM int) ([]model.Flight, error) {
	if distNM > 250 {
		distNM = 250
	}
	if distNM < 25 {
		distNM = 25
	}
	urls := []string{
		fmt.Sprintf("https://api.adsb.lol/v2/point/%.4f/%.4f/%d", lat, lon, distNM),
		fmt.Sprintf("https://api.airplanes.live/v2/point/%.4f/%.4f/%d", lat, lon, distNM),
	}
	var lastErr error
	for _, u := range urls {
		flights, err := fetchADSBLolURL(ctx, client, u, "ADSB", false)
		if err == nil {
			return flights, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// FetchADSBLolMil fetches live military aircraft worldwide from adsb.lol.
func FetchADSBLolMil(ctx context.Context, client *http.Client) ([]model.Flight, error) {
	urls := []string{
		"https://api.adsb.lol/v2/mil",
		"https://api.airplanes.live/v2/mil",
	}
	var lastErr error
	for _, u := range urls {
		flights, err := fetchADSBLolURL(ctx, client, u, "MIL", true)
		if err == nil {
			return flights, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func fetchADSBLolURL(ctx context.Context, client *http.Client, url, source string, forceMil bool) ([]model.Flight, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "GokRadar/1.0 (terminal-flight-tracker)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s HTTP %d", source, resp.StatusCode)
	}

	var raw adsbLolResp
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return ParseADSBLolAC(raw.AC, source, forceMil), nil
}

// ParseADSBLolAC converts adsb.lol / airplanes.live aircraft JSON objects into Flight structs.
func ParseADSBLolAC(list []adsbLolAC, source string, forceMil bool) []model.Flight {
	now := time.Now()
	out := make([]model.Flight, 0, len(list))
	for _, ac := range list {
		if ac.Lat == 0 && ac.Lon == 0 {
			continue
		}
		hex := strings.ToLower(strings.TrimSpace(ac.Hex))
		if hex == "" {
			continue
		}
		altFt, onGround := parseAltBaro(ac.AltBaro)
		if altFt == 0 && !onGround {
			altFt, _ = parseAltBaro(ac.AltGeom)
		}
		vrate := ac.BaroRate
		if vrate == 0 {
			vrate = ac.GeomRate
		}
		isMil := forceMil || (ac.DBFlags&1) != 0

		out = append(out, model.Flight{
			Hex:          hex,
			Callsign:     strings.TrimSpace(ac.Flight),
			Registration: strings.TrimSpace(ac.Reg),
			AircraftType: strings.TrimSpace(ac.Type),
			Lat:          ac.Lat,
			Lon:          ac.Lon,
			AltFeet:      altFt,
			OnGround:     onGround,
			SpeedKt:      ac.GS,
			TrackDeg:     ac.Track,
			VertRateFPM:  vrate,
			Squawk:       strings.TrimSpace(ac.Squawk),
			IsMilitary:   isMil,
			Source:       source,
			LastSeen:     now,
		})
	}
	return out
}

func parseAltBaro(v any) (int, bool) {
	if v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case string:
		if strings.EqualFold(strings.TrimSpace(t), "ground") {
			return 0, true
		}
		return 0, false
	case float64:
		return int(t), false
	case int:
		return t, false
	default:
		return 0, false
	}
}
