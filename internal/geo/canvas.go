package geo

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Layer identifies the visual priority and color of a drawn element.
type Layer uint8

const (
	LayerNone Layer = iota
	LayerGrid
	LayerRangeRing
	LayerBorder
	LayerCoast
	LayerVector
	LayerRouteRem
	LayerRouteDone
	LayerTrailLow
	LayerTrailMid
	LayerTrailHigh
	LayerAirport
	LayerRouteEndpoint
	LayerScaleBar
	LayerPlaneLow
	LayerPlaneMid
	LayerPlaneHigh
	LayerPlaneFav
	LayerPlaneMil
	LayerPlaneEmerg
	LayerPlaneSel
)

// Aliases kept for readability and backward compatibility.
const (
	LayerRoute = LayerRouteRem
	LayerTrail = LayerTrailMid
)

type cellOverlay struct {
	ch    string
	layer Layer
}

// Canvas renders map vectors (coastlines, borders, trails, routes, range rings, velocity vectors)
// using either high-resolution Unicode Braille (2x4 subpixels per cell) or pure ASCII (1x1 per cell),
// plus crisp character overlays and collision-avoiding labels.
type Canvas struct {
	Width    int
	Height   int
	ASCII    bool
	braille  []uint8
	bgLayer  []Layer
	asciiCh  []byte
	overlays []cellOverlay
	occupied []bool
}

// NewCanvas allocates a new map canvas of size (width x height) terminal cells.
func NewCanvas(width, height int, ascii bool) *Canvas {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	n := width * height
	return &Canvas{
		Width:    width,
		Height:   height,
		ASCII:    ascii,
		braille:  make([]uint8, n),
		bgLayer:  make([]Layer, n),
		asciiCh:  make([]byte, n),
		overlays: make([]cellOverlay, n),
		occupied: make([]bool, n),
	}
}

// PixelSize returns the subpixel dimensions of the canvas.
func (c *Canvas) PixelSize() (int, int) {
	if c.ASCII {
		return c.Width, c.Height
	}
	return c.Width * 2, c.Height * 4
}

// brailleBit maps subpixel (dx in 0..1, dy in 0..3) to Unicode Braille dot bit.
var brailleBit = [4][2]uint8{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// SetPixel turns on a pixel at (px, py) with the given layer priority.
func (c *Canvas) SetPixel(px, py int, layer Layer) {
	pw, ph := c.PixelSize()
	if px < 0 || px >= pw || py < 0 || py >= ph {
		return
	}
	if c.ASCII {
		idx := py*c.Width + px
		if layer >= c.bgLayer[idx] {
			c.bgLayer[idx] = layer
			switch layer {
			case LayerGrid, LayerRangeRing:
				c.asciiCh[idx] = ','
			case LayerBorder:
				c.asciiCh[idx] = ':'
			case LayerCoast:
				c.asciiCh[idx] = '.'
			case LayerVector:
				c.asciiCh[idx] = '\''
			case LayerRouteRem:
				c.asciiCh[idx] = '-'
			case LayerRouteDone:
				c.asciiCh[idx] = '='
			case LayerTrailLow, LayerTrailMid, LayerTrailHigh:
				c.asciiCh[idx] = '~'
			default:
				c.asciiCh[idx] = '*'
			}
		}
		return
	}
	cx := px / 2
	cy := py / 4
	dx := px % 2
	dy := py % 4
	idx := cy*c.Width + cx
	c.braille[idx] |= brailleBit[dy][dx]
	if layer > c.bgLayer[idx] {
		c.bgLayer[idx] = layer
	}
}

// DrawLine draws a line between two pixel coordinates using Bresenham's algorithm.
// If dashed > 0, it skips every other `dashed` pixels.
func (c *Canvas) DrawLine(x0, y0, x1, y1 int, layer Layer, dashed int) {
	dx := int(math.Abs(float64(x1 - x0)))
	dy := -int(math.Abs(float64(y1 - y0)))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	step := 0
	for {
		if dashed <= 0 || (step/dashed)%2 == 0 {
			c.SetPixel(x0, y0, layer)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
		step++
		if step > 4000 {
			break
		}
	}
}

// SetOverlay places a character symbol at terminal cell (cx, cy) if its layer >= existing overlay.
func (c *Canvas) SetOverlay(cx, cy int, ch string, layer Layer) {
	if cx < 0 || cx >= c.Width || cy < 0 || cy >= c.Height {
		return
	}
	idx := cy*c.Width + cx
	if layer >= c.overlays[idx].layer {
		c.overlays[idx] = cellOverlay{ch: ch, layer: layer}
		if layer >= LayerAirport {
			c.occupied[idx] = true
		}
	}
}

// DrawLabel writes a short text label starting at (cx, cy) without overwriting higher layers.
func (c *Canvas) DrawLabel(cx, cy int, label string, layer Layer) {
	if cy < 0 || cy >= c.Height {
		return
	}
	x := cx
	for _, r := range label {
		if x >= 0 && x < c.Width {
			idx := cy*c.Width + x
			if layer > c.overlays[idx].layer {
				c.overlays[idx] = cellOverlay{ch: string(r), layer: layer}
				c.occupied[idx] = true
			}
		}
		x++
	}
}

// DrawLabelForce writes a label starting at (cx, cy) if layer >= existing overlay.
func (c *Canvas) DrawLabelForce(cx, cy int, label string, layer Layer) {
	if cy < 0 || cy >= c.Height {
		return
	}
	x := cx
	for _, r := range label {
		if x >= 0 && x < c.Width {
			idx := cy*c.Width + x
			if layer >= c.overlays[idx].layer {
				c.overlays[idx] = cellOverlay{ch: string(r), layer: layer}
				c.occupied[idx] = true
			}
		}
		x++
	}
}

// canPlaceLabel checks whether a label of width `w` fits cleanly at (startX, y) without collisions.
func (c *Canvas) canPlaceLabel(startX, y, w int) bool {
	if y < 0 || y >= c.Height || startX < 0 || startX+w > c.Width {
		return false
	}
	for x := startX; x < startX+w; x++ {
		if c.occupied[y*c.Width+x] {
			return false
		}
	}
	return true
}

// DrawLabelSmart places a label near anchor (cx, cy) avoiding collisions with aircraft and other labels.
// Tries right, left, above, then below. Returns true if placed.
func (c *Canvas) DrawLabelSmart(cx, cy int, label string, layer Layer) bool {
	runes := []rune(label)
	w := len(runes)
	if w == 0 {
		return false
	}

	candidates := [][2]int{
		{cx + 2, cy},     // right (with 1-space gap)
		{cx + 1, cy},     // right (adjacent)
		{cx - w - 1, cy}, // left (with 1-space gap)
		{cx - w, cy},     // left (adjacent)
		{cx - w/2, cy - 1}, // centered above
		{cx - w/2, cy + 1}, // centered below
	}
	for _, pos := range candidates {
		if c.canPlaceLabel(pos[0], pos[1], w) {
			c.DrawLabelForce(pos[0], pos[1], label, layer)
			return true
		}
	}
	return false
}

// DefaultLayerStyles returns the default Classic Radar color palette.
func DefaultLayerStyles() map[Layer]lipgloss.Style {
	return map[Layer]lipgloss.Style{
		LayerGrid:          lipgloss.NewStyle().Foreground(lipgloss.Color("236")),
		LayerRangeRing:     lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		LayerBorder:        lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
		LayerCoast:         lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		LayerVector:        lipgloss.NewStyle().Foreground(lipgloss.Color("248")),
		LayerRouteRem:      lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		LayerRouteDone:     lipgloss.NewStyle().Foreground(lipgloss.Color("51")),
		LayerTrailLow:      lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		LayerTrailMid:      lipgloss.NewStyle().Foreground(lipgloss.Color("45")),
		LayerTrailHigh:     lipgloss.NewStyle().Foreground(lipgloss.Color("141")),
		LayerAirport:       lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
		LayerRouteEndpoint: lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true),
		LayerScaleBar:      lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("235")),
		LayerPlaneLow:      lipgloss.NewStyle().Foreground(lipgloss.Color("82")),
		LayerPlaneMid:      lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		LayerPlaneHigh:     lipgloss.NewStyle().Foreground(lipgloss.Color("141")),
		LayerPlaneFav:      lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true),
		LayerPlaneMil:      lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true),
		LayerPlaneEmerg:    lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		LayerPlaneSel:      lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Background(lipgloss.Color("24")).Bold(true),
	}
}

var defaultStyles = DefaultLayerStyles()

// Render converts the canvas into ANSI-styled terminal lines using the default palette.
func (c *Canvas) Render() string {
	return c.RenderWithStyles(defaultStyles)
}

// RenderWithStyles converts the canvas into ANSI-styled terminal lines using custom theme styles.
func (c *Canvas) RenderWithStyles(styles map[Layer]lipgloss.Style) string {
	if styles == nil {
		styles = defaultStyles
	}
	var sb strings.Builder
	sb.Grow(c.Width * c.Height * 4)

	for y := 0; y < c.Height; y++ {
		var runBuf strings.Builder
		curLayer := LayerNone

		flush := func() {
			if runBuf.Len() == 0 {
				return
			}
			if curLayer == LayerNone {
				sb.WriteString(runBuf.String())
			} else if st, ok := styles[curLayer]; ok {
				sb.WriteString(st.Render(runBuf.String()))
			} else {
				sb.WriteString(runBuf.String())
			}
			runBuf.Reset()
		}

		for x := 0; x < c.Width; x++ {
			idx := y*c.Width + x
			var ch string
			var lyr Layer

			if c.overlays[idx].layer != LayerNone {
				ch = c.overlays[idx].ch
				lyr = c.overlays[idx].layer
			} else if c.ASCII {
				if c.asciiCh[idx] != 0 {
					ch = string(c.asciiCh[idx])
					lyr = c.bgLayer[idx]
				} else {
					ch = " "
					lyr = LayerNone
				}
			} else {
				if c.braille[idx] != 0 {
					ch = string(rune(0x2800 + int(c.braille[idx])))
					lyr = c.bgLayer[idx]
				} else {
					ch = " "
					lyr = LayerNone
				}
			}

			if lyr != curLayer {
				flush()
				curLayer = lyr
			}
			runBuf.WriteString(ch)
		}
		flush()
		if y < c.Height-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
