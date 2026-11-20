package data

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"gokradar/internal/model"
)

const openSkyAllURL = "https://opensky-network.org/api/states/all"

type openSkyResp struct {
	Time   int64   `json:"time"`
	States [][]any `json:"states"`
}

// FetchOpenSkyGlobal fetches all live aircraft worldwide from OpenSky Network.
func FetchOpenSkyGlobal(ctx context.Context, client *http.Client) ([]model.Flight, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openSkyAllURL, nil)
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

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("OpenSky hız limiti (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenSky HTTP %d", resp.StatusCode)
	}

	var raw openSkyResp
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return ParseOpenSkyStates(raw.States), nil
}

// ParseOpenSkyStates converts raw OpenSky state vectors into Flight structs.
func ParseOpenSkyStates(states [][]any) []model.Flight {
	now := time.Now()
	flights := make([]model.Flight, 0, len(states))
	for _, s := range states {
		if len(s) < 15 {
			continue
		}
		lon, okLon := toFloat(s[5])
		lat, okLat := toFloat(s[6])
		if !okLon || !okLat {
			continue
		}
		hex := strings.ToLower(strings.TrimSpace(toString(s[0])))
		if hex == "" {
			continue
		}
		callsign := strings.TrimSpace(toString(s[1]))
		country := strings.TrimSpace(toString(s[2]))
		baroAltM, hasBaro := toFloat(s[7])
		geoAltM, hasGeo := toFloat(s[13])
		onGround, _ := s[8].(bool)
		velMS, _ := toFloat(s[9])
		trackDeg, _ := toFloat(s[10])
		vertMS, _ := toFloat(s[11])
		squawk := strings.TrimSpace(toString(s[14]))

		altM := baroAltM
		if !hasBaro && hasGeo {
			altM = geoAltM
		}
		altFt := int(math.Round(altM * 3.28084))
		if onGround {
			altFt = 0
		}

		flights = append(flights, model.Flight{
			Hex:         hex,
			Callsign:    callsign,
			Country:     country,
			Lat:         lat,
			Lon:         lon,
			AltFeet:     altFt,
			OnGround:    onGround,
			SpeedKt:     velMS * 1.94384,
			TrackDeg:    trackDeg,
			VertRateFPM: int(math.Round(vertMS * 196.8504)),
			Squawk:      squawk,
			Source:      "OS",
			LastSeen:    now,
		})
	}
	return flights
}

func toFloat(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
