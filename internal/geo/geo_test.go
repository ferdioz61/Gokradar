package geo

import (
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
