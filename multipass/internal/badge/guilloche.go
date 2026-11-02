package badge

import "math"

type pt struct{ X, Y float64 }

// lobedRing overlays phase-shifted copies of the closed curve
// ρ(θ) = r + a·sin(lobes·θ + φ) + b·sin(2·lobes·θ − φ). Overlaid, the copies
// weave into the lattice that banknote rosettes are cut from.
func lobedRing(c pt, r, a, b float64, lobes, copies int, twist float64) [][]pt {
	steps := lobes * 32
	out := make([][]pt, 0, copies)
	for k := 0; k < copies; k++ {
		phase := 2*math.Pi*float64(k)/float64(copies) + twist
		curve := make([]pt, 0, steps+1)
		for i := 0; i <= steps; i++ {
			th := 2 * math.Pi * float64(i) / float64(steps)
			n := float64(lobes) * th
			rho := r + a*math.Sin(n+phase) + b*math.Sin(2*n-phase)
			curve = append(curve, pt{c.X + rho*math.Cos(th), c.Y + rho*math.Sin(th)})
		}
		out = append(out, curve)
	}
	return out
}

// circle samples a full circle finely enough to look round when scored.
func circle(c pt, r float64) []pt {
	return arc(c, r, r, 0, 360)
}

// roundRect is a rounded rectangle that curves can be laid along.
type roundRect struct {
	x0, y0, x1, y1, r float64
}

func (rr roundRect) inset(d float64) roundRect {
	return roundRect{rr.x0 + d, rr.y0 + d, rr.x1 - d, rr.y1 - d, math.Max(rr.r-d, 0)}
}

func (rr roundRect) perimeter() float64 {
	return 2*(rr.x1-rr.x0-2*rr.r) + 2*(rr.y1-rr.y0-2*rr.r) + 2*math.Pi*rr.r
}

// at returns the point s along the perimeter and the outward normal there.
// The walk starts at the left end of the top edge and runs clockwise as seen
// in SVG's y-down space.
func (rr roundRect) at(s float64) (p, normal pt) {
	r := rr.r
	w := rr.x1 - rr.x0 - 2*r
	h := rr.y1 - rr.y0 - 2*r
	edges := []struct {
		start, dir, normal pt
		length             float64
		corner             pt // center of the corner that follows the edge
		a0                 float64
	}{
		{pt{rr.x0 + r, rr.y0}, pt{1, 0}, pt{0, -1}, w, pt{rr.x1 - r, rr.y0 + r}, -math.Pi / 2},
		{pt{rr.x1, rr.y0 + r}, pt{0, 1}, pt{1, 0}, h, pt{rr.x1 - r, rr.y1 - r}, 0},
		{pt{rr.x1 - r, rr.y1}, pt{-1, 0}, pt{0, 1}, w, pt{rr.x0 + r, rr.y1 - r}, math.Pi / 2},
		{pt{rr.x0, rr.y1 - r}, pt{0, -1}, pt{-1, 0}, h, pt{rr.x0 + r, rr.y0 + r}, math.Pi},
	}
	s = math.Mod(s, rr.perimeter())
	if s < 0 {
		s += rr.perimeter()
	}
	for _, e := range edges {
		if s <= e.length {
			return pt{e.start.X + e.dir.X*s, e.start.Y + e.dir.Y*s}, e.normal
		}
		s -= e.length
		if quarter := math.Pi / 2 * r; r > 0 && s <= quarter {
			a := e.a0 + s/r
			n := pt{math.Cos(a), math.Sin(a)}
			return pt{e.corner.X + r*n.X, e.corner.Y + r*n.Y}, n
		}
		s -= math.Pi / 2 * r
	}
	return edges[0].start, edges[0].normal
}

// band lays phase-shifted copies of a sine wave along rr, each offset from it by
// amp·sin(2π·waves·s/P + φ). The wave count is whole, so every curve closes.
func band(rr roundRect, amp float64, waves, copies int) [][]pt {
	per := rr.perimeter()
	steps := waves * 24
	out := make([][]pt, 0, copies)
	for k := 0; k < copies; k++ {
		phase := 2 * math.Pi * float64(k) / float64(copies)
		curve := make([]pt, 0, steps+1)
		for i := 0; i <= steps; i++ {
			s := per * float64(i) / float64(steps)
			p, n := rr.at(s)
			off := amp * math.Sin(2*math.Pi*float64(waves)*s/per+phase)
			curve = append(curve, pt{p.X + n.X*off, p.Y + n.Y*off})
		}
		out = append(out, curve)
	}
	return out
}

// outline traces rr itself.
func (rr roundRect) outline() []pt {
	per := rr.perimeter()
	steps := 360
	out := make([]pt, 0, steps+1)
	for i := 0; i <= steps; i++ {
		p, _ := rr.at(per * float64(i) / float64(steps))
		out = append(out, p)
	}
	return out
}
