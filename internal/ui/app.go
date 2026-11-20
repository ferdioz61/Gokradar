package ui

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gokradar/internal/config"
	"gokradar/internal/data"
	"gokradar/internal/geo"
	"gokradar/internal/model"
)

// Options configures initial startup behavior from CLI flags.
type Options struct {
	ASCII  bool
	Metric bool
	Region string
}

type focusArea int

const (
	focusList focusArea = iota
	focusMap
)

type altFilter int

const (
	altAll altFilter = iota
	altHigh          // >= 30,000 ft
	altMid           // 10,000 - 30,000 ft
	altLow           // < 10,000 ft
)

func (a altFilter) Label() string {
	switch a {
	case altHigh:
		return ">30k"
	case altMid:
		return "10k-30k"
	case altLow:
		return "<10k"
	default:
		return ""
	}
}

// Messages
type tickMsg time.Time
type animTickMsg time.Time

type globalDataMsg struct {
	flights      []model.Flight
	usedFallback bool
	openSky429   bool
	err          error
}

type regionalDataMsg struct {
	flights []model.Flight
	err     error
}

type milDataMsg struct {
	flights []model.Flight
	err     error
}

type routeDataMsg struct {
	callsign string
	info     *model.RouteInfo
	err      error
}

// AppModel is the root Bubble Tea model for GökRadar.
type AppModel struct {
	width  int
	height int

	world     *data.WorldData
	client    *http.Client
	routes    *data.RouteCache
	watchlist *config.Watchlist

	vp         geo.Viewport
	focus      focusArea
	fullscreen bool
	showHelp   bool
	showStats  bool
	ascii      bool
	metric     bool

	// Visual & radar layer modes
	labelMode   LabelMode
	trailMode   TrailMode
	showRings   bool
	showVectors bool
	followMode  bool
	paused      bool
	themeIdx    int

	// Mouse interaction state
	dragging      bool
	dragLastX     int
	dragLastY     int
	dragMoved     bool
	lastClickTime time.Time
	lastClickX    int
	lastClickY    int

	// Raw aircraft store keyed by ICAO24 hex
	flightsByHex map[string]model.Flight
	trails       map[string][]model.Point

	// Filtered & sorted view
	filtered    []model.Flight
	cursor      int
	selectedHex string

	// Filters & search
	searching   bool
	searchQuery string
	altMode     altFilter
	onlyMil     bool
	onlyEmerg   bool
	onlyFav     bool

	// Status & async state
	loadingGlobal      bool
	openSkyCooldownEnd time.Time
	routeLoading       string
	lastUpdate         time.Time
	statusMsg          string
	globalCount        int
	milCount           int
	sourceLabel        string
}

// NewAppModel creates the initialized Bubble Tea model.
func NewAppModel(world *data.WorldData, opts Options) AppModel {
	vp := geo.Presets[0].Viewport // Default: DÜNYA
	if opts.Region != "" {
		r := strings.ToUpper(strings.TrimSpace(opts.Region))
		for _, p := range geo.Presets {
			if strings.Contains(p.Name, r) || p.Key == r ||
				(r == "TR" && p.Key == "2") ||
				(r == "EU" && p.Key == "3") ||
				(r == "US" && p.Key == "5") {
				vp = p.Viewport
				break
			}
		}
	}

	return AppModel{
		width:         120,
		height:        34,
		world:         world,
		client:        &http.Client{Timeout: 18 * time.Second},
		routes:        data.NewRouteCache(),
		watchlist:     config.LoadWatchlist(),
		vp:            vp,
		focus:         focusList,
		ascii:         opts.ASCII,
		metric:        opts.Metric,
		labelMode:     LabelSmart,
		trailMode:     TrailSelected,
		showRings:     false,
		showVectors:   true,
		flightsByHex:  make(map[string]model.Flight, 12000),
		trails:        make(map[string][]model.Point, 4096),
		loadingGlobal: true,
		statusMsg:     "Dünya geneli uçuş verileri yükleniyor...",
		sourceLabel:   "Yükleniyor",
	}
}

func (m AppModel) activeTheme() Theme {
	if m.themeIdx >= 0 && m.themeIdx < len(Themes) {
		return Themes[m.themeIdx]
	}
	return Themes[0]
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchGlobalCmd(),
		m.fetchMilCmd(),
		m.fetchRegionalCmd(),
		tickCmd(),
		animTickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(15*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func animTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return animTickMsg(t)
	})
}

func (m AppModel) fetchGlobalCmd() tea.Cmd {
	client := m.client
	skipOpenSky := time.Now().Before(m.openSkyCooldownEnd)
	return func() tea.Msg {
		if !skipOpenSky {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			fl, err := data.FetchOpenSkyGlobal(ctx, client)
			cancel()
			if err == nil && len(fl) > 0 {
				return globalDataMsg{flights: fl}
			}
			is429 := err != nil && strings.Contains(err.Error(), "429")
			// Automatic worldwide multi-hub fallback via adsb.lol + adsb.fi
			ctx2, cancel2 := context.WithTimeout(context.Background(), 18*time.Second)
			defer cancel2()
			fb, fbErr := data.FetchGlobalADSB(ctx2, client)
			return globalDataMsg{flights: fb, usedFallback: true, openSky429: is429, err: fbErr}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
		defer cancel()
		fb, err := data.FetchGlobalADSB(ctx, client)
		return globalDataMsg{flights: fb, usedFallback: true, err: err}
	}
}

func (m AppModel) fetchMilCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		fl, err := data.FetchADSBLolMil(ctx, client)
		return milDataMsg{flights: fl, err: err}
	}
}

func (m AppModel) fetchRegionalCmd() tea.Cmd {
	client := m.client
	vp := m.vp
	mapW, mapH, _ := m.mapDimensions()
	// For very wide world view, global hub sweep handles everything; otherwise query viewport grid
	if vp.SpanLon > 110 {
		return nil
	}
	pts := [][2]float64{{vp.CenterLat, vp.CenterLon}}
	if vp.SpanLon > 16 {
		minLat, maxLat, minLon, maxLon := vp.Bounds(mapW, mapH)
		dLat := (maxLat - minLat) * 0.25
		dLon := (maxLon - minLon) * 0.25
		pts = append(pts,
			[2]float64{vp.CenterLat + dLat, vp.CenterLon - dLon},
			[2]float64{vp.CenterLat + dLat, vp.CenterLon + dLon},
			[2]float64{vp.CenterLat - dLat, vp.CenterLon - dLon},
			[2]float64{vp.CenterLat - dLat, vp.CenterLon + dLon},
		)
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
		defer cancel()
		fl, err := data.FetchADSBLolMultiPoint(ctx, client, pts, 250)
		return regionalDataMsg{flights: fl, err: err}
	}
}

func (m AppModel) fetchRouteCmd(callsign string) tea.Cmd {
	client := m.client
	cs := strings.ToUpper(strings.TrimSpace(callsign))
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		info, err := data.FetchRoute(ctx, client, cs)
		return routeDataMsg{callsign: cs, info: info, err: err}
	}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildFiltered()
		return m, nil

	case tickMsg:
		cmds := []tea.Cmd{
			tickCmd(),
			m.fetchGlobalCmd(),
			m.fetchMilCmd(),
			m.fetchRegionalCmd(),
		}
		m.loadingGlobal = true
		return m, tea.Batch(cmds...)

	case animTickMsg:
		if !m.paused && len(m.flightsByHex) > 0 {
			m.advanceDeadReckoning(2.0)
			if m.followMode {
				m.syncFollowCamera()
			}
			m.rebuildFiltered()
		}
		return m, animTickCmd()

	case globalDataMsg:
		m.loadingGlobal = false
		if msg.openSky429 {
			m.openSkyCooldownEnd = time.Now().Add(5 * time.Minute)
		}
		if msg.err != nil {
			m.statusMsg = "Küresel veri uyarısı: " + msg.err.Error() + " (bölgesel/askeri akış aktif)"
		} else {
			m.mergeFlights(msg.flights, true)
			m.globalCount = len(msg.flights)
			m.lastUpdate = time.Now()
			if msg.usedFallback {
				m.sourceLabel = "ADSB.lol+fi Global"
			} else {
				m.sourceLabel = "OpenSky+ADSB"
			}
			m.statusMsg = fmt.Sprintf("Güncellendi (%s) — %d aktif uçak", m.lastUpdate.Format("15:04:05"), len(m.flightsByHex))
		}
		if m.followMode {
			m.syncFollowCamera()
		}
		m.rebuildFiltered()
		return m, m.maybeFetchSelectedRoute()

	case regionalDataMsg:
		if msg.err == nil && len(msg.flights) > 0 {
			m.mergeFlights(msg.flights, false)
			m.lastUpdate = time.Now()
			if m.sourceLabel == "Yükleniyor" {
				m.sourceLabel = "ADSB.lol"
			}
			if m.followMode {
				m.syncFollowCamera()
			}
			m.rebuildFiltered()
			return m, m.maybeFetchSelectedRoute()
		}
		return m, nil

	case milDataMsg:
		if msg.err == nil {
			m.milCount = len(msg.flights)
			m.mergeFlights(msg.flights, false)
			if m.followMode {
				m.syncFollowCamera()
			}
			m.rebuildFiltered()
			return m, m.maybeFetchSelectedRoute()
		}
		return m, nil

	case routeDataMsg:
		if msg.callsign == m.routeLoading {
			m.routeLoading = ""
		}
		if msg.err == nil && msg.info != nil {
			m.routes.Set(msg.callsign, msg.info)
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// advanceDeadReckoning smoothly interpolates airborne aircraft positions between API refreshes.
func (m *AppModel) advanceDeadReckoning(dtSec float64) {
	for hex, f := range m.flightsByHex {
		if f.OnGround || f.SpeedKt < 30 {
			continue
		}
		nLat, nLon := geo.ProjectPoint(f.Lat, f.Lon, f.SpeedKt, f.TrackDeg, dtSec)
		f.Lat = nLat
		f.Lon = nLon
		m.flightsByHex[hex] = f
	}
}

func (m *AppModel) syncFollowCamera() {
	if m.selectedHex == "" {
		return
	}
	if f, ok := m.flightsByHex[m.selectedHex]; ok {
		m.vp.CenterLat = f.Lat
		m.vp.CenterLon = f.Lon
		m.vp.Name = "KİLİT:" + f.DisplayID()
	}
}

func (m *AppModel) mergeFlights(incoming []model.Flight, pruneOldOpenSky bool) {
	now := time.Now()
	if pruneOldOpenSky {
		for hex, existing := range m.flightsByHex {
			if now.Sub(existing.LastSeen) > 90*time.Second {
				delete(m.flightsByHex, hex)
				delete(m.trails, hex)
			}
		}
	}

	for _, f := range incoming {
		if existing, ok := m.flightsByHex[f.Hex]; ok {
			if f.Registration == "" {
				f.Registration = existing.Registration
			}
			if f.AircraftType == "" {
				f.AircraftType = existing.AircraftType
			}
			if f.Country == "" {
				f.Country = existing.Country
			}
			if f.Callsign == "" {
				f.Callsign = existing.Callsign
			}
			if f.Squawk == "" {
				f.Squawk = existing.Squawk
			}
			if existing.IsMilitary {
				f.IsMilitary = true
			}
		}
		m.flightsByHex[f.Hex] = f

		// Append to altitude-tagged trail if moved noticeably
		tr := m.trails[f.Hex]
		pt := model.Point{Lat: f.Lat, Lon: f.Lon, AltFeet: f.AltFeet}
		if len(tr) == 0 {
			m.trails[f.Hex] = []model.Point{pt}
		} else {
			last := tr[len(tr)-1]
			if (last.Lat-pt.Lat)*(last.Lat-pt.Lat)+(last.Lon-pt.Lon)*(last.Lon-pt.Lon) > 0.0001 {
				tr = append(tr, pt)
				if len(tr) > 120 {
					tr = tr[len(tr)-120:]
				}
				m.trails[f.Hex] = tr
			}
		}
	}
}

func (m *AppModel) mapDimensions() (mapW, mapH, listW int) {
	bodyH := m.height - 7
	if bodyH < 6 {
		bodyH = 6
	}
	if m.fullscreen || m.width < 68 {
		return m.width - 2, bodyH, 0
	}
	listW = 36
	if m.width < 100 {
		listW = 28
	}
	mapW = m.width - listW - 5
	if mapW < 20 {
		mapW = 20
	}
	return mapW, bodyH, listW
}

// FindNearestFlightAt returns the index in flights closest to terminal map cell (cx, cy) within maxDistCells.
func FindNearestFlightAt(flights []model.Flight, vp geo.Viewport, mapW, mapH, cx, cy int, maxDistCells float64) int {
	bestIdx := -1
	bestDist := maxDistCells + 0.001
	fw, fh := float64(mapW-1), float64(mapH-1)
	for i := range flights {
		x, y, vis := vp.Project(flights[i].Lat, flights[i].Lon, mapW, mapH, fw, fh)
		if !vis {
			continue
		}
		// Weight vertical distance slightly more because terminal cells are ~2x taller than wide
		dx := x - float64(cx)
		dy := (y - float64(cy)) * 1.4
		d := math.Hypot(dx, dy)
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx
}

func (m AppModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Close modals on click
	if m.showHelp || m.showStats {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			m.showHelp = false
			m.showStats = false
		}
		return m, nil
	}

	mapW, mapH, listW := m.mapDimensions()
	// Layout geometry:
	// Row 0: header bar
	// Row 1: top border of mapBox & listBox
	// Rows 2..mapH+1: inner content of mapBox (cols 1..mapW) and listBox (cols mapW+3..mapW+2+listW)
	inMap := msg.X >= 1 && msg.X <= mapW && msg.Y >= 2 && msg.Y <= mapH+1
	inList := listW > 0 && msg.X >= mapW+3 && msg.X <= mapW+2+listW && msg.Y >= 2 && msg.Y <= mapH+1

	switch {
	// Wheel scroll
	case msg.Button == tea.MouseButtonWheelUp:
		if inMap {
			m.focus = focusMap
			if m.followMode && m.selectedHex != "" {
				m.vp.Zoom(0.75)
				m.syncFollowCamera()
			} else {
				cx := float64(msg.X - 1)
				cy := float64(msg.Y - 2)
				m.vp.ZoomAt(0.75, cx, cy, mapW, mapH)
			}
			m.rebuildFiltered()
			if m.vp.SpanLon < 90 {
				return m, m.fetchRegionalCmd()
			}
			return m, nil
		}
		if inList && len(m.filtered) > 0 {
			m.focus = focusList
			m.cursor -= 3
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.selectedHex = m.filtered[m.cursor].Hex
			return m, m.maybeFetchSelectedRoute()
		}
		return m, nil

	case msg.Button == tea.MouseButtonWheelDown:
		if inMap {
			m.focus = focusMap
			if m.followMode && m.selectedHex != "" {
				m.vp.Zoom(1.33)
				m.syncFollowCamera()
			} else {
				cx := float64(msg.X - 1)
				cy := float64(msg.Y - 2)
				m.vp.ZoomAt(1.33, cx, cy, mapW, mapH)
			}
			m.rebuildFiltered()
			if m.vp.SpanLon < 110 {
				return m, m.fetchRegionalCmd()
			}
			return m, nil
		}
		if inList && len(m.filtered) > 0 {
			m.focus = focusList
			m.cursor += 3
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			m.selectedHex = m.filtered[m.cursor].Hex
			return m, m.maybeFetchSelectedRoute()
		}
		return m, nil

	// Right click on map: toggle follow lock on aircraft, or center map on clicked point
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonRight:
		if inMap {
			m.focus = focusMap
			cx := msg.X - 1
			cy := msg.Y - 2
			idx := FindNearestFlightAt(m.filtered, m.vp, mapW, mapH, cx, cy, 3.0)
			if idx >= 0 {
				m.cursor = idx
				m.selectedHex = m.filtered[idx].Hex
				m.followMode = !m.followMode
				if m.followMode {
					m.syncFollowCamera()
					if m.vp.SpanLon > 24 {
						m.vp.SpanLon = 16
					}
					m.statusMsg = m.filtered[idx].DisplayID() + " canlı takip kilidine alındı"
				} else {
					m.vp.Name = "ÖZEL"
					m.statusMsg = "Canlı takip kilidi kapatıldı"
				}
				m.rebuildFiltered()
				return m, tea.Batch(m.fetchRegionalCmd(), m.maybeFetchSelectedRoute())
			}
			lat, lon := m.vp.Unproject(float64(cx), float64(cy), mapW, mapH)
			m.followMode = false
			m.vp.CenterLat = lat
			m.vp.CenterLon = lon
			m.vp.Name = "ÖZEL"
			m.rebuildFiltered()
			return m, m.fetchRegionalCmd()
		}
		return m, nil

	// Left button press: start drag (in map) or select row (in list)
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		now := time.Now()
		isDouble := now.Sub(m.lastClickTime) < 420*time.Millisecond &&
			absInt(msg.X-m.lastClickX) <= 2 && absInt(msg.Y-m.lastClickY) <= 1
		m.lastClickTime = now
		m.lastClickX = msg.X
		m.lastClickY = msg.Y

		if inMap {
			m.focus = focusMap
			m.dragging = true
			m.dragLastX = msg.X
			m.dragLastY = msg.Y
			m.dragMoved = false
			if isDouble {
				m.dragging = false
				cx := msg.X - 1
				cy := msg.Y - 2
				idx := FindNearestFlightAt(m.filtered, m.vp, mapW, mapH, cx, cy, 3.2)
				if idx >= 0 {
					m.cursor = idx
					m.selectedHex = m.filtered[idx].Hex
					m.followMode = true
					m.syncFollowCamera()
					if m.vp.SpanLon > 22 {
						m.vp.SpanLon = 14
					}
					m.statusMsg = m.filtered[idx].DisplayID() + " canlı takip kilidine alındı"
					m.rebuildFiltered()
					return m, tea.Batch(m.fetchRegionalCmd(), m.maybeFetchSelectedRoute())
				}
				// Double-click empty map: center & zoom in
				m.followMode = false
				m.vp.ZoomAt(0.55, float64(cx), float64(cy), mapW, mapH)
				m.rebuildFiltered()
				return m, m.fetchRegionalCmd()
			}
			return m, nil
		}

		if inList && len(m.filtered) > 0 {
			m.focus = focusList
			// First 2 rows inside listBox are title and column header
			relRow := (msg.Y - 2) - 2
			rowsAvail := mapH - 2
			if relRow >= 0 && relRow < rowsAvail {
				start, end := ListScrollWindow(len(m.filtered), m.cursor, rowsAvail)
				idx := start + relRow
				if idx >= start && idx < end {
					m.cursor = idx
					m.selectedHex = m.filtered[idx].Hex
					if isDouble {
						m.followMode = true
						m.syncFollowCamera()
						if m.vp.SpanLon > 24 {
							m.vp.SpanLon = 16
						}
						m.rebuildFiltered()
						return m, tea.Batch(m.fetchRegionalCmd(), m.maybeFetchSelectedRoute())
					}
					return m, m.maybeFetchSelectedRoute()
				}
			}
		}
		return m, nil

	// Mouse drag motion
	case msg.Action == tea.MouseActionMotion && m.dragging:
		dx := msg.X - m.dragLastX
		dy := msg.Y - m.dragLastY
		if dx != 0 || dy != 0 {
			m.dragLastX = msg.X
			m.dragLastY = msg.Y
			m.dragMoved = true
			m.followMode = false
			m.vp.PanByCells(dx, dy, mapW, mapH)
			m.rebuildFiltered()
		}
		return m, nil

	// Mouse release
	case msg.Action == tea.MouseActionRelease:
		if m.dragging {
			wasMoved := m.dragMoved
			m.dragging = false
			m.dragMoved = false
			if wasMoved {
				if m.vp.SpanLon < 55 {
					return m, m.fetchRegionalCmd()
				}
				return m, nil
			}
			// Simple click without drag -> select nearest aircraft
			if inMap {
				cx := msg.X - 1
				cy := msg.Y - 2
				idx := FindNearestFlightAt(m.filtered, m.vp, mapW, mapH, cx, cy, 3.2)
				if idx >= 0 {
					m.cursor = idx
					m.selectedHex = m.filtered[idx].Hex
					return m, m.maybeFetchSelectedRoute()
				}
			}
		}
		return m, nil
	}

	return m, nil
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (m *AppModel) rebuildFiltered() {
	mapW, mapH, _ := m.mapDimensions()
	minLat, maxLat, minLon, maxLon := m.vp.Bounds(mapW, mapH)
	q := strings.ToUpper(strings.TrimSpace(m.searchQuery))

	res := make([]model.Flight, 0, 512)
	for _, f := range m.flightsByHex {
		if q == "" {
			if f.Lat < minLat || f.Lat > maxLat || f.Lon < minLon || f.Lon > maxLon {
				continue
			}
		}
		if m.onlyMil && !f.IsMilitary {
			continue
		}
		if m.onlyEmerg && !f.IsEmergency() {
			continue
		}
		if m.onlyFav && !m.watchlist.Has(f.Callsign, f.Hex) {
			continue
		}
		switch m.altMode {
		case altHigh:
			if f.AltFeet < 30000 {
				continue
			}
		case altMid:
			if f.AltFeet < 10000 || f.AltFeet >= 30000 {
				continue
			}
		case altLow:
			if f.AltFeet >= 10000 {
				continue
			}
		}
		if q != "" {
			match := strings.Contains(strings.ToUpper(f.Callsign), q) ||
				strings.Contains(strings.ToUpper(f.Hex), q) ||
				strings.Contains(strings.ToUpper(f.Country), q) ||
				strings.Contains(strings.ToUpper(f.Registration), q) ||
				strings.Contains(strings.ToUpper(f.AircraftType), q)
			if !match && f.Callsign != "" {
				if r, ok := m.routes.Get(f.Callsign); ok && !r.NotFound {
					if strings.Contains(strings.ToUpper(r.OriginIATA+r.DestIATA+r.OriginCity+r.DestCity+r.AirlineName), q) {
						match = true
					}
				}
			}
			if !match {
				continue
			}
		}
		res = append(res, f)
	}

	cLat, cLon := m.vp.CenterLat, m.vp.CenterLon
	sort.Slice(res, func(i, j int) bool {
		fi, fj := res[i], res[j]
		if fi.IsEmergency() != fj.IsEmergency() {
			return fi.IsEmergency()
		}
		favi := m.watchlist.Has(fi.Callsign, fi.Hex)
		favj := m.watchlist.Has(fj.Callsign, fj.Hex)
		if favi != favj {
			return favi
		}
		if fi.IsMilitary != fj.IsMilitary {
			return fi.IsMilitary
		}
		di := (fi.Lat-cLat)*(fi.Lat-cLat) + (fi.Lon-cLon)*(fi.Lon-cLon)
		dj := (fj.Lat-cLat)*(fj.Lat-cLat) + (fj.Lon-cLon)*(fj.Lon-cLon)
		if di != dj {
			return di < dj
		}
		return fi.Hex < fj.Hex
	})

	m.filtered = res

	if m.selectedHex != "" {
		found := false
		for i, f := range m.filtered {
			if f.Hex == m.selectedHex {
				m.cursor = i
				found = true
				break
			}
		}
		if !found {
			// Preserve user's selected aircraft as long as it still exists in flightsByHex
			// (e.g. while zooming/panning to view its route). Only fall back if it truly disappeared.
			if _, alive := m.flightsByHex[m.selectedHex]; !alive {
				if m.cursor >= len(m.filtered) {
					m.cursor = len(m.filtered) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
				if len(m.filtered) > 0 {
					m.selectedHex = m.filtered[m.cursor].Hex
				} else {
					m.selectedHex = ""
				}
			} else if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
				if m.cursor < 0 {
					m.cursor = 0
				}
			}
		}
	} else if len(m.filtered) > 0 {
		m.cursor = 0
		m.selectedHex = m.filtered[0].Hex
	}
}

func (m *AppModel) selectedFlight() *model.Flight {
	if m.selectedHex != "" {
		if f, ok := m.flightsByHex[m.selectedHex]; ok {
			return &f
		}
	}
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		return &m.filtered[m.cursor]
	}
	return nil
}

func (m *AppModel) maybeFetchSelectedRoute() tea.Cmd {
	sel := m.selectedFlight()
	if sel == nil || sel.Callsign == "" {
		return nil
	}
	cs := strings.ToUpper(strings.TrimSpace(sel.Callsign))
	if _, ok := m.routes.Get(cs); ok {
		return nil
	}
	if m.routeLoading == cs {
		return nil
	}
	m.routeLoading = cs
	return m.fetchRouteCmd(cs)
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		return m, tea.Quit
	}

	// Modals
	if m.showHelp {
		if key == "?" || key == "esc" || key == "q" || key == "enter" {
			m.showHelp = false
		}
		return m, nil
	}
	if m.showStats {
		if key == "i" || key == "esc" || key == "q" || key == "enter" {
			m.showStats = false
		}
		return m, nil
	}

	// Search input mode
	if m.searching {
		switch key {
		case "esc":
			m.searching = false
			m.searchQuery = ""
			m.rebuildFiltered()
			return m, m.maybeFetchSelectedRoute()
		case "enter":
			m.searching = false
			m.rebuildFiltered()
			if sel := m.selectedFlight(); sel != nil {
				m.vp.CenterLat = sel.Lat
				m.vp.CenterLon = sel.Lon
				if m.vp.SpanLon > 40 {
					m.vp.SpanLon = 24
				}
				m.vp.Name = "ARAMA"
			}
			return m, m.maybeFetchSelectedRoute()
		case "backspace":
			r := []rune(m.searchQuery)
			if len(r) > 0 {
				m.searchQuery = string(r[:len(r)-1])
				m.rebuildFiltered()
			}
			return m, nil
		default:
			if len([]rune(key)) == 1 {
				m.searchQuery += key
				m.rebuildFiltered()
			}
			return m, nil
		}
	}

	mapW, mapH, _ := m.mapDimensions()

	switch key {
	case "q":
		return m, tea.Quit
	case "?":
		m.showHelp = true
		return m, nil
	case "i":
		m.showStats = true
		return m, nil
	case "tab":
		if m.focus == focusList {
			m.focus = focusMap
		} else {
			m.focus = focusList
		}
		return m, nil
	case "/":
		m.searching = true
		return m, nil
	case "esc":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.rebuildFiltered()
		} else if m.followMode {
			m.followMode = false
			m.vp.Name = "ÖZEL"
		}
		return m, nil
	case "f":
		m.fullscreen = !m.fullscreen
		m.rebuildFiltered()
		return m, nil
	case "m":
		m.ascii = !m.ascii
		return m, nil
	case "u":
		m.metric = !m.metric
		return m, nil
	case "b":
		m.labelMode = (m.labelMode + 1) % 3
		m.statusMsg = "Etiket modu: " + m.labelMode.String()
		return m, nil
	case "t":
		m.trailMode = (m.trailMode + 1) % 3
		m.statusMsg = "İz modu: " + m.trailMode.String()
		return m, nil
	case "v":
		m.showVectors = !m.showVectors
		if m.showVectors {
			m.statusMsg = "Hız/yön vektörleri açıldı"
		} else {
			m.statusMsg = "Hız/yön vektörleri kapatıldı"
		}
		return m, nil
	case "g":
		m.showRings = !m.showRings
		if m.showRings {
			m.statusMsg = "Radar mesafe halkaları açıldı"
		} else {
			m.statusMsg = "Radar mesafe halkaları kapatıldı"
		}
		return m, nil
	case "p":
		m.paused = !m.paused
		if m.paused {
			m.statusMsg = "Canlı süzülme animasyonu duraklatıldı"
		} else {
			m.statusMsg = "Canlı süzülme animasyonu devam ediyor"
		}
		return m, nil
	case "T":
		m.themeIdx = (m.themeIdx + 1) % len(Themes)
		m.statusMsg = "Tema: " + m.activeTheme().Name
		return m, nil
	case "L":
		if sel := m.selectedFlight(); sel != nil {
			m.followMode = !m.followMode
			if m.followMode {
				m.syncFollowCamera()
				if m.vp.SpanLon > 24 {
					m.vp.SpanLon = 16
				}
				m.statusMsg = sel.DisplayID() + " canlı takip kilidine alındı"
				m.rebuildFiltered()
				return m, tea.Batch(m.fetchRegionalCmd(), m.maybeFetchSelectedRoute())
			}
			m.vp.Name = "ÖZEL"
			m.statusMsg = "Canlı takip kilidi kapatıldı"
		}
		return m, nil
	case "a":
		m.altMode = (m.altMode + 1) % 4
		m.rebuildFiltered()
		return m, m.maybeFetchSelectedRoute()
	case "M":
		m.onlyMil = !m.onlyMil
		m.rebuildFiltered()
		return m, m.maybeFetchSelectedRoute()
	case "E":
		m.onlyEmerg = !m.onlyEmerg
		m.rebuildFiltered()
		return m, m.maybeFetchSelectedRoute()
	case "S":
		m.onlyFav = !m.onlyFav
		m.rebuildFiltered()
		return m, m.maybeFetchSelectedRoute()
	case "s":
		if sel := m.selectedFlight(); sel != nil {
			added := m.watchlist.Toggle(sel.DisplayID())
			if added {
				m.statusMsg = sel.DisplayID() + " favorilere eklendi"
			} else {
				m.statusMsg = sel.DisplayID() + " favorilerden çıkarıldı"
			}
			m.rebuildFiltered()
		}
		return m, nil
	case "c", "enter":
		if sel := m.selectedFlight(); sel != nil {
			m.vp.CenterLat = sel.Lat
			m.vp.CenterLon = sel.Lon
			if m.vp.SpanLon > 30 {
				m.vp.SpanLon = 18
			}
			m.vp.Name = "TAKİP"
			m.rebuildFiltered()
			return m, tea.Batch(m.fetchRegionalCmd(), m.maybeFetchSelectedRoute())
		}
		return m, nil
	case "r":
		m.loadingGlobal = true
		m.statusMsg = "Yenileniyor..."
		return m, tea.Batch(m.fetchGlobalCmd(), m.fetchMilCmd(), m.fetchRegionalCmd())
	case "+", "=":
		m.vp.Zoom(0.7)
		m.rebuildFiltered()
		if m.vp.SpanLon < 40 {
			return m, m.fetchRegionalCmd()
		}
		return m, nil
	case "-", "_":
		m.vp.Zoom(1.4)
		m.rebuildFiltered()
		return m, nil
	case "1", "2", "3", "4", "5":
		for _, p := range geo.Presets {
			if p.Key == key {
				m.followMode = false
				m.vp = p.Viewport
				m.rebuildFiltered()
				return m, tea.Batch(m.fetchRegionalCmd(), m.maybeFetchSelectedRoute())
			}
		}
		return m, nil
	case "up", "k":
		if m.focus == focusMap || m.fullscreen {
			m.followMode = false
			m.vp.Pan(0, 0.15, mapW, mapH)
			m.rebuildFiltered()
			return m, nil
		}
		if len(m.filtered) > 0 && m.cursor > 0 {
			m.cursor--
			m.selectedHex = m.filtered[m.cursor].Hex
			return m, m.maybeFetchSelectedRoute()
		}
		return m, nil
	case "down", "j":
		if m.focus == focusMap || m.fullscreen {
			m.followMode = false
			m.vp.Pan(0, -0.15, mapW, mapH)
			m.rebuildFiltered()
			return m, nil
		}
		if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.selectedHex = m.filtered[m.cursor].Hex
			return m, m.maybeFetchSelectedRoute()
		}
		return m, nil
	case "left", "h":
		m.followMode = false
		m.vp.Pan(-0.15, 0, mapW, mapH)
		m.rebuildFiltered()
		return m, nil
	case "right", "l":
		m.followMode = false
		m.vp.Pan(0.15, 0, mapW, mapH)
		m.rebuildFiltered()
		return m, nil
	case "pgup":
		if len(m.filtered) > 0 {
			m.cursor -= 10
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.selectedHex = m.filtered[m.cursor].Hex
			return m, m.maybeFetchSelectedRoute()
		}
		return m, nil
	case "pgdown":
		if len(m.filtered) > 0 {
			m.cursor += 10
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			m.selectedHex = m.filtered[m.cursor].Hex
			return m, m.maybeFetchSelectedRoute()
		}
		return m, nil
	}

	return m, nil
}

func (m AppModel) View() string {
	if m.width < 30 || m.height < 12 {
		return "GökRadar için terminal penceresini biraz büyütün (en az 30x12)."
	}

	theme := m.activeTheme()

	if m.showHelp {
		return m.renderHelp(theme)
	}
	if m.showStats {
		return RenderStats(m.width, m.height, m.filtered, m.flightsByHex, m.metric, m.ascii, theme)
	}

	border := lipgloss.RoundedBorder()
	if m.ascii {
		border = lipgloss.ASCIIBorder()
	}

	mapW, mapH, listW := m.mapDimensions()

	// Top header bar
	modeTag := "BRAILLE"
	if m.ascii {
		modeTag = "ASCII"
	}
	unitTag := "FT/KT"
	if m.metric {
		unitTag = "M/KM"
	}
	extraTags := ""
	if m.paused {
		extraTags += " [DURAKLATILDI]"
	}
	if m.loadingGlobal {
		extraTags += " [Yükleniyor...]"
	}

	headerText := fmt.Sprintf(" GÖKRADAR [%s]  |  Görünür: %d / Toplam: %d  |  Askeri: %d  |  %s  |  %s  |  %s  |  %s%s",
		m.vp.Name,
		len(m.filtered),
		len(m.flightsByHex),
		m.milCount,
		m.sourceLabel,
		modeTag,
		unitTag,
		theme.Name,
		extraTags,
	)
	headerBar := theme.HeaderStyle.
		Width(m.width).
		Render(truncate(headerText, m.width))

	// Selected flight & route
	sel := m.selectedFlight()
	var selRoute *model.RouteInfo
	routeLoading := false
	selFav := false
	if sel != nil {
		selFav = m.watchlist.Has(sel.Callsign, sel.Hex)
		if sel.Callsign != "" {
			if r, ok := m.routes.Get(sel.Callsign); ok {
				selRoute = r
			} else if m.routeLoading == strings.ToUpper(strings.TrimSpace(sel.Callsign)) {
				routeLoading = true
			}
		}
	}

	// Map panel
	mapContent := RenderMap(
		mapW, mapH,
		m.vp,
		m.world,
		m.filtered,
		m.selectedHex,
		sel,
		selRoute,
		m.trails,
		m.watchlist.Has,
		MapRenderOptions{
			ASCII:       m.ascii,
			Metric:      m.metric,
			LabelMode:   m.labelMode,
			TrailMode:   m.trailMode,
			ShowRings:   m.showRings,
			ShowVectors: m.showVectors,
			Theme:       theme,
		},
	)
	mapBorderColor := theme.IdleBorder
	if m.focus == focusMap || m.fullscreen {
		mapBorderColor = theme.FocusBorder
	}
	mapBox := lipgloss.NewStyle().
		Border(border).
		BorderForeground(mapBorderColor).
		Render(mapContent)

	var middle string
	if listW > 0 {
		var tags []string
		if m.altMode != altAll {
			tags = append(tags, m.altMode.Label())
		}
		if m.onlyMil {
			tags = append(tags, "MIL")
		}
		if m.onlyEmerg {
			tags = append(tags, "ACİL")
		}
		if m.onlyFav {
			tags = append(tags, "FAV")
		}
		listContent := RenderList(
			listW, mapH,
			m.filtered,
			m.cursor,
			m.routes,
			m.watchlist.Has,
			m.metric,
			m.ascii,
			m.searching,
			m.searchQuery,
			strings.Join(tags, ","),
			theme,
		)
		listBorderColor := theme.IdleBorder
		if m.focus == focusList {
			listBorderColor = theme.FocusBorder
		}
		listBox := lipgloss.NewStyle().
			Border(border).
			BorderForeground(listBorderColor).
			Render(listContent)

		middle = lipgloss.JoinHorizontal(lipgloss.Top, mapBox, listBox)
	} else {
		middle = mapBox
	}

	// Bottom detail panel
	detailW := m.width - 2
	detailContent := RenderDetail(detailW, sel, selRoute, routeLoading, selFav, m.metric, m.ascii, m.statusMsg, theme)
	detailBox := lipgloss.NewStyle().
		Border(border).
		BorderForeground(theme.IdleBorder).
		Width(detailW).
		Render(detailContent)

	return lipgloss.JoinVertical(lipgloss.Left, headerBar, middle, detailBox)
}

func (m AppModel) renderHelp(theme Theme) string {
	border := lipgloss.RoundedBorder()
	if m.ascii {
		border = lipgloss.ASCIIBorder()
	}
	helpText := strings.Join([]string{
		theme.DetailTitle.Render("GÖKRADAR — FARE & KLAVYE KISAYOLLARI"),
		"",
		theme.ListHeader.Render("  FARE KONTROLLERİ"),
		"  Sol Tık + Sürükle   : Haritayı serbestçe kaydır",
		"  Fare Tekerleği      : Haritada imleç odaklı yakınlaş/uzaklaş | Listede kaydır",
		"  Sol Tık             : Haritada en yakın uçağı veya listeden satırı seç",
		"  Çift Sol Tık        : Uçağa canlı takip kilidi aç | Boş alana yakınlaş",
		"  Sağ Tık             : Uçağa takip kilidi aç/kapat | Boş noktayı ortala",
		"",
		theme.ListHeader.Render("  KLAVYE KISAYOLLARI"),
		"  1 - 5               : Hızlı Bölge (Dünya, Türkiye, Avrupa, Orta Doğu, ABD)",
		"  Tab                 : Odak değiştir (Liste ↔ Harita)",
		"  ↑↓←→ veya hjkl      : Listede uçak seç / Haritada gezin",
		"  + / -               : Yakınlaştır / Uzaklaştır",
		"  Enter veya c        : Seçili uçağı haritada ortala",
		"  L                   : Canlı Takip Kilidi (Follow Mode) aç / kapat",
		"  b / v / t / g       : Etiket modu / Hız vektörleri / İz modu / Radar halkaları",
		"  T / i / p           : Tema değiştir / Canlı istatistikler / Animasyonu duraklat",
		"  f / m / u / r       : Tam ekran / ASCII modu / Birimler (ft↔m) / Yenile",
		"  /  |  a  |  M E s S : Ara | İrtifa filtresi | Askeri / Acil / Favori",
		"  q veya Ctrl+C       : Çıkış",
		"",
		"  Kapatmak için ?, Esc veya Enter tuşuna basın.",
	}, "\n")

	box := lipgloss.NewStyle().
		Border(border).
		BorderForeground(theme.FocusBorder).
		Padding(1, 3).
		Render(helpText)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
