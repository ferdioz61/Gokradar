package ui

import (
	"fmt"
	"math"

	"gokradar/internal/data"
	"gokradar/internal/geo"
	"gokradar/internal/model"
)

// LabelMode controls how aircraft callsign/ATC blocks are drawn on the map.
type LabelMode int

const (
	LabelSmart    LabelMode = iota // Selected, emergency, fav/mil + nearby when zoomed in
	LabelDetailed                  // 2-line ATC data block for selected + nearby labels
	LabelMinimal                   // Only selected and emergency
)

func (l LabelMode) String() string {
	switch l {
	case LabelDetailed:
		return "ATC"
	case LabelMinimal:
		return "MİN"
	default:
		return "AKILLI"
	}
}

// TrailMode controls which aircraft historical trails are rendered.
type TrailMode int

const (
	TrailSelected TrailMode = iota // Selected aircraft only
	TrailAll                       // Short trails for all visible aircraft + full selected
	TrailOff                       // Trails hidden
)

func (t TrailMode) String() string {
	switch t {
	case TrailAll:
		return "TÜMÜ"
	case TrailOff:
		return "KAPALI"
	default:
		return "SEÇİLİ"
	}
}

// MapRenderOptions configures visual layers on the radar map.
type MapRenderOptions struct {
	ASCII       bool
	Metric      bool
	LabelMode   LabelMode
	TrailMode   TrailMode
	ShowRings   bool
	ShowVectors bool
	Theme       Theme
}

// RenderMap draws the geographic map, optional radar range rings, airports, velocity vectors,
// altitude-colored trails, two-stage routes, aircraft markers, and collision-free ATC labels.
func RenderMap(
	width, height int,
	vp geo.Viewport,
	world *data.WorldData,
	flights []model.Flight,
	selectedHex string,
	selectedFlight *model.Flight,
	selectedRoute *model.RouteInfo,
	trails map[string][]model.Point,
	isFav func(callsign, hex string) bool,
	opts MapRenderOptions,
) string {
	if width < 10 || height < 5 {
		return "Terminal çok küçük"
	}

	ascii := opts.ASCII
	canvas := geo.NewCanvas(width, height, ascii)
	pw, ph := canvas.PixelSize()
	fw, fh := float64(pw-1), float64(ph-1)

	minLat, maxLat, minLon, maxLon := vp.Bounds(width, height)

	drawPolyline := func(pts [][2]float64, layer geo.Layer, dashed int) {
		for i := 0; i < len(pts)-1; i++ {
			lon1, lat1 := pts[i][0], pts[i][1]
			lon2, lat2 := pts[i+1][0], pts[i+1][1]
			if math.Abs(lon1-lon2) > 180 {
				continue
			}
			if (lon1 < minLon && lon2 < minLon) || (lon1 > maxLon && lon2 > maxLon) ||
				(lat1 < minLat && lat2 < minLat) || (lat1 > maxLat && lat2 > maxLat) {
				continue
			}
			x0, y0, _ := vp.Project(lat1, lon1, width, height, fw, fh)
			x1, y1, _ := vp.Project(lat2, lon2, width, height, fw, fh)
			canvas.DrawLine(int(math.Round(x0)), int(math.Round(y0)), int(math.Round(x1)), int(math.Round(y1)), layer, dashed)
		}
	}

	// 1. Optional ATC Radar Range Rings centered on viewport
	if opts.ShowRings {
		drawRangeRings(canvas, vp, width, height, fw, fh, drawPolyline, opts.Metric)
	}

	// 2. Country borders & Coastlines
	if world != nil {
		for _, line := range world.Borders {
			drawPolyline(line, geo.LayerBorder, 0)
		}
		for _, line := range world.Coastlines {
			drawPolyline(line, geo.LayerCoast, 0)
		}
	}

	// Locate selected flight (prefer explicit selectedFlight so route stays visible even when panned off-screen)
	selFlight := selectedFlight
	if selFlight == nil {
		for i := range flights {
			if flights[i].Hex == selectedHex {
				selFlight = &flights[i]
				break
			}
		}
	}

	// 3. Two-stage Selected Route (Origin -> Aircraft solid, Aircraft -> Destination dashed)
	if selFlight != nil && selectedRoute != nil && selectedRoute.HasCoords() {
		drawGreatCircle := func(lat1, lon1, lat2, lon2 float64, layer geo.Layer, dashed int) {
			gc := geo.GreatCirclePoints(lat1, lon1, lat2, lon2, 56)
			poly := make([][2]float64, len(gc))
			for i, p := range gc {
				poly[i] = [2]float64{p[1], p[0]}
			}
			drawPolyline(poly, layer, dashed)
		}
		drawGreatCircle(selectedRoute.OriginLat, selectedRoute.OriginLon, selFlight.Lat, selFlight.Lon, geo.LayerRouteDone, 0)
		drawGreatCircle(selFlight.Lat, selFlight.Lon, selectedRoute.DestLat, selectedRoute.DestLon, geo.LayerRouteRem, 2)

		origSym, destSym := "◉", "◎"
		if ascii {
			origSym, destSym = "O", "D"
		}
		if ox, oy, ok := vp.Project(selectedRoute.OriginLat, selectedRoute.OriginLon, width, height, float64(width-1), float64(height-1)); ok {
			lbl := selectedRoute.OriginIATA
			if lbl == "" {
				lbl = selectedRoute.OriginICAO
			}
			cx, cy := int(math.Round(ox)), int(math.Round(oy))
			canvas.SetOverlay(cx, cy, origSym, geo.LayerRouteEndpoint)
			canvas.DrawLabelSmart(cx, cy, lbl, geo.LayerRouteEndpoint)
		}
		if dx, dy, ok := vp.Project(selectedRoute.DestLat, selectedRoute.DestLon, width, height, float64(width-1), float64(height-1)); ok {
			lbl := selectedRoute.DestIATA
			if lbl == "" {
				lbl = selectedRoute.DestICAO
			}
			cx, cy := int(math.Round(dx)), int(math.Round(dy))
			canvas.SetOverlay(cx, cy, destSym, geo.LayerRouteEndpoint)
			canvas.DrawLabelSmart(cx, cy, lbl, geo.LayerRouteEndpoint)
		}
	}

	// 4. Altitude-colored Historical Trails
	drawTrail := func(pts []model.Point, maxPts int) {
		if len(pts) < 2 {
			return
		}
		start := 0
		if maxPts > 0 && len(pts) > maxPts {
			start = len(pts) - maxPts
		}
		for i := start; i < len(pts)-1; i++ {
			p1, p2 := pts[i], pts[i+1]
			if math.Abs(p1.Lon-p2.Lon) > 180 {
				continue
			}
			if (p1.Lon < minLon && p2.Lon < minLon) || (p1.Lon > maxLon && p2.Lon > maxLon) ||
				(p1.Lat < minLat && p2.Lat < minLat) || (p1.Lat > maxLat && p2.Lat > maxLat) {
				continue
			}
			x0, y0, _ := vp.Project(p1.Lat, p1.Lon, width, height, fw, fh)
			x1, y1, _ := vp.Project(p2.Lat, p2.Lon, width, height, fw, fh)
			canvas.DrawLine(int(math.Round(x0)), int(math.Round(y0)), int(math.Round(x1)), int(math.Round(y1)), trailLayerForAlt(p2.AltFeet), 0)
		}
	}

	if opts.TrailMode == TrailAll {
		for i := range flights {
			f := flights[i]
			if f.Hex == selectedHex {
				continue
			}
			if pts := trails[f.Hex]; len(pts) > 1 {
				drawTrail(pts, 12)
			}
		}
	}
	if opts.TrailMode != TrailOff && selFlight != nil {
		if pts := trails[selFlight.Hex]; len(pts) > 1 {
			drawTrail(pts, 0)
		}
	}

	// 5. Velocity / Heading Leader Vectors (2-minute projected path in Braille subpixels)
	if opts.ShowVectors {
		for i := range flights {
			f := flights[i]
			if f.OnGround || f.SpeedKt < 40 {
				continue
			}
			isSel := f.Hex == selectedHex
			if !isSel && vp.SpanLon >= 45 {
				continue
			}
			x0, y0, vis := vp.Project(f.Lat, f.Lon, width, height, fw, fh)
			if !vis {
				continue
			}
			// 2.5 min lookahead for selected, 2 min for others
			lookSec := 120.0
			if isSel {
				lookSec = 180.0
			}
			pLat, pLon := geo.ProjectPoint(f.Lat, f.Lon, f.SpeedKt, f.TrackDeg, lookSec)
			x1, y1, _ := vp.Project(pLat, pLon, width, height, fw, fh)
			// Clamp vector length in pixels so it looks crisp at every zoom level
			dx, dy := x1-x0, y1-y0
			dist := math.Hypot(dx, dy)
			maxLen := 9.0
			if isSel {
				maxLen = 14.0
			}
			if dist > maxLen && dist > 0 {
				scale := maxLen / dist
				x1 = x0 + dx*scale
				y1 = y0 + dy*scale
			}
			if dist >= 1.2 {
				lyr := geo.LayerVector
				if isSel {
					lyr = geo.LayerRouteDone
				}
				canvas.DrawLine(int(math.Round(x0)), int(math.Round(y0)), int(math.Round(x1)), int(math.Round(y1)), lyr, 0)
			}
		}
	}

	// 6. Airport markers
	if world != nil {
		maxPriority := 1
		if vp.SpanLon < 55 {
			maxPriority = 2
		}
		apChar := "◈"
		if ascii {
			apChar = "+"
		}
		showLabels := vp.SpanLon < 95
		for _, ap := range world.Airports {
			if ap.Priority > maxPriority {
				continue
			}
			x, y, vis := vp.Project(ap.Lat, ap.Lon, width, height, float64(width-1), float64(height-1))
			if !vis {
				continue
			}
			cx, cy := int(math.Round(x)), int(math.Round(y))
			canvas.SetOverlay(cx, cy, apChar, geo.LayerAirport)
			if showLabels {
				canvas.DrawLabel(cx+1, cy, ap.IATA, geo.LayerAirport)
			}
		}
	}

	// 7. Aircraft markers (pass 1: place all symbols first so labels never cover aircraft!)
	type placedPlane struct {
		f      model.Flight
		cx, cy int
		lyr    geo.Layer
		isSel  bool
		isFav  bool
	}
	placed := make([]placedPlane, 0, len(flights))
	var selPlaced *placedPlane

	for i := range flights {
		f := flights[i]
		x, y, vis := vp.Project(f.Lat, f.Lon, width, height, float64(width-1), float64(height-1))
		if !vis {
			continue
		}
		cx, cy := int(math.Round(x)), int(math.Round(y))
		sym := model.HeadingSymbol(f.TrackDeg, ascii, f.OnGround)
		fav := isFav(f.Callsign, f.Hex)
		isSel := f.Hex == selectedHex

		var lyr geo.Layer
		switch {
		case isSel:
			lyr = geo.LayerPlaneSel
		case f.IsEmergency():
			lyr = geo.LayerPlaneEmerg
			sym = "!"
		case f.IsMilitary:
			lyr = geo.LayerPlaneMil
		case fav:
			lyr = geo.LayerPlaneFav
		case f.AltFeet < 15000:
			lyr = geo.LayerPlaneLow
		case f.AltFeet < 30000:
			lyr = geo.LayerPlaneMid
		default:
			lyr = geo.LayerPlaneHigh
		}

		canvas.SetOverlay(cx, cy, sym, lyr)
		p := placedPlane{f: f, cx: cx, cy: cy, lyr: lyr, isSel: isSel, isFav: fav}
		placed = append(placed, p)
		if isSel {
			cp := p
			selPlaced = &cp
		}
	}

	// 8. Aircraft Labels (pass 2: highest priority first using collision-avoidance)
	// 8a. Selected aircraft reticle & label
	if selPlaced != nil {
		p := selPlaced
		if !ascii {
			canvas.SetOverlay(p.cx-1, p.cy, "[", geo.LayerRouteEndpoint)
			canvas.SetOverlay(p.cx+1, p.cy, "]", geo.LayerRouteEndpoint)
		}
		altTag := model.FormatATCAlt(p.f.AltFeet, p.f.OnGround, opts.Metric)
		line1 := p.f.DisplayID()
		if altTag != "" && opts.LabelMode != LabelDetailed {
			line1 += " " + altTag
		}
		if !canvas.DrawLabelSmart(p.cx+1, p.cy, line1, geo.LayerPlaneSel) {
			canvas.DrawLabelForce(p.cx+2, p.cy, line1, geo.LayerPlaneSel)
		}
		if opts.LabelMode == LabelDetailed {
			spdStr := model.FormatSpeed(p.f.SpeedKt, opts.Metric)
			line2 := fmt.Sprintf("%s %s", altTag, spdStr)
			canvas.DrawLabelSmart(p.cx+1, p.cy+1, line2, geo.LayerRouteDone)
		}
	}

	// 8b. Emergency, favorite, military, and nearby zoomed-in aircraft
	if opts.LabelMode != LabelMinimal {
		for _, p := range placed {
			if p.isSel {
				continue
			}
			if p.f.IsEmergency() {
				canvas.DrawLabelSmart(p.cx, p.cy, p.f.DisplayID()+"!", geo.LayerPlaneEmerg)
			}
		}
		for _, p := range placed {
			if p.isSel || p.f.IsEmergency() {
				continue
			}
			if vp.SpanLon < 40 && (p.isFav || p.f.IsMilitary) {
				lbl := p.f.DisplayID()
				if vp.SpanLon < 22 {
					if at := model.FormatATCAlt(p.f.AltFeet, p.f.OnGround, opts.Metric); at != "" {
						lbl += " " + at
					}
				}
				canvas.DrawLabelSmart(p.cx, p.cy, lbl, p.lyr)
			}
		}
		// Nearby regular aircraft when zoomed in closely
		if vp.SpanLon < 22 {
			for _, p := range placed {
				if p.isSel || p.f.IsEmergency() || p.isFav || p.f.IsMilitary {
					continue
				}
				lbl := p.f.DisplayID()
				if opts.LabelMode == LabelDetailed || vp.SpanLon < 12 {
					if at := model.FormatATCAlt(p.f.AltFeet, p.f.OnGround, opts.Metric); at != "" {
						lbl += " " + at
					}
				}
				canvas.DrawLabelSmart(p.cx, p.cy, lbl, p.lyr)
			}
		}
	} else {
		// Minimal mode: still show emergency labels
		for _, p := range placed {
			if !p.isSel && p.f.IsEmergency() {
				canvas.DrawLabelSmart(p.cx, p.cy, p.f.DisplayID()+"!", geo.LayerPlaneEmerg)
			}
		}
	}

	// 9. Optional Scale Bar in bottom-right corner (shown when Rings enabled or zoomed in)
	if opts.ShowRings || vp.SpanLon < 60 {
		drawScaleBar(canvas, vp, width, height, opts.Metric, ascii)
	}

	return canvas.RenderWithStyles(opts.Theme.CanvasStyles)
}

func trailLayerForAlt(feet int) geo.Layer {
	switch {
	case feet < 15000:
		return geo.LayerTrailLow
	case feet < 30000:
		return geo.LayerTrailMid
	default:
		return geo.LayerTrailHigh
	}
}

func drawRangeRings(
	canvas *geo.Canvas,
	vp geo.Viewport,
	width, height int,
	fw, fh float64,
	drawPolyline func([][2]float64, geo.Layer, int),
	metric bool,
) {
	// Pick a clean ring step based on visible radius
	radNM := float64(vp.RadiusNM(width, height))
	stepNM := 50.0
	switch {
	case radNM > 2500:
		stepNM = 1000
	case radNM > 1200:
		stepNM = 500
	case radNM > 500:
		stepNM = 200
	case radNM > 220:
		stepNM = 100
	case radNM > 90:
		stepNM = 50
	default:
		stepNM = 25
	}

	for ring := 1; ring <= 3; ring++ {
		r := stepNM * float64(ring)
		pts := geo.CirclePoints(vp.CenterLat, vp.CenterLon, r, 72)
		drawPolyline(pts, geo.LayerRangeRing, 2)

		// Label at north edge of ring
		nLat, nLon := geo.ProjectPoint(vp.CenterLat, vp.CenterLon, r*3600.0, 0, 1.0)
		if x, y, vis := vp.Project(nLat, nLon, width, height, float64(width-1), float64(height-1)); vis {
			var lbl string
			if metric {
				lbl = fmt.Sprintf("%dkm", int(math.Round(r*1.852)))
			} else {
				lbl = fmt.Sprintf("%dNM", int(r))
			}
			canvas.DrawLabel(int(math.Round(x))-len(lbl)/2, int(math.Round(y)), lbl, geo.LayerRangeRing)
		}
	}
}

func drawScaleBar(canvas *geo.Canvas, vp geo.Viewport, width, height int, metric, ascii bool) {
	if width < 26 || height < 6 {
		return
	}
	// Compute ground distance corresponding to 12 terminal columns at viewport center
	const barCells = 12
	_, lonA := vp.Unproject(float64(width/2), float64(height/2), width, height)
	_, lonB := vp.Unproject(float64(width/2+barCells), float64(height/2), width, height)
	distNM := geo.HaversineNM(vp.CenterLat, lonA, vp.CenterLat, lonB)
	if distNM <= 0 {
		return
	}
	var label string
	if metric {
		km := distNM * 1.852
		if km >= 100 {
			label = fmt.Sprintf("%d km", int(math.Round(km/10)*10))
		} else {
			label = fmt.Sprintf("%d km", int(math.Round(km)))
		}
	} else {
		if distNM >= 100 {
			label = fmt.Sprintf("%d NM", int(math.Round(distNM/10)*10))
		} else {
			label = fmt.Sprintf("%d NM", int(math.Round(distNM)))
		}
	}
	bar := fmt.Sprintf("└─ %s ─┘", label)
	if ascii {
		bar = fmt.Sprintf("+-- %s --+", label)
	}
	startX := width - len([]rune(bar)) - 1
	if startX >= 1 {
		canvas.DrawLabelForce(startX, height-1, bar, geo.LayerScaleBar)
	}
}
