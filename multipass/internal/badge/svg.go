package badge

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"multipass/internal/pdf417"
)

// Card is everything engraved on one member's badge.
type Card struct {
	Org      string    // makerspace name, across the top
	Name     string    // member's full name
	MemberID string    // Authentik PK
	Serial   int       // badge serial in the store
	Since    string    // year the member joined, if known
	Issued   time.Time // when this badge was made
	URL      string    // what the QR code encodes
	Sig      string    // badge signature; also seeds the rosette
	QR       [][]bool  // QR modules, row by row, without a quiet zone
	Slot     bool      // cut a lanyard slot
}

// The badge is a portrait CR80 card, the size of a credit card, in mm.
const (
	CardWidth  = 53.98
	CardHeight = 85.6
	cornerR    = 4.5
	cx         = CardWidth / 2
)

// Layers are told apart by color, so each can get its own settings in xTool
// Creative Space: cut through, score the fine lines, fill-engrave the QR.
const (
	colorCut      = "#ff0000"
	colorPattern  = "#0000ff"
	colorText     = "#00a000"
	colorQR       = "#000000"
	colorAlign    = "#808080" // outline only for lining up the back; don't output
	hairline      = "0.05"
	penWeightStep = 0.07 // offset between passes of bold text: about a kerf
)

// Render returns the badge as an SVG, in millimetres, ready to import into
// xTool Creative Space.
func Render(c Card) []byte {
	seed := sha256.Sum256([]byte("rosette:" + c.Sig))
	card := roundRect{0, 0, CardWidth, CardHeight, cornerR}

	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%smm" height="%smm" viewBox="0 0 %s %s">
<title>%s badge for %s</title>
`, num(CardWidth), num(CardHeight), num(CardWidth), num(CardHeight), xmlEscape(c.Org), xmlEscape(c.Name))

	// Fine guilloche: the border band and the rosette.
	group(&b, "pattern", colorPattern, func(p *pathWriter) {
		p.border(card)
		p.rosette(pt{cx, 31}, 11.5, seed)
	})

	// Lettering, from the name down to the microtext.
	group(&b, "text", colorText, func(p *pathWriter) {
		org := strings.ToUpper(c.Org)
		p.text(org, cx, 14.5, fitCap(org, 3.2, 40), 2)
		micro := " · MULTIPASS · " + org
		p.textFill(micro, 5, 16.6, CardWidth-10, 0.6)

		center := pt{cx, 31}
		p.text(initials(c.Name), center.X, center.Y+1.5, 3.0, 3)
		ring := fmt.Sprintf("%s · MEMBER %s · BADGE %d · %s · ISSUED %s · ", c.Name, c.MemberID, c.Serial, c.Sig, strings.ToUpper(c.Issued.Format("Jan 2006")))
		p.textRing(ring, center, 12.05, 0.65)

		p.text(c.Name, cx, 50, fitCap(c.Name, 4.0, CardWidth-10), 3)
		details := "MEMBER NO. " + padID(c.MemberID)
		if c.Since != "" {
			details += " · SINCE " + c.Since
		}
		p.text(details, cx, 53.8, fitCap(details, 1.6, CardWidth-12), 1)

		shown := strings.TrimPrefix(c.URL, "HTTPS://")
		p.text(shown, cx, 79.6, fitCap(shown, 1.0, CardWidth-10), 1)
	})

	// The QR code, filled. 20mm keeps a version 4 code at 0.6mm modules.
	if len(c.QR) > 0 {
		const size = 20.0
		fmt.Fprintf(&b, `<g id="qr" fill="%s" stroke="none">`+"\n", colorQR)
		b.WriteString(`<path fill-rule="nonzero" d="`)
		writeQR(&b, c.QR, cx-size/2, 56.6, size)
		b.WriteString("\"/>\n</g>\n")
	}

	// Cut last, so the card stays put in the bed until everything is engraved.
	group(&b, "cut", colorCut, func(p *pathWriter) {
		if c.Slot {
			p.closed(slot.outline())
		}
		p.closed(card.outline())
	})

	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// slot is the lanyard slot, centered near the top.
var slot = roundRect{cx - 6.5, 6, cx + 6.5, 8.8, 1.4}

// Back is what's engraved on the back of a badge: who to return it to, and
// the signed AAMVA PDF417 ID scanners read (see barcode.go).
type Back struct {
	Org    string
	Host   string // multipass host, for the return address
	Issuer string // the DID that signs the barcode
	Serial int
	PDF417 [][]bool // modules, one row per barcode row, no quiet zone
}

// BarcodeBox is where the back's PDF417 goes. It runs down the card's
// length, the longest line on it: the bars come out about 0.27mm, close to
// a licence's, where across the card they'd be too fine to burn. Pass its
// Height as the symbol's width and Width as its height when fitting one.
var BarcodeBox = struct{ X, Y, Width, Height float64 }{5.5, 19.8, CardWidth - 11, 58.4}

// RenderBack returns the back of a badge as an SVG. Engrave it after the
// front is cut: flip the card over in its hole in the stock, which the grey
// outline (set to not output) helps line up. The card flips left to right,
// and every element here is centered, so the slot lands where it was cut.
func RenderBack(c Back) []byte {
	card := roundRect{0, 0, CardWidth, CardHeight, cornerR}
	org := strings.ToUpper(c.Org)
	host := strings.ToUpper(c.Host)

	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%smm" height="%smm" viewBox="0 0 %s %s">
<title>%s badge %d, back</title>
`, num(CardWidth), num(CardHeight), num(CardWidth), num(CardHeight), xmlEscape(c.Org), c.Serial)

	group(&b, "align", colorAlign, func(p *pathWriter) {
		p.closed(card.outline())
		p.closed(slot.outline())
	})

	group(&b, "pattern", colorPattern, func(p *pathWriter) {
		p.border(card)
	})

	group(&b, "text", colorText, func(p *pathWriter) {
		p.text("IF FOUND, PLEASE RETURN TO", cx, 12.2, 1.4, 1)
		p.text(org, cx, 15.6, fitCap(org, 2.4, CardWidth-12), 2)
		p.text(host, cx, 18.2, fitCap(host, 1.2, CardWidth-12), 1)

		// The serial, member and issue date are in the barcode, and on the front
		signer := fmt.Sprintf("BADGE %d · SIGNED BY %s · NOT A GOVERNMENT ID", c.Serial, strings.ToUpper(c.Issuer))
		p.text(signer, cx, 80.4, fitCap(signer, 0.8, CardWidth-11), 1)
	})

	if len(c.PDF417) > 0 {
		fmt.Fprintf(&b, `<g id="pdf417" fill="%s" stroke="none">`+"\n", colorQR)
		b.WriteString(`<path fill-rule="nonzero" d="`)
		box := BarcodeBox
		writePDF417Down(&b, c.PDF417, box.X, box.Y, box.Width, box.Height)
		b.WriteString("\"/>\n</g>\n")
	}

	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// border is the guilloche band just inside the card's edge.
func (p *pathWriter) border(card roundRect) {
	frame := card.inset(2.6)
	p.closed(card.inset(1.45).outline())
	p.closed(card.inset(3.75).outline())
	waves := int(math.Round(frame.perimeter() / 2.6))
	p.closedAll(band(frame, 0.85, waves, 5))
}

// rosette is two woven rings around a clear center, their lobe counts and
// twist taken from seed so no two members' rosettes match.
func (p *pathWriter) rosette(center pt, R float64, seed [32]byte) {
	p.closed(circle(center, R))
	p.closed(circle(center, 0.36*R))
	lobes1, copies1 := 17+int(seed[0]%7), 9+int(seed[1]%4)
	lobes2, copies2 := 9+int(seed[2]%5), 7+int(seed[3]%3)
	twist := float64(seed[4]) / 255 * 2 * math.Pi
	p.closedAll(lobedRing(center, 0.81*R, 0.14*R, 0.02*R, lobes1, copies1, twist))
	p.closedAll(lobedRing(center, 0.56*R, 0.12*R, 0.015*R, lobes2, copies2, -twist))
}

func group(b *strings.Builder, id, color string, draw func(*pathWriter)) {
	fmt.Fprintf(b, `<g id="%s" fill="none" stroke="%s" stroke-width="%s" stroke-linecap="round" stroke-linejoin="round">`+"\n", id, color, hairline)
	p := &pathWriter{b: b}
	draw(p)
	b.WriteString("</g>\n")
}

// pathWriter writes each polyline as its own <path>, so the laser software
// sees separate objects and an operator can pick them apart.
type pathWriter struct {
	b *strings.Builder
}

func (p *pathWriter) line(pts []pt, closed bool) {
	if len(pts) < 2 {
		return
	}
	p.b.WriteString(`<path d="M`)
	for i, q := range pts {
		if i > 0 {
			p.b.WriteString(" ")
		}
		p.b.WriteString(num(q.X) + "," + num(q.Y))
	}
	if closed {
		p.b.WriteString("Z")
	}
	p.b.WriteString("\"/>\n")
}

func (p *pathWriter) closed(pts []pt) { p.line(pts, true) }

func (p *pathWriter) closedAll(curves [][]pt) {
	for _, c := range curves {
		p.closed(c)
	}
}

// text sets s centered on (x, baseline) at cap height h. weight > 1 scores
// extra passes on a ring a kerf wide, which reads as a heavier stroke.
func (p *pathWriter) text(s string, x, baseline, h float64, weight int) {
	w := textWidth(s, h)
	p.place(s, h, trackUnits*h/capUnits, func(q pt, pen float64) pt {
		return pt{x - w/2 + pen + q.X, baseline - q.Y}
	}, weight)
}

// textFill repeats s along a line from x, stretching the tracking a little
// so the run ends exactly at width.
func (p *pathWriter) textFill(s string, x, baseline, width, h float64) {
	run, track := repeatToFill(s, h, width, false)
	p.place(run, h, track, func(q pt, pen float64) pt {
		return pt{x + pen + q.X, baseline - q.Y}
	}, 1)
}

// textRing repeats s clockwise around a circle of radius r, glyph tops out,
// bending each glyph to the curve so the ring closes seamlessly.
func (p *pathWriter) textRing(s string, c pt, r, h float64) {
	run, track := repeatToFill(s, h, 2*math.Pi*r, true)
	p.place(run, h, track, func(q pt, pen float64) pt {
		th := -math.Pi/2 + (pen+q.X)/r
		rho := r + q.Y
		return pt{c.X + rho*math.Cos(th), c.Y + rho*math.Sin(th)}
	}, 1)
}

func (p *pathWriter) place(s string, h, track float64, to func(q pt, pen float64) pt, weight int) {
	offsets := []pt{{0, 0}}
	for i := 1; i < weight; i++ {
		a := 2 * math.Pi * float64(i-1) / float64(weight-1)
		offsets = append(offsets, pt{penWeightStep * math.Cos(a), penWeightStep * math.Sin(a)})
	}
	layoutText(s, h, track, func(strokes [][]pt, pen float64) {
		for _, off := range offsets {
			for _, st := range strokes {
				out := make([]pt, len(st))
				for i, q := range st {
					w := to(q, pen)
					out[i] = pt{w.X + off.X, w.Y + off.Y}
				}
				p.line(out, false)
			}
		}
	})
}

// repeatToFill repeats s enough times to span length at cap height h, and
// returns the tracking that makes it fit exactly. A ring also needs the gap
// that wraps from the last glyph back around to the first.
func repeatToFill(s string, h, length float64, wrap bool) (string, float64) {
	defaultTrack := trackUnits * h / capUnits
	unit := layoutText(s, h, defaultTrack, nil) + defaultTrack
	n := int(math.Max(1, math.Round(length/unit)))
	run := strings.TrimRight(strings.Repeat(s, n), " ")
	gaps := len([]rune(foldText(run))) - 1
	if wrap {
		run = strings.Repeat(s, n)
		gaps = len([]rune(foldText(run)))
	}
	bare := layoutText(run, h, 0, nil)
	return run, (length - bare) / float64(gaps)
}

// fitCap is the cap height that sets s at most maxWidth wide, up to h.
func fitCap(s string, h, maxWidth float64) float64 {
	if w := textWidth(s, h); w > maxWidth {
		return h * maxWidth / w
	}
	return h
}

func initials(name string) string {
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return string([]rune(parts[0])[:1])
	default:
		return string([]rune(parts[0])[:1]) + string([]rune(parts[len(parts)-1])[:1])
	}
}

func padID(id string) string {
	for len(id) < 4 {
		id = "0" + id
	}
	return id
}

// writeQR emits each horizontal run of dark modules as one rectangle. Runs in
// neighbouring rows share edges exactly, so the fill has no seams.
func writeQR(b *strings.Builder, m [][]bool, x, y, size float64) {
	mod := size / float64(len(m))
	for r, row := range m {
		for c := 0; c < len(row); {
			if !row[c] {
				c++
				continue
			}
			start := c
			for c < len(row) && row[c] {
				c++
			}
			fmt.Fprintf(b, "M%s,%sh%sv%sh-%sz", num(x+float64(start)*mod), num(y+float64(r)*mod),
				num(float64(c-start)*mod), num(mod), num(float64(c-start)*mod))
		}
	}
}

// writePDF417Down fits the barcode into the box at x, y, turned a quarter
// so its rows run down the card, as large as it goes with rows three modules
// tall, the spec's minimum. The start pattern is at the top and the first
// row at the right. Each dark run becomes one rectangle.
func writePDF417Down(b *strings.Builder, m [][]bool, x, y, w, h float64) {
	cols, rows := float64(len(m[0])), float64(len(m))
	mod := math.Min(h/cols, w/(pdf417.RowHeight*rows))
	rowW := pdf417.RowHeight * mod
	x += (w - rows*rowW) / 2
	y += (h - cols*mod) / 2
	for r, row := range m {
		rx := x + (rows-1-float64(r))*rowW
		for c := 0; c < len(row); {
			if !row[c] {
				c++
				continue
			}
			start := c
			for c < len(row) && row[c] {
				c++
			}
			run := float64(c-start) * mod
			fmt.Fprintf(b, "M%s,%sh%sv%sh-%sz", num(rx), num(y+float64(start)*mod), num(rowW), num(run), num(rowW))
		}
	}
}

func num(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
