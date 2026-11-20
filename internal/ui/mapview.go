package ui

import (
	"math"

	"gokradar/internal/data"
	"gokradar/internal/geo"
	"gokradar/internal/model"
)

// RenderMap draws the geographic map, airports, aircraft, trails, and selected route.
func RenderMap(
	width, height int,
	vp geo.Viewport,
	world *data.WorldData,
	flights []model.Flight,
	selectedHex string,
	selectedRoute *model.RouteInfo,
	trails map[string][]model.Point,
	isFav func(callsign, hex string) bool,
	ascii bool,
) string {
	if width < 10 || height < 5 {
		return "Terminal çok küçük"
	}

	canvas := geo.NewCanvas(width, height, ascii)
	pw, ph := canvas.PixelSize()
	fw, fh := float64(pw-1), float64(ph-1)

	minLat, maxLat, minLon, maxLon := vp.Bounds(width, height)

	// Helper to draw a polyline clipped to viewport bounds.
	drawPolyline := func(pts [][2]float64, layer geo.Layer, dashed int) {
		for i := 0; i < len(pts)-1; i++ {
			lon1, lat1 := pts[i][0], pts[i][1]
			lon2, lat2 := pts[i+1][0], pts[i+1][1]
			// Skip antimeridian wrap jumps
			if math.Abs(lon1-lon2) > 180 {
				continue
			}
			// Quick bounding-box reject
			if (lon1 < minLon && lon2 < minLon) || (lon1 > maxLon && lon2 > maxLon) ||
				(lat1 < minLat && lat2 < minLat) || (lat1 > maxLat && lat2 > maxLat) {
				continue
			}
			x0, y0, _ := vp.Project(lat1, lon1, width, height, fw, fh)
			x1, y1, _ := vp.Project(lat2, lon2, width, height, fw, fh)
			canvas.DrawLine(int(math.Round(x0)), int(math.Round(y0)), int(math.Round(x1)), int(math.Round(y1)), layer, dashed)
		}
	}

	// 1. Country borders & Coastlines
	if world != nil {
		for _, line := range world.Borders {
			drawPolyline(line, geo.LayerBorder, 0)
		}
		for _, line := range world.Coastlines {
			drawPolyline(line, geo.LayerCoast, 0)
		}
	}

	// 2. Selected aircraft route line (Origin -> Aircraft -> Destination)
	var selFlight *model.Flight
	for i := range flights {
		if flights[i].Hex == selectedHex {
			selFlight = &flights[i]
			break
		}
	}

	if selFlight != nil && selectedRoute != nil && selectedRoute.HasCoords() {
		drawGreatCircle := func(lat1, lon1, lat2, lon2 float64) {
			gc := geo.GreatCirclePoints(lat1, lon1, lat2, lon2, 48)
			poly := make([][2]float64, len(gc))
			for i, p := range gc {
				poly[i] = [2]float64{p[1], p[0]} // [lon, lat]
			}
			drawPolyline(poly, geo.LayerRoute, 2)
		}
		drawGreatCircle(selectedRoute.OriginLat, selectedRoute.OriginLon, selFlight.Lat, selFlight.Lon)
		drawGreatCircle(selFlight.Lat, selFlight.Lon, selectedRoute.DestLat, selectedRoute.DestLon)

		// Mark Origin and Destination airports on map
		if ox, oy, ok := vp.Project(selectedRoute.OriginLat, selectedRoute.OriginLon, width, height, float64(width-1), float64(height-1)); ok {
			lbl := selectedRoute.OriginIATA
			if lbl == "" {
				lbl = selectedRoute.OriginICAO
			}
			canvas.SetOverlay(int(math.Round(ox)), int(math.Round(oy)), "O", geo.LayerRoute)
			canvas.DrawLabel(int(math.Round(ox))+1, int(math.Round(oy)), lbl, geo.LayerRoute)
		}
		if dx, dy, ok := vp.Project(selectedRoute.DestLat, selectedRoute.DestLon, width, height, float64(width-1), float64(height-1)); ok {
			lbl := selectedRoute.DestIATA
			if lbl == "" {
				lbl = selectedRoute.DestICAO
			}
			canvas.SetOverlay(int(math.Round(dx)), int(math.Round(dy)), "D", geo.LayerRoute)
			canvas.DrawLabel(int(math.Round(dx))+1, int(math.Round(dy)), lbl, geo.LayerRoute)
		}
	}

	// 3. Selected aircraft historical trail
	if selFlight != nil {
		if pts := trails[selFlight.Hex]; len(pts) > 1 {
			poly := make([][2]float64, len(pts))
			for i, p := range pts {
				poly[i] = [2]float64{p.Lon, p.Lat}
			}
			drawPolyline(poly, geo.LayerTrail, 0)
		}
	}

	// 4. Airport markers
	if world != nil {
		maxPriority := 1
		if vp.SpanLon < 55 {
			maxPriority = 2
		}
		apChar := "◈"
		if ascii {
			apChar = "+"
		}
		showLabels := vp.SpanLon < 100
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

	// 5. Aircraft markers
	for i := range flights {
		f := flights[i]
		x, y, vis := vp.Project(f.Lat, f.Lon, width, height, float64(width-1), float64(height-1))
		if !vis {
			continue
		}
		cx, cy := int(math.Round(x)), int(math.Round(y))
		sym := model.HeadingArrow(f.TrackDeg, ascii)

		var lyr geo.Layer
		switch {
		case f.Hex == selectedHex:
			lyr = geo.LayerPlaneSel
		case f.IsEmergency():
			lyr = geo.LayerPlaneEmerg
			sym = "!"
		case f.IsMilitary:
			lyr = geo.LayerPlaneMil
		case isFav(f.Callsign, f.Hex):
			lyr = geo.LayerPlaneFav
		case f.AltFeet < 15000:
			lyr = geo.LayerPlaneLow
		case f.AltFeet < 30000:
			lyr = geo.LayerPlaneMid
		default:
			lyr = geo.LayerPlaneHigh
		}

		canvas.SetOverlay(cx, cy, sym, lyr)

		// Show callsign label for selected, emergency, or favorite planes when zoomed in
		if f.Hex == selectedHex {
			canvas.DrawLabel(cx+1, cy, " "+f.DisplayID(), geo.LayerPlaneSel)
		} else if f.IsEmergency() {
			canvas.DrawLabel(cx+1, cy, " "+f.DisplayID()+"!", geo.LayerPlaneEmerg)
		} else if vp.SpanLon < 35 && (isFav(f.Callsign, f.Hex) || f.IsMilitary) {
			canvas.DrawLabel(cx+1, cy, " "+f.DisplayID(), lyr)
		}
	}

	return canvas.Render()
}
