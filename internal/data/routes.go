package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gokradar/internal/model"
)

// RouteCache stores resolved origin/destination routes with TTL.
type RouteCache struct {
	mu    sync.RWMutex
	items map[string]*model.RouteInfo
}

func NewRouteCache() *RouteCache {
	return &RouteCache{
		items: make(map[string]*model.RouteInfo),
	}
}

func (c *RouteCache) Get(callsign string) (*model.RouteInfo, bool) {
	cs := strings.ToUpper(strings.TrimSpace(callsign))
	if cs == "" {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	info, ok := c.items[cs]
	if !ok {
		return nil, false
	}
	ttl := 30 * time.Minute
	if info.NotFound {
		ttl = 10 * time.Minute
	}
	if time.Since(info.FetchedAt) > ttl {
		return nil, false
	}
	return info, true
}

func (c *RouteCache) Set(callsign string, info *model.RouteInfo) {
	cs := strings.ToUpper(strings.TrimSpace(callsign))
	if cs == "" || info == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[cs] = info
}

type adsbdbResp struct {
	Response struct {
		Flightroute *struct {
			Callsign string `json:"callsign"`
			Airline  *struct {
				Name string `json:"name"`
				IATA string `json:"iata"`
				ICAO string `json:"icao"`
			} `json:"airline"`
			Origin *struct {
				IATA         string  `json:"iata_code"`
				ICAO         string  `json:"icao_code"`
				Name         string  `json:"name"`
				Municipality string  `json:"municipality"`
				Lat          float64 `json:"latitude"`
				Lon          float64 `json:"longitude"`
			} `json:"origin"`
			Destination *struct {
				IATA         string  `json:"iata_code"`
				ICAO         string  `json:"icao_code"`
				Name         string  `json:"name"`
				Municipality string  `json:"municipality"`
				Lat          float64 `json:"latitude"`
				Lon          float64 `json:"longitude"`
			} `json:"destination"`
		} `json:"flightroute"`
	} `json:"response"`
}

// FetchRoute resolves origin, destination, and airline for a callsign via api.adsbdb.com.
func FetchRoute(ctx context.Context, client *http.Client, callsign string) (*model.RouteInfo, error) {
	cs := strings.ToUpper(strings.TrimSpace(callsign))
	if cs == "" {
		return &model.RouteInfo{Callsign: cs, NotFound: true, FetchedAt: time.Now()}, nil
	}
	endpoint := fmt.Sprintf("https://api.adsbdb.com/v0/callsign/%s", url.PathEscape(cs))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	if resp.StatusCode == http.StatusNotFound {
		return &model.RouteInfo{Callsign: cs, NotFound: true, FetchedAt: time.Now()}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("adsbdb HTTP %d", resp.StatusCode)
	}

	var raw adsbdbResp
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	fr := raw.Response.Flightroute
	if fr == nil {
		return &model.RouteInfo{Callsign: cs, NotFound: true, FetchedAt: time.Now()}, nil
	}

	info := &model.RouteInfo{
		Callsign:  cs,
		FetchedAt: time.Now(),
	}
	if fr.Airline != nil {
		info.AirlineName = fr.Airline.Name
		info.AirlineIATA = fr.Airline.IATA
	}
	if fr.Origin != nil {
		info.OriginIATA = fr.Origin.IATA
		info.OriginICAO = fr.Origin.ICAO
		info.OriginName = fr.Origin.Name
		info.OriginCity = fr.Origin.Municipality
		info.OriginLat = fr.Origin.Lat
		info.OriginLon = fr.Origin.Lon
	}
	if fr.Destination != nil {
		info.DestIATA = fr.Destination.IATA
		info.DestICAO = fr.Destination.ICAO
		info.DestName = fr.Destination.Name
		info.DestCity = fr.Destination.Municipality
		info.DestLat = fr.Destination.Lat
		info.DestLon = fr.Destination.Lon
	}
	if info.ShortRoute() == "" {
		info.NotFound = true
	}
	return info, nil
}
