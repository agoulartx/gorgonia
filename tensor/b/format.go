package tensorb

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/chewxy/gorgonia/tensor/types"
)

var fmtFlags = [...]rune{'+', '-', '#', ' ', '0'}

func (t *Tensor) Format(state fmt.State, c rune) {
	if t.IsScalar() {
		var formatBuf bytes.Buffer
		formatBuf.WriteRune('%')
		for _, flag := range fmtFlags {
			if state.Flag(int(flag)) {
				formatBuf.WriteRune(flag)
			}
		}
		if width, ok := state.Width(); ok {
			formatBuf.WriteString(strconv.Itoa(width))
		}
		if prec, ok := state.Precision(); ok {
			formatBuf.WriteRune('.')
			formatBuf.WriteString(strconv.Itoa(prec))
		}
		formatBuf.WriteRune(c)

		fmt.Fprintf(state, formatBuf.String(), t.data[0])
		return
	}

	var rows, cols int
	if t.IsVector() {
		rows = 1
		cols = t.Size()
	} else {
		rows = t.Shape()[t.Dims()-2]
		cols = t.Shape()[t.Dims()-1]
	}

	metadata := state.Flag('+')
	flat := state.Flag('-')
	extended := state.Flag('#')
	compress := c == 's'
	hElision := "... "
	vElision := ".\n.\n.\n"
	buf := make([]byte, 0, 10)

	width := 5

	pad := make([]byte, types.MaxInt(width, 2))
	for i := range pad {
		pad[i] = ' '
	}

	var printedCols, printedRows int

	switch {
	case flat && extended:
		printedCols = len(t.data)
	case flat && compress:
		printedCols = 5
		hElision = "⋯ "
	case flat:
		printedCols = 10
	case extended:
		printedCols = cols
		printedRows = rows
	case compress:
		printedCols = types.MinInt(cols, 4)
		printedRows = types.MinInt(rows, 4)
		hElision = "⋯ "
		vElision = "  ⋮  \n"
	default:
		printedCols = types.MinInt(cols, 8)
		printedRows = types.MinInt(rows, 8)
	}

	// start printing
	if metadata {
		var userFriendly string
		switch {
		case t.IsScalar():
			userFriendly = "Scalar"
		case t.IsVector():
			userFriendly = "Vector"
		case t.Dims() == 2:
			userFriendly = "Matrix"
		default:
			userFriendly = fmt.Sprintf("%d-Tensor", t.Dims())
		}
		fmt.Fprintf(state, "%s %v %v\n", userFriendly, t.Shape(), t.Strides())
	}

	if flat {
		fmt.Fprintf(state, "[")
		switch {
		case extended:
			for i, v := range t.data {
				buf = strconv.AppendBool(buf[:0], v)
				state.Write(buf)
				if i < len(t.data)-1 {
					state.Write(pad[:1])
				}
			}
		case t.viewOf != nil:
			it := types.NewFlatIterator(t.AP)
			var c, i int
			var err error
			for i, err = it.Next(); err == nil; i, err = it.Next() {
				buf = strconv.AppendBool(buf[:0], t.data[i])
				state.Write(buf)
				state.Write(pad[:1])

				c++
				if c >= printedCols {
					fmt.Fprintf(state, hElision)
					break
				}
			}
			if err != nil {
				if _, noop := err.(NoOpError); !noop {
					fmt.Fprintf(state, "ERROR ITERATING: %v", err)

				}
			}
		default:
			for i := 0; i < printedCols; i++ {
				buf = strconv.AppendBool(buf[:0], t.data[i])
				state.Write(buf)
				state.Write(pad[:1])

			}

			if printedCols < len(t.data) {
				fmt.Fprintf(state, hElision)
			}
		}
		fmt.Fprintf(state, "]")
		return
	}

	const (
		matFirstStart = "⎡"
		matFirstEnd   = "⎤\n"
		matLastStart  = "⎣"
		matLastEnd    = "⎦\n"
		rowStart      = "⎢"
		rowEnd        = "⎥\n"
		vecStart      = "["
		vecEnd        = "]"
		colVecStart   = "C["
		rowVecStart   = "R["
	)

	it := types.NewFlatIterator(t.AP)
	coord := it.Coord()
	firstRow := true
	firstVal := true
	var lastRow, lastCol int

	for next, err := it.Next(); err == nil; next, err = it.Next() {
		var col, row int
		row = lastRow
		col = lastCol
		if rows > printedRows && row > printedRows/2 && row < rows-printedRows/2 {
			continue
		}

		if firstVal {
			if firstRow {
				switch {
				case t.IsColVec():
					fmt.Fprintf(state, colVecStart)
				case t.IsRowVec():
					fmt.Fprintf(state, rowVecStart)
				case t.IsVector():
					fmt.Fprintf(state, vecStart)
				default:
					fmt.Fprintf(state, matFirstStart)
				}

			} else {
				var matLastRow bool
				if !t.IsVector() {
					matLastRow = coord[len(coord)-2] == rows-1
				}
				if matLastRow {
					fmt.Fprintf(state, matLastStart)
				} else {
					fmt.Fprintf(state, rowStart)
				}
			}
			firstVal = false
		}

		// actual printing of the value
		if cols <= printedCols || (col < printedCols/2 || (col >= cols-printedCols/2)) {
			v := t.data[next]
			buf = strconv.AppendBool(buf[:0], v)
			state.Write(pad[:width-len(buf)]) // prepad
			state.Write(buf)                  // write the number
			if col < cols-1 {                 // pad with a space
				state.Write(pad[:2])
			}
		} else if col == printedCols/2 {
			fmt.Fprintf(state, hElision)
		}

		// done printing

		// end of row checks
		if col == cols-1 {
			eom := row == rows-1
			switch {
			case t.IsVector():
				fmt.Fprintf(state, vecEnd)
				return
			case firstRow:
				fmt.Fprintf(state, matFirstEnd)
			case eom:
				fmt.Fprintf(state, matLastEnd)
				if t.IsMatrix() {
					return
				}

				// one newline for every dimension above 2
				for i := t.Dims(); i > 2; i-- {
					fmt.Fprintf(state, "\n")
				}

			default:
				fmt.Fprintf(state, rowEnd)
			}

			if firstRow {
				firstRow = false
			}

			if eom {
				firstRow = true
			}

			firstVal = true

			// figure out elision
			if rows > printedRows && row+1 == printedRows/2 {
				coord[len(coord)-2] = rows - (printedRows / 2) //ooh dangerous... modiftying the coordinates!
				fmt.Fprintf(state, vElision)
			}
		}

		// cleanup
		switch {
		case t.IsRowVec():
			lastRow = coord[len(coord)-2]
			lastCol = coord[len(coord)-1]
		case t.IsColVec():
			lastRow = coord[len(coord)-1]
			lastCol = coord[len(coord)-2]
		case t.IsVector():
			lastCol = coord[len(coord)-1]
		default:
			lastRow = coord[len(coord)-2]
			lastCol = coord[len(coord)-1]
		}
	}
}

func (t *Tensor) String() string {
	return fmt.Sprintf("%v", t)
}
