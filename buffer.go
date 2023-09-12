package main

import (
	"fmt"
	"image/color"
	"strings"
	"time"
)

type cursor struct {
	x, y int
}

var (
	defaultFG = color.NRGBA{A: 255}
	defaultBG = color.NRGBA{A: 0xff, R: 0xff, G: 0xff, B: 0xff}
)

type brush struct {
	fg color.NRGBA
	bg color.NRGBA
}

type bufferType int

const (
	bufPrimary = iota
	bufSecondary
)

type Buffer struct {
	lines           [][]paintedRune
	alternateLines  [][]paintedRune
	bufferType      bufferType
	size            BufferSize
	cursor          cursor
	savedCursor     cursor
	scrollAreaStart int
	scrollAreaEnd   int
	brush
}

type BufferSize struct {
	rows int
	cols int
}

func NewBuffer(cols, rows int) *Buffer {
	size := BufferSize{rows: rows, cols: cols}
	buffer := &Buffer{size: size}
	buffer.ResetBrush()
	buffer.lines = buffer.makeNewLines(size)
	buffer.alternateLines = buffer.makeNewLines(size)
	buffer.resetScrollArea()
	return buffer
}

func (b *Buffer) CR() {
	b.cursor.x = 0
}
func (b *Buffer) LF() {
	b.cursor.y++
	if b.cursor.y >= b.scrollAreaEnd {
		b.scrollUp()
		b.cursor.y--
	}
}

func (b *Buffer) scrollUp() {
	for i := b.scrollAreaStart + 1; i < b.scrollAreaEnd; i++ {
		b.lines[i-1] = b.lines[i]
	}
	b.lines[b.scrollAreaEnd-1] = b.newLine(b.size.cols)
}

func (b *Buffer) ResetBrush() {
	b.brush = brush{fg: defaultFG, bg: defaultBG}
}

func (b *Buffer) newLine(cols int) []paintedRune {
	line := make([]paintedRune, cols)
	for c := range line {
		line[c] = b.makeRune(' ')
	}
	return line
}

func (b *Buffer) SetScrollArea(start, end int) {
	b.scrollAreaStart = start
	b.scrollAreaEnd = end
	b.cursor = cursor{x: 0, y: start}
}

func (b *Buffer) resetScrollArea() {
	b.scrollAreaStart = 0
	b.scrollAreaEnd = len(b.lines)

}

func (b *Buffer) makeRune(r rune) paintedRune {
	return paintedRune{
		r:  r,
		fg: b.brush.fg,
		bg: b.brush.bg,
	}
}

func (b *Buffer) WriteRune(r rune) {
	b.lines[b.cursor.y][b.cursor.x] = b.makeRune(r)
	b.cursor.x++
	if b.cursor.x >= b.size.cols {
		//soft wrap
		b.CR()
		b.LF()
	}
}

func (b *Buffer) Runes() []paintedRune {
	out := make([]paintedRune, 0, b.size.rows*b.size.cols) // extra space for new lines
	for ri, r := range b.lines {
		for ci, c := range r {
			// invert cursor every odd interval
			if (b.cursor.x == ci) && b.cursor.y == ri && shouldInvertCursor() {
				out = append(out, paintedRune{
					r:  c.r,
					fg: c.bg,
					bg: c.fg,
				})
			} else {
				out = append(out, c)
			}
		}
		// FIXME: why do I need the new lines here?
		out = append(out, b.makeRune('\n'))
	}

	return out
}

func (b *Buffer) String() string {
	var sb strings.Builder
	for _, r := range b.lines {
		for _, c := range r {
			sb.WriteRune(c.r)
		}
		sb.WriteRune('\n')
	}
	return sb.String()
}

func (b *Buffer) ClearFull() {
	for r := range b.lines {
		for c := range b.lines[r] {
			b.lines[r][c] = b.makeRune(' ')
		}
	}
	b.cursor.x, b.cursor.y = 0, b.scrollAreaStart
}

func (b *Buffer) CleanForward() {
	currentLineToClean := b.lines[b.cursor.y][b.cursor.x:]
	for i := range currentLineToClean {
		currentLineToClean[i] = b.makeRune(' ')
	}
	for r := b.cursor.y + 1; r < len(b.lines); r++ {
		for c := range b.lines[r] {
			b.lines[r][c] = b.makeRune(' ')
		}
	}
}

func (b *Buffer) CleanBackward() {
	currentLineToClean := b.lines[b.cursor.y][:b.cursor.x+1]
	for i := range currentLineToClean {
		currentLineToClean[i] = b.makeRune(' ')
	}
	for r := 0; r < b.cursor.y-1; r++ {
		for c := range b.lines[r] {
			b.lines[r][c] = b.makeRune(' ')
		}
	}
}

func (b *Buffer) Tab() {
	newX := (b.cursor.x / 8 * 8) + 8
	if newX < b.size.cols {
		b.cursor.x = newX
	} else {
		b.cursor.x = b.size.cols - 1 // if the tab can't be fully added, lets move the cursor to the last column
	}
}

func (b *Buffer) makeNewLines(size BufferSize) [][]paintedRune {
	newLines := make([][]paintedRune, size.rows)
	for r := range newLines {
		newLines[r] = b.newLine(size.cols)
	}
	return newLines
}

func shouldInvertCursor() bool {
	currentTime := time.Now()
	return (currentTime.UnixNano()/int64(time.Millisecond)/500)%2 == 0
}

// Resize changes ensures that the dimensions are rows x cols
// returns true if the dimensions changed, otherwise returns false
func (b *Buffer) Resize(size BufferSize) bool {
	if b.size == size {
		fmt.Println("ignoring resize")
		return false
	}
	b.size = size
	b.lines = b.makeNewLines(size)
	b.alternateLines = b.makeNewLines(size)
	b.resetScrollArea()
	fmt.Printf("buffer resized rows: %v, cols: %v\n", b.size.rows, b.size.cols)
	return true
}

func (b *Buffer) Backspace() {
	x := b.cursor.x
	if x == 0 {
		return
	}
	// s.lines[y][x-1] = s.makeRune(' ')
	b.cursor.x = x - 1
}

func (b *Buffer) MoveCursorRelative(dx, dy int) {
	b.cursor.x = minmax(b.cursor.x+dx, 0, b.size.cols-1)
	b.cursor.y = minmax(b.cursor.y+dy, 0, b.size.rows-1)
}

func (b *Buffer) SaveCursor() {
	b.savedCursor = b.cursor
}

func (b *Buffer) SwitchToAlternateBuffer() {
	if b.bufferType == bufSecondary {
		return
	}
	primaryLines := b.lines
	b.lines = b.alternateLines
	b.alternateLines = primaryLines
	b.bufferType = bufSecondary
	b.ClearFull()
}

func (b *Buffer) SwitchToPrimaryBuffer() {
	if b.bufferType == bufPrimary {
		return
	}
	alternateLines := b.lines
	b.lines = b.alternateLines
	b.alternateLines = alternateLines
	b.bufferType = bufPrimary
}

func (b *Buffer) RestoreCursor() {
	b.cursor = b.savedCursor
}

func (b *Buffer) ReverseIndex() {
	if b.cursor.y == b.scrollAreaStart {
		b.scrollDown(1)
	} else {
		b.cursor.y = b.cursor.y - 1
	}
}

func (b *Buffer) scrollDown(lines int) {
	for i := b.scrollAreaEnd - lines - 1; i >= b.scrollAreaStart; i-- {
		b.lines[i+lines] = b.lines[i]
	}
	for i := b.scrollAreaStart; i < b.scrollAreaStart+lines; i++ {
		b.lines[i] = b.newLine(b.size.cols)
	}
}

// LineOp is function that can change line content and cursor column position
type LineOp func(line []paintedRune, cursorCol int) int

func (b *Buffer) LineOp(lo LineOp) {
	newCol := lo(b.lines[b.cursor.y], b.cursor.x)
	b.cursor.x = newCol
}

// minmax returns n if it fits into the range set by min and max, otherwise it
// returns min or max depending on the n being smaller or larger respectively
func minmax(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
