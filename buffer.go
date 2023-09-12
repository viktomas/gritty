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
	bufAlternate
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
	nextWriteWraps  bool
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
		line[c] = b.MakeRune(' ')
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

func (b *Buffer) MakeRune(r rune) paintedRune {
	return paintedRune{
		r:  r,
		fg: b.brush.fg,
		bg: b.brush.bg,
	}
}

func (b *Buffer) WriteRune(r rune) {
	b.lines[b.cursor.y][b.cursor.x] = b.MakeRune(r)
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
		out = append(out, b.MakeRune('\n'))
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

	currentLineToClean := b.lines[b.cursor.y][s:e]
	for i := range currentLineToClean {
		currentLineToClean[i] = b.MakeRune(' ')
	}
}

func (b *Buffer) ClearFull() {
	b.ClearLines(0, b.size.rows)
	b.cursor.x, b.cursor.y = 0, b.scrollAreaStart
}

func (b *Buffer) CleanForward() {
	b.ClearCurrentLine(b.cursor.x, b.size.cols)
	b.ClearLines(b.cursor.y+1, b.size.rows)
}

func (b *Buffer) CleanBackward() {
	b.ClearCurrentLine(0, b.cursor.x+1)
	b.ClearLines(0, b.cursor.y-1)
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
	b.cursor.x = x - 1
}

func (b *Buffer) MoveCursorRelative(dx, dy int) {
	b.cursor.x = clamp(b.cursor.x+dx, 0, b.size.cols-1)
	b.cursor.y = clamp(b.cursor.y+dy, 0, b.size.rows-1)
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
