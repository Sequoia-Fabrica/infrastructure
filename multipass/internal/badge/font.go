package badge

import (
	"math"
	"strconv"
	"strings"
)

// The badge is lettered in a single-stroke engraver's font, like the Hershey
// fonts pen plotters used: each glyph is a few centerlines the laser scores
// once. Outline fonts fill-engraved below ~1.5mm turn into smudges on wood;
// centerlines stay legible down to well under a millimetre.
//
// Glyphs are drawn with a cap height of 10 units, y up from the baseline.
// Strokes are separated by ";". A stroke is a run of "x,y" points and
// "@cx,cy,rx,ry,a0,a1" elliptical arcs, swept from a0 to a1 degrees
// (counterclockwise when a1 > a0).

const (
	capUnits     = 10.0
	trackUnits   = 2.4 // gap between glyphs
	spaceAdvance = 5.0
)

type glyph struct {
	width   float64
	strokes [][]pt
}

var glyphSpecs = map[rune]struct {
	width float64
	spec  string
}{
	'A': {6, "0,0 3,10 6,0; 1.1,3.6 4.9,3.6"},
	'B': {6, "0,0 0,10 3.6,10 @3.6,7.6,2.3,2.4,90,-90 0,5.2; 3.4,5.2 @3.4,2.6,2.6,2.6,90,-90 0,0"},
	'C': {6, "@3.2,5,3.1,5,48,312"},
	'D': {6, "0,0 0,10 2.6,10 @2.6,5,3.4,5,90,-90 0,0"},
	'E': {5.4, "5.4,10 0,10 0,0 5.4,0; 0,5.2 4.4,5.2"},
	'F': {5.4, "5.4,10 0,10 0,0; 0,5.2 4.4,5.2"},
	'G': {6.2, "@3.2,5,3.1,5,48,360 6.3,3.6 6.3,0.8; 6.3,4.4 3.6,4.4"},
	'H': {6, "0,0 0,10; 6,0 6,10; 0,5.2 6,5.2"},
	'I': {0, "0,0 0,10"},
	'J': {5, "5,10 5,2.8 @2.5,2.8,2.5,2.8,0,-180"},
	'K': {6, "0,0 0,10; 6,10 0,3.4; 2.1,5.3 6,0"},
	'L': {5.2, "0,10 0,0 5.2,0"},
	'M': {7.2, "0,0 0,10 3.6,2.6 7.2,10 7.2,0"},
	'N': {6, "0,0 0,10 6,0 6,10"},
	'O': {6.6, "@3.3,5,3.3,5,0,360"},
	'P': {5.8, "0,0 0,10 3.4,10 @3.4,7.3,2.4,2.7,90,-90 0,4.6"},
	'Q': {6.6, "@3.3,5,3.3,5,0,360; 3.9,2.6 6.8,-0.6"},
	'R': {5.8, "0,0 0,10 3.4,10 @3.4,7.4,2.4,2.6,90,-90 0,4.8; 2.8,4.8 5.8,0"},
	'S': {6, "@3,7.5,2.8,2.5,25,270 @3,2.5,3,2.5,90,-155"},
	'T': {6, "0,10 6,10; 3,10 3,0"},
	'U': {6, "0,10 0,3 @3,3,3,3,180,360 6,10"},
	'V': {6, "0,10 3,0 6,10"},
	'W': {8.4, "0,10 2.1,0 4.2,7.4 6.3,0 8.4,10"},
	'X': {6, "0,0 6,10; 0,10 6,0"},
	'Y': {6, "0,10 3,5 6,10; 3,5 3,0"},
	'Z': {6, "0,10 6,10 0,0 6,0"},

	'0': {5.6, "@2.8,5,2.8,5,0,360; 4.7,8.4 0.9,1.6"},
	'1': {4.4, "0.6,7.8 2.6,10 2.6,0; 0.4,0 4.8,0"},
	'2': {5.8, "@2.9,7.2,2.8,2.8,160,-38 0,0 5.8,0"},
	'3': {5.8, "@2.9,7.5,2.6,2.5,150,-90 @2.9,2.6,2.9,2.6,90,-150"},
	'4': {6.2, "4.6,0 4.6,10 0,3 6.2,3"},
	'5': {5.8, "5.4,10 0.9,10 0.5,5.2 @2.9,3.2,2.9,3.2,140,-150"},
	'6': {6, "4.6,10 0.5,4.5; @3,3,3,3,0,360"},
	'7': {5.8, "0,10 5.8,10 1.9,0"},
	'8': {6, "@3,7.6,2.5,2.4,0,360; @3,2.6,3,2.6,0,360"},
	'9': {6, "1.4,0 5.5,5.5; @3,7,3,3,0,360"},

	'.':  {0, "0,0 0,0.4"},
	',':  {0.6, "0.6,0.4 0,-1.4"},
	':':  {0, "0,0 0,0.4; 0,6 0,6.4"},
	'-':  {4, "0,4.4 4,4.4"},
	'·':  {0, "0,4.6 0,5.0"},
	'/':  {5, "0,-0.4 5,10.4"},
	'#':  {6.4, "1.8,0 2.8,10; 4,0 5,10; 0.2,3.3 6.2,3.3; 0.6,6.7 6.6,6.7"},
	'\'': {0, "0,10 0,7.4"},
	'(':  {2.2, "@5.2,5,5.2,6.2,126,234"},
	')':  {2.2, "@-3,5,5.2,6.2,54,-54"},
	'+':  {5, "0,5 5,5; 2.5,2.5 2.5,7.5"},
	'*':  {5, "2.5,2.4 2.5,7.6; 0.2,6.3 4.8,3.7; 0.2,3.7 4.8,6.3"},
	'&':  {6.2, "6.2,0 1.4,6.6 @2.9,8.2,1.6,1.8,215,-30 1,2.8 @2.6,2.2,2.2,2.2,155,270 6,4.4"},
	'?':  {5.6, "@2.8,7.4,2.8,2.6,165,-50 2.8,4.2 2.8,2.6; 2.8,0 2.8,0.4"},
}

var font = func() map[rune]glyph {
	f := make(map[rune]glyph, len(glyphSpecs))
	for r, g := range glyphSpecs {
		f[r] = glyph{width: g.width, strokes: parseGlyph(g.spec)}
	}
	return f
}()

func parseGlyph(spec string) [][]pt {
	var strokes [][]pt
	for _, s := range strings.Split(spec, ";") {
		var stroke []pt
		for _, tok := range strings.Fields(s) {
			if strings.HasPrefix(tok, "@") {
				v := parseFloats(tok[1:], 6)
				stroke = append(stroke, arc(pt{v[0], v[1]}, v[2], v[3], v[4], v[5])...)
				continue
			}
			v := parseFloats(tok, 2)
			stroke = append(stroke, pt{v[0], v[1]})
		}
		if len(stroke) == 1 {
			// A lone point would be invisible; nudge it into a dot.
			stroke = append(stroke, pt{stroke[0].X, stroke[0].Y + 0.4})
		}
		strokes = append(strokes, stroke)
	}
	return strokes
}

func parseFloats(s string, n int) []float64 {
	parts := strings.Split(s, ",")
	if len(parts) != n {
		panic("badge font: bad glyph token " + s)
	}
	out := make([]float64, n)
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			panic("badge font: bad glyph token " + s)
		}
		out[i] = v
	}
	return out
}

// arc samples an elliptical arc every few degrees.
func arc(c pt, rx, ry, a0, a1 float64) []pt {
	steps := int(math.Ceil(math.Abs(a1-a0) / 6))
	if steps < 2 {
		steps = 2
	}
	out := make([]pt, 0, steps+1)
	for i := 0; i <= steps; i++ {
		a := (a0 + (a1-a0)*float64(i)/float64(steps)) * math.Pi / 180
		out = append(out, pt{c.X + rx*math.Cos(a), c.Y + ry*math.Sin(a)})
	}
	return out
}

// foldText uppercases s and folds the accented Latin letters people's names
// carry onto the plain letters the font has.
func foldText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if f, ok := accentFolds[r]; ok {
			b.WriteString(f)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var accentFolds = func() map[rune]string {
	m := map[rune]string{'Æ': "AE", 'Œ': "OE", 'ß': "SS", 'Ð': "D", 'Þ': "TH", 'Ł': "L", 'Ø': "O"}
	for base, accented := range map[string]string{
		"A": "ÀÁÂÃÄÅĀĂĄ", "C": "ÇĆĈĊČ", "D": "Ď", "E": "ÈÉÊËĒĔĖĘĚ", "G": "ĜĞĠĢ",
		"H": "ĤĦ", "I": "ÌÍÎÏĨĪĬĮİ", "J": "Ĵ", "K": "Ķ", "L": "ĹĻĽĿ", "N": "ÑŃŅŇ",
		"O": "ÒÓÔÕÖŌŎŐ", "R": "ŔŖŘ", "S": "ŚŜŞŠ", "T": "ŢŤŦ", "U": "ÙÚÛÜŨŪŬŮŰŲ",
		"W": "Ŵ", "Y": "ÝŶŸ", "Z": "ŹŻŽ",
	} {
		for _, r := range accented {
			m[r] = base
		}
	}
	return m
}()

// textWidth is the width of s set at cap height h with the default tracking.
func textWidth(s string, h float64) float64 {
	return layoutText(s, h, trackUnits*h/capUnits, nil)
}

// layoutText walks s glyph by glyph, calling place with each glyph's strokes
// scaled to cap height h and its pen position along the baseline. It returns
// the advance of the whole string. Characters the font lacks become "?".
func layoutText(s string, h, track float64, place func(strokes [][]pt, x float64)) float64 {
	scale := h / capUnits
	x := 0.0
	first := true
	for _, r := range foldText(s) {
		if !first {
			x += track
		}
		first = false
		if r == ' ' {
			x += spaceAdvance * scale
			continue
		}
		g, ok := font[r]
		if !ok {
			g = font['?']
		}
		if place != nil {
			scaled := make([][]pt, len(g.strokes))
			for i, st := range g.strokes {
				scaled[i] = make([]pt, len(st))
				for j, p := range st {
					scaled[i][j] = pt{p.X * scale, p.Y * scale}
				}
			}
			place(scaled, x)
		}
		x += g.width * scale
	}
	return x
}
