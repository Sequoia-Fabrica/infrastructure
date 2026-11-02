package pdf417

import (
	"image/color"
	"strings"
	"testing"

	upstream "github.com/boombuler/barcode/pdf417"
)

// upstreamMatrix is the upstream encoder's symbol as rows of modules.
func upstreamMatrix(t *testing.T, data string, level byte) [][]bool {
	code, err := upstream.Encode(data, level)
	if err != nil {
		t.Fatal(err)
	}
	b := code.Bounds()
	var rows [][]bool
	for y := b.Min.Y; y < b.Max.Y; y += 2 { // upstream draws rows 2px tall
		row := make([]bool, b.Dx())
		for x := range row {
			row[x] = color.GrayModel.Convert(code.At(b.Min.X+x, y)).(color.Gray).Y < 128
		}
		rows = append(rows, row)
	}
	return rows
}

func TestMatchesUpstream(t *testing.T) {
	// Upstream's left row indicator uses (rows-3)/3 where the spec (and
	// ZXing) use (rows-1)/3; the two agree when rows is a multiple of 3, so
	// compare there
	compared := 0
	for n := 20; n < 400 && compared < 5; n += 7 {
		data := "@\n\x1e\rANSI " + strings.Repeat("DAQ123\nDCSSMITH-JONES\n", n/20) + strings.Repeat("x", n%20)
		for _, level := range []byte{2, 5} {
			want := upstreamMatrix(t, data, level)
			if len(want)%3 != 0 {
				continue
			}
			cols := (len(want[0])-1)/17 - 4
			words, err := highlevelEncode(data)
			if err != nil {
				t.Fatal(err)
			}
			got, err := matrix(words, securitylevel(level), cols)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("n=%d: %d rows, upstream %d", n, len(got), len(want))
			}
			for r := range got {
				if string(boolsToBytes(got[r])) != string(boolsToBytes(want[r])) {
					t.Fatalf("n=%d level %d: row %d differs from upstream", n, level, r)
				}
			}
			compared++
		}
	}
	if compared == 0 {
		t.Fatal("no symbol compared")
	}
}

func TestFitPicksTheLargestModules(t *testing.T) {
	data := strings.Repeat("ZZA2csdghhkpgGDGYAAGYABGYACGJ2CGHYY", 12)
	m, mod, err := Fit(data, 5, 54.8, 43)
	if err != nil {
		t.Fatal(err)
	}
	cols, rows := len(m[0]), len(m)
	if (cols-1)%17 != 0 || rows < minRows || rows > maxRows {
		t.Fatalf("odd symbol %dx%d", cols, rows)
	}
	if float64(cols)*mod > 54.8+1e-9 || float64(RowHeight*rows)*mod > 43+1e-9 {
		t.Errorf("%dx%d modules of %.3f overflow the box", cols, rows, mod)
	}
	// Upstream's 3:1 shape in the same box
	up := upstreamMatrix(t, data, 5)
	upMod := min(54.8/float64(len(up[0])), 43/float64(RowHeight*len(up)))
	if mod < upMod {
		t.Errorf("modules %.3fmm, upstream's shape gives %.3fmm", mod, upMod)
	}
	t.Logf("%d columns x %d rows, %.3fmm modules (upstream %.3fmm)", (cols-69)/17, rows, mod, upMod)
}

func boolsToBytes(b []bool) []byte {
	out := make([]byte, len(b))
	for i, v := range b {
		if v {
			out[i] = '1'
		} else {
			out[i] = '0'
		}
	}
	return out
}
