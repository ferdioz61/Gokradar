package geo

import (
	"math"
	"strings"
	"testing"
)

func TestViewportProjectAndBounds(t *testing.T) {
	vp := Presets[1].Viewport // TÜRKİYE
	minLat, maxLat, minLon, maxLon := vp.Bounds(80, 24)
	if minLat >= maxLat || minLon >= maxLon {
		t.Fatalf("invalid bounds: %v %v %v %v", minLat, maxLat, minLon, maxLon)
	}
	// Istanbul should be visible in Turkey preset
	_, _, vis := vp.Project(41.26, 28.74, 80, 24, 79, 23)
	if !vis {
		t.Fatalf("expected Istanbul to be visible in Turkey preset")
	}
}

func TestGreatCirclePoints(t *testing.T) {
	pts := GreatCirclePoints(41.26, 28.74, 50.03, 8.56, 20)
	if len(pts) != 21 {
		t.Fatalf("expected 21 points, got %d", len(pts))
	}
	if pts[0][0] < 41.0 || pts[0][0] > 41.5 {
		t.Fatalf("unexpected start lat: %v", pts[0][0])
	}
}

func TestCanvasBrailleAndASCII(t *testing.T) {
	cBraille := NewCanvas(20, 10, false)
	cBraille.DrawLine(0, 0, 39, 39, LayerCoast, 0)
	cBraille.SetOverlay(10, 5, "↑", LayerPlaneSel)
	out := cBraille.Render()
	if !strings.Contains(out, "↑") {
		t.Fatalf("expected overlay symbol in Braille render")
	}

	cASCII := NewCanvas(20, 10, true)
	cASCII.DrawLine(0, 0, 19, 9, LayerCoast, 0)
	cASCII.SetOverlay(10, 5, "^", LayerPlaneSel)
	outA := cASCII.Render()
	if !strings.Contains(outA, "^") || !strings.Contains(outA, ".") {
		t.Fatalf("expected ASCII markers in ASCII render")
	}
}

func TestUnprojectAndZoomAt(t *testing.T) {
	vp := Presets[1].Viewport // TÜRKİYE
	const w, h = 80, 24
	origLat, origLon := 41.26, 28.74 // IST

	x, y, vis := vp.Project(origLat, origLon, w, h, float64(w-1), float64(h-1))
	if !vis {
		t.Fatalf("expected IST visible")
	}
	uLat, uLon := vp.Unproject(x, y, w, h)
	if math.Abs(uLat-origLat) > 0.01 || math.Abs(uLon-origLon) > 0.01 {
		t.Fatalf("unproject mismatch: got (%.4f, %.4f), want (%.4f, %.4f)", uLat, uLon, origLat, origLon)
	}

	// ZoomAt should keep the geographic point under (x, y) stationary
	vp.ZoomAt(0.6, x, y, w, h)
	nx, ny, vis2 := vp.Project(origLat, origLon, w, h, float64(w-1), float64(h-1))
	if !vis2 {
		t.Fatalf("expected IST still visible after ZoomAt")
	}
	if math.Abs(nx-x) > 0.5 || math.Abs(ny-y) > 0.5 {
		t.Fatalf("ZoomAt drifted anchor: before (%.2f, %.2f), after (%.2f, %.2f)", x, y, nx, ny)
	}
}

func TestProjectPointDeadReckoning(t *testing.T) {
	// Flying due North at 600 kt for 60 seconds should move 10 NM (~10/60 = 0.1667 deg) north
	nLat, nLon := ProjectPoint(40.0, 29.0, 600.0, 0.0, 60.0)
	if math.Abs((nLat-40.0)-0.1667) > 0.01 {
		t.Fatalf("unexpected north displacement: %.4f", nLat-40.0)
	}
	if math.Abs(nLon-29.0) > 0.01 {
		t.Fatalf("unexpected lon drift when heading north: %.4f", nLon-29.0)
	}
}

func TestDrawLabelSmartCollisionAvoidance(t *testing.T) {
	c := NewCanvas(30, 10, false)
	c.SetOverlay(10, 5, "▲", LayerPlaneMid)
	ok1 := c.DrawLabelSmart(10, 5, "THY1831", LayerPlaneMid)
	if !ok1 {
		t.Fatalf("expected first smart label to succeed")
	}
	// Another aircraft right next to it should still place its label elsewhere without panic/overwrite
	c.SetOverlay(10, 5, "▲", LayerPlaneHigh)
	ok2 := c.DrawLabelSmart(10, 5, "PGT102", LayerPlaneHigh)
	if !ok2 {
		t.Fatalf("expected second smart label to find alternative placement")
	}
	out := c.Render()
	if !strings.Contains(out, "THY1831") || !strings.Contains(out, "PGT102") {
		t.Fatalf("expected both labels to appear without colliding")
	}
}
