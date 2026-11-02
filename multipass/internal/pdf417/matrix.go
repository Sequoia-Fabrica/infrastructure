// Package pdf417 is github.com/boombuler/barcode's PDF417 encoder (MIT; see
// LICENSE), with the symbol's shape chosen by the caller. The upstream
// encoder always aims for a wide 3:1 symbol of at most 30 rows; a badge needs
// whichever shape gives the largest modules in the space it has, since
// every fraction of a millimeter helps a laser burn bars a scanner can read.
//
// codewords.go, errorcorrection.go and highlevel.go are upstream's, as of
// v1.1.0, unchanged.
package pdf417

import (
	"errors"
	"fmt"
	"math"

	"github.com/boombuler/barcode/utils"
)

const (
	padding_codeword = 900
	minCols, maxCols = 1, 30
	minRows, maxRows = 3, 90
	maxCodewords     = 928
	// RowHeight is a row's height in modules: PDF417's minimum, which lets
	// the modules themselves be as large as possible.
	RowHeight = 3
)

// Fit encodes data at the given error correction level (0-8) in the shape
// whose modules come out largest in a box w by h, with rows RowHeight
// modules tall. It returns the modules, one []bool per row, and their size
// in the box's units.
func Fit(data string, securityLevel byte, w, h float64) ([][]bool, float64, error) {
	if securityLevel > 8 {
		return nil, 0, fmt.Errorf("pdf417: invalid security level %d", securityLevel)
	}
	words, err := highlevelEncode(data)
	if err != nil {
		return nil, 0, err
	}
	ec := securitylevel(securityLevel).ErrorCorrectionWordCount()

	best, bestMod := 0, 0.0
	for c := minCols; c <= maxCols; c++ {
		r := rowsFor(len(words), ec, c)
		if r < minRows || r > maxRows || r*c > maxCodewords {
			continue
		}
		mod := math.Min(w/float64(width(c)), h/float64(RowHeight*r))
		if mod > bestMod {
			best, bestMod = c, mod
		}
	}
	if best == 0 {
		return nil, 0, errors.New("pdf417: data does not fit")
	}
	m, err := matrix(words, securitylevel(securityLevel), best)
	return m, bestMod, err
}

// width is a symbol's width in modules: start, left indicator, data
// columns, right indicator, and the 18-module stop pattern.
func width(cols int) int { return (cols+4)*17 + 1 }

func rowsFor(dataWords, ecWords, cols int) int {
	total := dataWords + 1 + ecWords // +1 for the length descriptor
	return (total + cols - 1) / cols
}

func matrix(dataWords []int, sl securitylevel, cols int) ([][]bool, error) {
	rows := rowsFor(len(dataWords), sl.ErrorCorrectionWordCount(), cols)
	pad := rows*cols - (len(dataWords) + 1 + sl.ErrorCorrectionWordCount())
	for i := 0; i < pad; i++ {
		dataWords = append(dataWords, padding_codeword)
	}
	words := append([]int{len(dataWords) + 1}, dataWords...)
	words = append(words, sl.Compute(words)...)

	bl := new(utils.BitList)
	for r := 0; r < rows; r++ {
		table := r % 3
		bl.AddBits(start_word, 17)
		bl.AddBits(getCodeword(table, leftIndicator(r, rows, cols, sl)), 17)
		for _, w := range words[r*cols : (r+1)*cols] {
			bl.AddBits(getCodeword(table, w), 17)
		}
		bl.AddBits(getCodeword(table, rightIndicator(r, rows, cols, sl)), 17)
		bl.AddBits(stop_word, 18)
	}

	wd := width(cols)
	out := make([][]bool, rows)
	for r := range out {
		out[r] = make([]bool, wd)
		for c := range out[r] {
			out[r][c] = bl.GetBit(r*wd + c)
		}
	}
	return out, nil
}

// The row indicators tell a scanner the row count, column count and error
// correction level, a third of that in each row in turn.
func leftIndicator(row, rows, cols int, sl securitylevel) int {
	var x int
	switch row % 3 {
	case 0:
		x = (rows - 1) / 3
	case 1:
		x = int(sl)*3 + (rows-1)%3
	case 2:
		x = cols - 1
	}
	return 30*(row/3) + x
}

func rightIndicator(row, rows, cols int, sl securitylevel) int {
	var x int
	switch row % 3 {
	case 0:
		x = cols - 1
	case 1:
		x = (rows - 1) / 3
	case 2:
		x = int(sl)*3 + (rows-1)%3
	}
	return 30*(row/3) + x
}
