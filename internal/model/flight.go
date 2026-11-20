package model

import (
	"fmt"
	"strings"
	"time"
)

// Point represents a geographic coordinate with optional altitude for trail coloring.
type Point struct {
	Lat     float64
	Lon     float64
	AltFeet int
}

// Airport represents a major airport marker.
type Airport struct {
	IATA     string  `json:"iata"`
	ICAO     string  `json:"icao"`
	Name     string  `json:"name"`
	City     string  `json:"city"`
	Country  string  `json:"country"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Priority int     `json:"priority"`
}

// RouteInfo holds origin and destination details resolved from a callsign.
type RouteInfo struct {
	Callsign    string
	AirlineName string
	AirlineIATA string
	OriginIATA  string
	OriginICAO  string
	OriginName  string
	OriginCity  string
	OriginLat   float64
	OriginLon   float64
	DestIATA    string
	DestICAO    string
	DestName    string
	DestCity    string
	DestLat     float64
	DestLon     float64
	FetchedAt   time.Time
	NotFound    bool
}

// ShortRoute returns a compact "IST → FRA" label.
func (r *RouteInfo) ShortRoute() string {
	if r == nil || r.NotFound {
		return ""
	}
	orig := r.OriginIATA
	if orig == "" {
		orig = r.OriginICAO
	}
	dest := r.DestIATA
	if dest == "" {
		dest = r.DestICAO
	}
	if orig == "" && dest == "" {
		return ""
	}
	if orig == "" {
		orig = "?"
	}
	if dest == "" {
		dest = "?"
	}
	return orig + " → " + dest
}

// HasCoords returns true if both origin and destination have valid coordinates.
func (r *RouteInfo) HasCoords() bool {
	if r == nil || r.NotFound {
		return false
	}
	return (r.OriginLat != 0 || r.OriginLon != 0) && (r.DestLat != 0 || r.DestLon != 0)
}

// Flight represents a single tracked aircraft.
type Flight struct {
	Hex          string  // ICAO24 hex address (lowercase)
	Callsign     string  // Trimmed callsign (e.g. THY1831)
	Country      string  // Origin country
	Registration string  // Tail number (e.g. TC-JVA)
	AircraftType string  // ICAO type code (e.g. B738, A321)
	Lat          float64 // Degrees
	Lon          float64 // Degrees
	AltFeet      int     // Barometric altitude in feet
	OnGround     bool
	SpeedKt      float64 // Ground speed in knots
	TrackDeg     float64 // True track 0..360
	VertRateFPM  int     // Vertical rate in feet per minute
	Squawk       string  // 4-digit transponder code
	IsMilitary   bool
	Source       string // "OS" (OpenSky), "ADSB" (adsb.lol/airplanes.live), "MIL"
	LastSeen     time.Time
}

// DisplayID returns callsign if available, otherwise uppercase hex.
func (f Flight) DisplayID() string {
	if f.Callsign != "" {
		return f.Callsign
	}
	return strings.ToUpper(f.Hex)
}

// IsEmergency returns true if the aircraft is squawking an emergency code.
func (f Flight) IsEmergency() bool {
	return f.Squawk == "7700" || f.Squawk == "7600" || f.Squawk == "7500"
}

// EmergencyLabel returns a Turkish description for emergency squawks.
func (f Flight) EmergencyLabel() string {
	switch f.Squawk {
	case "7700":
		return "ACİL DURUM (7700)"
	case "7600":
		return "TELSİZ ARIZASI (7600)"
	case "7500":
		return "KAÇIRILMA (7500)"
	default:
		return ""
	}
}

// FormatAlt formats altitude in aviation (ft) or metric (m) units.
func FormatAlt(feet int, onGround bool, metric bool) string {
	if onGround {
		return "YER"
	}
	if feet <= 0 {
		return "-"
	}
	if metric {
		m := int(float64(feet) * 0.3048)
		return fmt.Sprintf("%d m", m)
	}
	return fmt.Sprintf("%d ft", feet)
}

// FormatSpeed formats speed in knots or km/h.
func FormatSpeed(kt float64, metric bool) string {
	if kt <= 0 {
		return "-"
	}
	if metric {
		kmh := int(kt * 1.852)
		return fmt.Sprintf("%d km/s", kmh)
	}
	return fmt.Sprintf("%d kt", int(kt))
}

// FormatVertRate formats vertical rate in fpm or m/s.
func FormatVertRate(fpm int, metric bool) string {
	if fpm == 0 {
		return "─ 0"
	}
	arrow := "▲"
	if fpm < 0 {
		arrow = "▼"
	}
	if metric {
		ms := float64(fpm) * 0.00508
		return fmt.Sprintf("%s %+.1f m/s", arrow, ms)
	}
	return fmt.Sprintf("%s %+d fpm", arrow, fpm)
}

// HeadingArrow returns a directional arrow (Unicode or ASCII) for a heading in degrees.
func HeadingArrow(deg float64, ascii bool) string {
	idx := headingIndex(deg)
	if ascii {
		arrows := []string{"^", "/", ">", "\\", "v", "/", "<", "\\"}
		return arrows[idx]
	}
	arrows := []string{"↑", "↗", "→", "↘", "↓", "↙", "←", "↖"}
	return arrows[idx]
}

// HeadingSymbol returns a bold, crisp directional aircraft marker for the radar map.
func HeadingSymbol(deg float64, ascii, onGround bool) string {
	if onGround {
		if ascii {
			return "o"
		}
		return "▪"
	}
	idx := headingIndex(deg)
	if ascii {
		arrows := []string{"^", "/", ">", "\\", "v", "/", "<", "\\"}
		return arrows[idx]
	}
	// Solid directional triangles stand out clearly over Braille coastlines/routes.
	symbols := []string{"▲", "◥", "▶", "◢", "▼", "◣", "◀", "◤"}
	return symbols[idx]
}

func headingIndex(deg float64) int {
	d := deg
	for d < 0 {
		d += 360
	}
	for d >= 360 {
		d -= 360
	}
	return int((d+22.5)/45.0) % 8
}

// FormatATCAlt formats altitude compactly for map ATC data blocks (e.g. FL350, 8.2k, 10.7km).
func FormatATCAlt(feet int, onGround, metric bool) string {
	if onGround {
		return "GND"
	}
	if feet <= 0 {
		return ""
	}
	if metric {
		m := float64(feet) * 0.3048
		if m >= 1000 {
			return fmt.Sprintf("%.1fkm", m/1000.0)
		}
		return fmt.Sprintf("%dm", int(m))
	}
	if feet >= 10000 {
		return fmt.Sprintf("FL%03d", (feet+50)/100)
	}
	return fmt.Sprintf("%.1fk", float64(feet)/1000.0)
}
