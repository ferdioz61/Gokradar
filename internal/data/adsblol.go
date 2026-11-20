package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
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

// GlobalHubs covers major global aviation corridors (250 NM radius each) so worldwide traffic
// remains rich across all continents even when OpenSky anonymous API returns HTTP 429.
var GlobalHubs = [][2]float64{
	{41.0, 29.0},   // Türkiye Batı / Balkanlar (IST)
	{38.5, 38.0},   // Türkiye Doğu / Kafkasya
	{50.1, 8.6},    // Orta & Batı Avrupa (FRA)
	{51.5, -0.5},   // Birleşik Krallık / Kuzey Denizi (LHR)
	{41.9, 12.5},   // Güney Avrupa / İtalya (FCO)
	{40.4, -3.7},   // İberya (MAD)
	{52.2, 21.0},   // Doğu Avrupa (WAW)
	{55.8, 37.6},   // Rusya Batı (MOW)
	{25.2, 55.3},   // Orta Doğu / Körfez (DXB)
	{30.1, 31.4},   // Kuzey Afrika (CAI)
	{-26.1, 28.2},  // Güney Afrika (JNB)
	{28.6, 77.1},   // Hindistan (DEL)
	{31.2, 121.5},  // Çin Doğu (PVG)
	{35.7, 139.8},  // Japonya / Uzak Doğu (TYO)
	{1.3, 103.9},   // Güneydoğu Asya (SIN)
	{-33.9, 151.2}, // Avustralya (SYD)
	{40.7, -74.0},  // ABD Doğu (NYC)
	{41.9, -87.9},  // ABD Orta (ORD)
	{33.9, -118.4}, // ABD Batı (LAX)
	{43.7, -79.6},  // Kanada (YYZ)
	{-23.4, -46.5}, // Güney Amerika / Brezilya (GRU)
	{-34.8, -58.5}, // Güney Amerika / Arjantin (EZE)
}

// FetchADSBLolPoint fetches rich ADS-B aircraft within distNM (max 250) of (lat, lon).
// Uses adsb.lol and falls back to opendata.adsb.fi if needed.
func FetchADSBLolPoint(ctx context.Context, client *http.Client, lat, lon float64, distNM int) ([]model.Flight, error) {
	if distNM > 250 {
		distNM = 250
	}
	if distNM < 25 {
		distNM = 25
	}
	urls := []string{
		fmt.Sprintf("https://api.adsb.lol/v2/point/%.4f/%.4f/%d", lat, lon, distNM),
		fmt.Sprintf("https://opendata.adsb.fi/api/v2/lat/%.4f/lon/%.4f/dist/%d", lat, lon, distNM),
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

// FetchADSBLolMultiPoint queries multiple geographic centers concurrently (capped at 4 workers)
// and deduplicates results by ICAO24 hex.
func FetchADSBLolMultiPoint(ctx context.Context, client *http.Client, points [][2]float64, distNM int) ([]model.Flight, error) {
	if len(points) == 0 {
		return nil, nil
	}
	if distNM > 250 {
		distNM = 250
	}
	if distNM < 25 {
		distNM = 25
	}

	const maxWorkers = 4
	sem := make(chan struct{}, maxWorkers)
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		byHex   = make(map[string]model.Flight, len(points)*120)
		okCount int
		lastErr error
	)

	for idx, pt := range points {
		wg.Add(1)
		go func(i int, lat, lon float64) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// Alternate primary provider between adsb.lol and adsb.fi to distribute load cleanly
			var urls []string
			if i%2 == 0 {
				urls = []string{
					fmt.Sprintf("https://api.adsb.lol/v2/point/%.4f/%.4f/%d", lat, lon, distNM),
					fmt.Sprintf("https://opendata.adsb.fi/api/v2/lat/%.4f/lon/%.4f/dist/%d", lat, lon, distNM),
				}
			} else {
				urls = []string{
					fmt.Sprintf("https://opendata.adsb.fi/api/v2/lat/%.4f/lon/%.4f/dist/%d", lat, lon, distNM),
					fmt.Sprintf("https://api.adsb.lol/v2/point/%.4f/%.4f/%d", lat, lon, distNM),
				}
			}

			var fl []model.Flight
			var err error
			for _, u := range urls {
				fl, err = fetchADSBLolURL(ctx, client, u, "ADSB", false)
				if err == nil {
					break
				}
			}

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				lastErr = err
				return
			}
			okCount++
			for _, f := range fl {
				byHex[f.Hex] = f
			}
		}(idx, pt[0], pt[1])
	}

	wg.Wait()
	if okCount == 0 && lastErr != nil {
		return nil, lastErr
	}
	out := make([]model.Flight, 0, len(byHex))
	for _, f := range byHex {
		out = append(out, f)
	}
	return out, nil
}

// FetchGlobalADSB sweeps major global aviation hubs via adsb.lol and adsb.fi.
func FetchGlobalADSB(ctx context.Context, client *http.Client) ([]model.Flight, error) {
	return FetchADSBLolMultiPoint(ctx, client, GlobalHubs, 250)
}

// FetchADSBLolMil fetches live military aircraft worldwide from adsb.lol (fallback: adsb.fi).
func FetchADSBLolMil(ctx context.Context, client *http.Client) ([]model.Flight, error) {
	urls := []string{
		"https://api.adsb.lol/v2/mil",
		"https://opendata.adsb.fi/api/v2/mil",
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
	req.Header.Set("User-Agent", "GokRadar/0.2 (terminal-flight-tracker)")
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

// ParseADSBLolAC converts adsb.lol / adsb.fi aircraft JSON objects into Flight structs.
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
