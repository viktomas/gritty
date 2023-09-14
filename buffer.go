package main

import (
	"fmt"
	"image/color"
	"strings"
	"time"
)

type Cursor struct {
	X, Y int
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
	bufAlternate
)

type Buffer struct {
	lines          [][]paintedRune
	alternateLines [][]paintedRune
	bufferType     bufferType
	size           BufferSize
	cursor         Cursor
	savedCursor    Cursor
	// nextWriteWraps indicates whether the next WriteRune will start on the new line.
	// if true, then before writing the next rune, we'll execute CR LF
	//
	// Without this field, we would have to CR LF straight after
	// writing the last rune in the row. But LF causes screen to scroll on the last line
	// which would make it impossible to write the last character on the screen
	nextWriteWraps  bool
	scrollAreaStart int
	scrollAreaEnd   int
	// originMode controls where the cursor can be placed with relationship to the scrolling region (margins)
	// false - the origin is at the upper-left character position on the screen. Line and column numbers are, therefore, independent of current margin settings. The cursor may be positioned outside the margins with a cursor position (CUP) or horizontal and vertical position (HVP) control.
	//
	// true - the origin is at the upper-left character position within the margins. Line and column numbers are therefore relative to the current margin settings. The cursor is not allowed to be positioned outside the margins.
	// described in https://vt100.net/docs/vt100-ug/chapter3.html
	originMode bool
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
	b.cursor.X = 0
}
func (b *Buffer) LF() {
	b.cursor.Y++
	if b.cursor.Y >= b.scrollAreaEnd {
		b.ScrollUp(1)
		b.cursor.Y--
	}
}

func (b *Buffer) ScrollUp(n int) {
	for i := b.scrollAreaStart + n; i < b.scrollAreaEnd; i++ {
		b.lines[i-n] = b.lines[i]
	}
	for i := b.scrollAreaEnd - n; i < b.scrollAreaEnd; i++ {
		b.lines[i] = b.newLine(b.size.cols)
	}
}

func (b *Buffer) ResetBrush() {
	b.brush = brush{fg: defaultFG, bg: defaultBG}
}

func (b *Buffer) newLine(cols int) []paintedRune {
	line := make([]paintedRune, cols)
	for c := range line {
		line[c] = b.MakeRune(' ')
	}
	return line
}

func (b *Buffer) SetScrollArea(start, end int) {
	b.scrollAreaStart = start
	b.scrollAreaEnd = end
	b.cursor = Cursor{X: 0, Y: start}
}

func (b *Buffer) resetScrollArea() {
	b.scrollAreaStart = 0
	b.scrollAreaEnd = len(b.lines)

}

func (b *Buffer) MakeRune(r rune) paintedRune {
	return paintedRune{
		r:  r,
		fg: b.brush.fg,
		bg: b.brush.bg,
	}
}

func (b *Buffer) WriteRune(r rune) {
	if b.nextWriteWraps == true {
		b.nextWriteWraps = false
		// soft wrap
		b.CR()
		b.LF()
	}
	b.lines[b.cursor.Y][b.cursor.X] = b.MakeRune(r)
	b.cursor.X++
	if b.cursor.X >= b.size.cols {
		b.nextWriteWraps = true
	}
}

func (b *Buffer) Runes() []paintedRune {
	out := make([]paintedRune, 0, b.size.rows*b.size.cols) // extra space for new lines
	for ri, r := range b.lines {
		for ci, c := range r {
			// invert cursor every odd interval
			if (b.cursor.X == ci) && b.cursor.Y == ri && shouldInvertCursor() {
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
		out = append(out, b.MakeRune('\n'))
	}

	return out
}

func (b *Buffer) Cursor() Cursor {
	return b.cursor

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

func (b *Buffer) ClearLines(start, end int) {
	// sanitize parameters
	s := clamp(start, 0, b.size.rows)
	e := clamp(end, 0, b.size.rows)

	toClean := b.lines[s:e]
	for r := range toClean {
		for c := range toClean[r] {
			toClean[r][c] = b.MakeRune(' ')
		}
	}
}

func (b *Buffer) ClearCurrentLine(start, end int) {
	// sanitize parameters
	s := clamp(start, 0, b.size.cols)
	e := clamp(end, 0, b.size.cols)

	currentLineToClean := b.lines[b.cursor.Y][s:e]
	for i := range currentLineToClean {
		currentLineToClean[i] = b.MakeRune(' ')
	}
}

func (b *Buffer) Tab() {
	newX := (b.cursor.X / 8 * 8) + 8
	if newX < b.size.cols {
		b.cursor.X = newX
	} else {
		b.cursor.X = b.size.cols - 1 // if the tab can't be fully added, lets move the cursor to the last column
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
	x := b.cursor.X
	if x == 0 {
		return
	}
	b.cursor.X = x - 1
}

func (b *Buffer) MoveCursorRelative(dx, dy int) {
	b.SetCursor(b.cursor.X+dx, b.cursor.Y+dy)
}

func (b *Buffer) SaveCursor() {
	b.savedCursor = b.cursor
}

func (b *Buffer) SwitchToAlternateBuffer() {
	if b.bufferType == bufAlternate {
		return
	}
	primaryLines := b.lines
	b.lines = b.alternateLines
	b.alternateLines = primaryLines
	b.bufferType = bufAlternate
	b.ClearLines(0, b.size.rows)
	b.SetCursor(0, 0)
}

// minY returns the index of the first row, this can be larger than 0 if the
// scroll area is reduced and the origin mode is enabled
func (b *Buffer) minY() int {
	if b.originMode {
		return b.scrollAreaStart
	}
	return 0
}

// maxY returns the index of the last row + 1, this can be smaller than b.size.rows if the
// scroll area is reduced and the origin mode is enabled
func (b *Buffer) maxY() int {
	if b.originMode {
		return b.scrollAreaEnd
	}
	return b.size.rows
}

func (b *Buffer) SetCursor(x, y int) {
	b.cursor = Cursor{
		X: clamp(x, 0, b.size.cols-1),
		Y: clamp(y, b.minY(), b.maxY()-1),
	}
	b.nextWriteWraps = false
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

// ReverseIndex Moves the active position to the same horizontal position on the preceding line. If the active position is at the top margin, a scroll down is performed. Format Effector
// [docs}(https://vt100.net/docs/vt100-ug/chapter3.html)
func (b *Buffer) ReverseIndex() {
	if b.cursor.Y == b.scrollAreaStart {
		b.scrollDown(1)
	} else {
		b.cursor.Y = b.cursor.Y - 1
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

func (b *Buffer) SetOriginMode(enabled bool) {
	b.originMode = true
	b.SetCursor(0, 0)
}

// clamp returns n if  fits into the range set by min and max, otherwise it
// returns min or max depending on the n being smaller or larger respectively
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
