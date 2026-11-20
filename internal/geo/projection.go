package geo

import "math"

// Viewport defines the current geographic camera for the radar map.
type Viewport struct {
	Name      string
	CenterLat float64
	CenterLon float64
	SpanLon   float64 // Total longitude degrees shown across width
}

// RegionPreset represents a quick-jump geographic preset.
type RegionPreset struct {
	Key      string
	Name     string
	Viewport Viewport
}

// Presets are the quick-jump regions bound to keys 1-5.
var Presets = []RegionPreset{
	{Key: "1", Name: "DÜNYA", Viewport: Viewport{Name: "DÜNYA", CenterLat: 22.0, CenterLon: 15.0, SpanLon: 340.0}},
	{Key: "2", Name: "TÜRKİYE", Viewport: Viewport{Name: "TÜRKİYE", CenterLat: 39.0, CenterLon: 35.0, SpanLon: 24.0}},
	{Key: "3", Name: "AVRUPA", Viewport: Viewport{Name: "AVRUPA", CenterLat: 49.0, CenterLon: 11.0, SpanLon: 48.0}},
	{Key: "4", Name: "ORTA DOĞU", Viewport: Viewport{Name: "ORTA DOĞU", CenterLat: 28.0, CenterLon: 44.0, SpanLon: 44.0}},
	{Key: "5", Name: "ABD", Viewport: Viewport{Name: "ABD", CenterLat: 39.0, CenterLon: -98.0, SpanLon: 64.0}},
}

// SpanLat computes the visible latitude span for a given terminal cell width/height,
// accounting for typical terminal character aspect ratio (~2:1 height:width) and cos(lat) scaling.
func (v Viewport) SpanLat(cellW, cellH int) float64 {
	if cellW <= 0 || cellH <= 0 {
		return v.SpanLon / 2.0
	}
	cosLat := math.Cos(v.CenterLat * math.Pi / 180.0)
	if cosLat < 0.35 {
		cosLat = 0.35
	}
	// Terminal cells are roughly twice as tall as they are wide.
	spanLat := v.SpanLon * (float64(cellH) * 2.0 / float64(cellW)) * cosLat
	if spanLat > 170.0 {
		spanLat = 170.0
	}
	return spanLat
}

// Bounds returns (minLat, maxLat, minLon, maxLon) for the current viewport.
func (v Viewport) Bounds(cellW, cellH int) (minLat, maxLat, minLon, maxLon float64) {
	spanLat := v.SpanLat(cellW, cellH)
	minLat = v.CenterLat - spanLat/2.0
	maxLat = v.CenterLat + spanLat/2.0
	minLon = v.CenterLon - v.SpanLon/2.0
	maxLon = v.CenterLon + v.SpanLon/2.0
	return
}

// Project converts (lat, lon) to floating-point coordinates in [0..w, 0..h].
func (v Viewport) Project(lat, lon float64, cellW, cellH int, targetW, targetH float64) (x, y float64, visible bool) {
	minLat, maxLat, minLon, maxLon := v.Bounds(cellW, cellH)
	if maxLon <= minLon || maxLat <= minLat {
		return 0, 0, false
	}
	nx := (lon - minLon) / (maxLon - minLon)
	ny := (maxLat - lat) / (maxLat - minLat)
	x = nx * targetW
	y = ny * targetH
	visible = nx >= 0 && nx <= 1 && ny >= 0 && ny <= 1
	return
}

// Pan moves the viewport by fractional amounts of the current span.
func (v *Viewport) Pan(dLonFrac, dLatFrac float64, cellW, cellH int) {
	v.Name = "ÖZEL"
	v.CenterLon += v.SpanLon * dLonFrac
	v.CenterLat += v.SpanLat(cellW, cellH) * dLatFrac
	v.clampCenter()
}

// PanByCells shifts the map so that a point dragged by (dxCells, dyCells) stays under the cursor.
func (v *Viewport) PanByCells(dxCells, dyCells int, cellW, cellH int) {
	if cellW <= 1 || cellH <= 1 || (dxCells == 0 && dyCells == 0) {
		return
	}
	v.Name = "ÖZEL"
	spanLat := v.SpanLat(cellW, cellH)
	v.CenterLon -= (float64(dxCells) / float64(cellW-1)) * v.SpanLon
	v.CenterLat += (float64(dyCells) / float64(cellH-1)) * spanLat
	v.clampCenter()
}

// Unproject converts terminal cell coordinates (cx in [0..cellW-1], cy in [0..cellH-1])
// back into geographic (lat, lon) coordinates.
func (v Viewport) Unproject(cx, cy float64, cellW, cellH int) (lat, lon float64) {
	minLat, maxLat, minLon, maxLon := v.Bounds(cellW, cellH)
	if cellW <= 1 || cellH <= 1 {
		return v.CenterLat, v.CenterLon
	}
	nx := cx / float64(cellW-1)
	ny := cy / float64(cellH-1)
	lon = minLon + nx*(maxLon-minLon)
	lat = maxLat - ny*(maxLat-minLat)
	return lat, lon
}

// Zoom multiplies SpanLon by factor (<1 zooms in, >1 zooms out).
func (v *Viewport) Zoom(factor float64) {
	v.SpanLon *= factor
	if v.SpanLon < 0.8 {
		v.SpanLon = 0.8
	}
	if v.SpanLon > 360.0 {
		v.SpanLon = 360.0
	}
}

// ZoomAt zooms by factor while keeping the geographic coordinate under (anchorX, anchorY) fixed.
func (v *Viewport) ZoomAt(factor float64, anchorX, anchorY float64, cellW, cellH int) {
	if cellW <= 1 || cellH <= 1 {
		v.Zoom(factor)
		return
	}
	targetLat, targetLon := v.Unproject(anchorX, anchorY, cellW, cellH)
	oldSpan := v.SpanLon
	v.Zoom(factor)
	if v.SpanLon == oldSpan {
		return
	}
	v.Name = "ÖZEL"
	// Recompute center so targetLat, targetLon projects back to (anchorX, anchorY).
	nx := anchorX / float64(cellW-1)
	ny := anchorY / float64(cellH-1)
	v.CenterLon = targetLon - (nx-0.5)*v.SpanLon
	// Iterate twice to account for cos(CenterLat) dependency in SpanLat.
	for i := 0; i < 2; i++ {
		spanLat := v.SpanLat(cellW, cellH)
		v.CenterLat = targetLat + (ny-0.5)*spanLat
		v.clampCenter()
	}
}

func (v *Viewport) clampCenter() {
	if v.CenterLat > 80 {
		v.CenterLat = 80
	}
	if v.CenterLat < -80 {
		v.CenterLat = -80
	}
	for v.CenterLon > 180 {
		v.CenterLon -= 360
	}
	for v.CenterLon < -180 {
		v.CenterLon += 360
	}
}

// ProjectPoint advances (lat, lon) along trackDeg at speedKt for dtSec seconds (dead reckoning).
func ProjectPoint(lat, lon, speedKt, trackDeg, dtSec float64) (newLat, newLon float64) {
	if speedKt <= 0 || dtSec <= 0 {
		return lat, lon
	}
	const earthRadiusNM = 3440.065
	distNM := speedKt * (dtSec / 3600.0)
	angDist := distNM / earthRadiusNM
	brng := trackDeg * math.Pi / 180.0
	phi1 := lat * math.Pi / 180.0
	lam1 := lon * math.Pi / 180.0

	sinPhi2 := math.Sin(phi1)*math.Cos(angDist) + math.Cos(phi1)*math.Sin(angDist)*math.Cos(brng)
	if sinPhi2 > 1 {
		sinPhi2 = 1
	} else if sinPhi2 < -1 {
		sinPhi2 = -1
	}
	phi2 := math.Asin(sinPhi2)
	lam2 := lam1 + math.Atan2(
		math.Sin(brng)*math.Sin(angDist)*math.Cos(phi1),
		math.Cos(angDist)-math.Sin(phi1)*sinPhi2,
	)

	newLat = phi2 * 180.0 / math.Pi
	newLon = lam2 * 180.0 / math.Pi
	for newLon > 180 {
		newLon -= 360
	}
	for newLon < -180 {
		newLon += 360
	}
	return newLat, newLon
}

// CirclePoints generates closed polyline points [lon, lat] for a geographic circle of radiusNM.
func CirclePoints(centerLat, centerLon, radiusNM float64, steps int) [][2]float64 {
	if steps < 12 {
		steps = 12
	}
	pts := make([][2]float64, 0, steps+1)
	for i := 0; i <= steps; i++ {
		brng := float64(i) * 360.0 / float64(steps)
		lat, lon := ProjectPoint(centerLat, centerLon, radiusNM*3600.0, brng, 1.0)
		pts = append(pts, [2]float64{lon, lat})
	}
	return pts
}

// RadiusNM estimates the radius in nautical miles from the center to the edge of the viewport.
func (v Viewport) RadiusNM(cellW, cellH int) int {
	minLat, maxLat, minLon, maxLon := v.Bounds(cellW, cellH)
	d := HaversineNM(v.CenterLat, v.CenterLon, maxLat, maxLon)
	d2 := HaversineNM(v.CenterLat, v.CenterLon, minLat, minLon)
	if d2 > d {
		d = d2
	}
	return int(math.Ceil(d))
}

// HaversineNM computes the great-circle distance in nautical miles between two points.
func HaversineNM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusNM = 3440.065
	rLat1 := lat1 * math.Pi / 180.0
	rLat2 := lat2 * math.Pi / 180.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rLat1)*math.Cos(rLat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusNM * c
}

// GreatCirclePoints generates intermediate points along the great circle between two coordinates.
func GreatCirclePoints(lat1, lon1, lat2, lon2 float64, steps int) [][2]float64 {
	if steps < 2 {
		steps = 2
	}
	phi1 := lat1 * math.Pi / 180.0
	lam1 := lon1 * math.Pi / 180.0
	phi2 := lat2 * math.Pi / 180.0
	lam2 := lon2 * math.Pi / 180.0

	// Unit vectors
	x1 := math.Cos(phi1) * math.Cos(lam1)
	y1 := math.Cos(phi1) * math.Sin(lam1)
	z1 := math.Sin(phi1)

	x2 := math.Cos(phi2) * math.Cos(lam2)
	y2 := math.Cos(phi2) * math.Sin(lam2)
	z2 := math.Sin(phi2)

	dot := x1*x2 + y1*y2 + z1*z2
	if dot > 1 {
		dot = 1
	} else if dot < -1 {
		dot = -1
	}
	omega := math.Acos(dot)
	sinOmega := math.Sin(omega)

	pts := make([][2]float64, 0, steps+1)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		var x, y, z float64
		if sinOmega < 1e-6 {
			x = x1*(1-t) + x2*t
			y = y1*(1-t) + y2*t
			z = z1*(1-t) + z2*t
		} else {
			a := math.Sin((1-t)*omega) / sinOmega
			b := math.Sin(t*omega) / sinOmega
			x = a*x1 + b*x2
			y = a*y1 + b*y2
			z = a*z1 + b*z2
		}
		lat := math.Atan2(z, math.Sqrt(x*x+y*y)) * 180.0 / math.Pi
		lon := math.Atan2(y, x) * 180.0 / math.Pi
		pts = append(pts, [2]float64{lat, lon})
	}
	return pts
}
