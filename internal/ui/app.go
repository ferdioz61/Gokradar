package ui

import (
	"context"
	"fmt"
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

type globalDataMsg struct {
	flights []model.Flight
	err     error
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
	ascii      bool
	metric     bool

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
	loadingGlobal bool
	routeLoading  string
	lastUpdate    time.Time
	statusMsg     string
	globalCount   int
	milCount      int
	sourceLabel   string
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
		width:        120,
		height:       34,
		world:        world,
		client:       &http.Client{Timeout: 18 * time.Second},
		routes:       data.NewRouteCache(),
		watchlist:    config.LoadWatchlist(),
		vp:           vp,
		focus:        focusList,
		ascii:        opts.ASCII,
		metric:       opts.Metric,
		flightsByHex: make(map[string]model.Flight, 12000),
		trails:       make(map[string][]model.Point, 4096),
		loadingGlobal: true,
		statusMsg:    "Dünya geneli uçuş verileri yükleniyor...",
		sourceLabel:  "Yükleniyor",
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchGlobalCmd(),
		m.fetchMilCmd(),
		m.fetchRegionalCmd(),
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(15*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m AppModel) fetchGlobalCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
		defer cancel()
		fl, err := data.FetchOpenSkyGlobal(ctx, client)
		return globalDataMsg{flights: fl, err: err}
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
	lat, lon := m.vp.CenterLat, m.vp.CenterLon
	// If viewing whole world, enrich Turkey region by default; otherwise enrich viewport center
	if m.vp.SpanLon > 120 {
		lat, lon = 39.0, 35.0
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		fl, err := data.FetchADSBLolPoint(ctx, client, lat, lon, 250)
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

	case globalDataMsg:
		m.loadingGlobal = false
		if msg.err != nil {
			m.statusMsg = "OpenSky uyarısı: " + msg.err.Error() + " (bölgesel/askeri akış aktif)"
		} else {
			m.mergeFlights(msg.flights, true)
			m.globalCount = len(msg.flights)
			m.lastUpdate = time.Now()
			m.sourceLabel = "OpenSky+ADSB"
			m.statusMsg = fmt.Sprintf("Güncellendi (%s) — %d aktif uçak", m.lastUpdate.Format("15:04:05"), len(m.flightsByHex))
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
			m.rebuildFiltered()
			return m, m.maybeFetchSelectedRoute()
		}
		return m, nil

	case milDataMsg:
		if msg.err == nil {
			m.milCount = len(msg.flights)
			m.mergeFlights(msg.flights, false)
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

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
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
			// Preserve richer fields when merging OpenSky with adsb.lol
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

		// Append to trail if moved noticeably
		tr := m.trails[f.Hex]
		pt := model.Point{Lat: f.Lat, Lon: f.Lon}
		if len(tr) == 0 {
			m.trails[f.Hex] = []model.Point{pt}
		} else {
			last := tr[len(tr)-1]
			if (last.Lat-pt.Lat)*(last.Lat-pt.Lat)+(last.Lon-pt.Lon)*(last.Lon-pt.Lon) > 0.0001 {
				tr = append(tr, pt)
				if len(tr) > 30 {
					tr = tr[len(tr)-30:]
				}
				m.trails[f.Hex] = tr
			}
		}
	}
}

func (m *AppModel) mapDimensions() (mapW, mapH, listW int) {
	// Reserve 5 lines for detail panel (3 lines + top/bottom border) + 1 top status bar
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

func (m *AppModel) rebuildFiltered() {
	mapW, mapH, _ := m.mapDimensions()
	minLat, maxLat, minLon, maxLon := m.vp.Bounds(mapW, mapH)
	q := strings.ToUpper(strings.TrimSpace(m.searchQuery))

	res := make([]model.Flight, 0, 512)
	for _, f := range m.flightsByHex {
		// Viewport filter (unless user is actively searching globally)
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
		// Prefer flights closer to viewport center
		di := (fi.Lat-cLat)*(fi.Lat-cLat) + (fi.Lon-cLon)*(fi.Lon-cLon)
		dj := (fj.Lat-cLat)*(fj.Lat-cLat) + (fj.Lon-cLon)*(fj.Lon-cLon)
		if di != dj {
			return di < dj
		}
		return fi.Hex < fj.Hex
	})

	m.filtered = res

	// Keep cursor synced to previously selected aircraft if still present
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
		}
	} else if len(m.filtered) > 0 {
		m.cursor = 0
		m.selectedHex = m.filtered[0].Hex
	}
}

func (m *AppModel) selectedFlight() *model.Flight {
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

	// Help overlay toggle
	if m.showHelp {
		if key == "?" || key == "esc" || key == "q" || key == "enter" {
			m.showHelp = false
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
				m.vp = p.Viewport
				m.rebuildFiltered()
				return m, tea.Batch(m.fetchRegionalCmd(), m.maybeFetchSelectedRoute())
			}
		}
		return m, nil
	case "up", "k":
		if m.focus == focusMap || m.fullscreen {
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
		m.vp.Pan(-0.15, 0, mapW, mapH)
		m.rebuildFiltered()
		return m, nil
	case "right", "l":
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

	if m.showHelp {
		return m.renderHelp()
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
	focusTag := "ODAK:LİSTE"
	if m.focus == focusMap || m.fullscreen {
		focusTag = "ODAK:HARİTA"
	}
	loadingTag := ""
	if m.loadingGlobal {
		loadingTag = " [Yükleniyor...]"
	}

	headerText := fmt.Sprintf(" GÖKRADAR [%s]  |  Görünür: %d / Toplam: %d  |  Askeri: %d  |  %s  |  %s  |  %s  |  %s%s",
		m.vp.Name,
		len(m.filtered),
		len(m.flightsByHex),
		m.milCount,
		m.sourceLabel,
		modeTag,
		unitTag,
		focusTag,
		loadingTag,
	)
	headerBar := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("24")).
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
		selRoute,
		m.trails,
		m.watchlist.Has,
		m.ascii,
	)
	mapBorderColor := lipgloss.Color("240")
	if m.focus == focusMap || m.fullscreen {
		mapBorderColor = lipgloss.Color("39")
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
		)
		listBorderColor := lipgloss.Color("240")
		if m.focus == focusList {
			listBorderColor = lipgloss.Color("39")
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
	detailContent := RenderDetail(detailW, sel, selRoute, routeLoading, selFav, m.metric, m.statusMsg)
	detailBox := lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color("240")).
		Width(detailW).
		Render(detailContent)

	return lipgloss.JoinVertical(lipgloss.Left, headerBar, middle, detailBox)
}

func (m AppModel) renderHelp() string {
	border := lipgloss.RoundedBorder()
	if m.ascii {
		border = lipgloss.ASCIIBorder()
	}
	helpText := strings.Join([]string{
		"GÖKRADAR — KLAVYE KISAYOLLARI",
		"",
		"  1 / 2 / 3 / 4 / 5   : Hızlı Bölge (Dünya, Türkiye, Avrupa, Orta Doğu, ABD)",
		"  Tab                 : Odak değiştir (Liste ↔ Harita)",
		"  ↑ / ↓ veya j / k    : Listede uçak seç / Haritada yukarı-aşağı kaydır",
		"  ← / → veya h / l    : Haritayı sola-sağa kaydır",
		"  + / -               : Haritayı yakınlaştır / uzaklaştır",
		"  Enter veya c        : Seçili uçağı haritanın ortasına al ve yakınlaş",
		"  f                   : Tam ekran harita modunu aç / kapat",
		"  /                   : Ara (çağrı kodu, ülke, uçak tipi, havalimanı, havayolu)",
		"  a                   : İrtifa filtresi (Tümü → >30k → 10k-30k → <10k)",
		"  M                   : Sadece askeri uçuşları göster",
		"  E                   : Sadece acil durum (7700 / 7600 / 7500) uçuşları",
		"  s / S               : Seçili uçağı favorile / Sadece favorileri filtrele",
		"  m                   : Braille ↔ ASCII harita modu (eski PowerShell için)",
		"  u                   : Havacılık (ft/kt) ↔ Metrik (m/km/s) birimleri",
		"  r                   : Verileri hemen yenile",
		"  q veya Ctrl+C       : Çıkış",
		"",
		"  Renkler: Yeşil (<15k ft) | Mavi (15k-30k) | Mor (>30k) | Turuncu (Askeri) | Kırmızı (Acil) | Altın (Favori)",
		"",
		"  Kapatmak için ?, Esc veya Enter tuşuna basın.",
	}, "\n")

	box := lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 3).
		Render(helpText)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
