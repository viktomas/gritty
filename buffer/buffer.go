package buffer

import (
	"fmt"
	"strings"
)

type Cursor struct {
	X, Y int
}

type Brush struct {
	FG     Color
	BG     Color
	Blink  bool
	Invert bool
	Bold   bool
}

type BrushedRune struct {
	R     rune
	Brush Brush
}

type bufferType int

const (
	bufPrimary = iota
	bufAlternate
)

type Buffer struct {
	lines          [][]BrushedRune
	alternateLines [][]BrushedRune
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
	brush      Brush
}

type BufferSize struct {
	Rows int
	Cols int
}

func New(cols, rows int) *Buffer {
	size := BufferSize{Rows: rows, Cols: cols}
	buffer := &Buffer{size: size}
	buffer.ResetBrush()
	buffer.lines = buffer.makeNewLines(size)
	buffer.alternateLines = buffer.makeNewLines(size)
	buffer.resetScrollArea()
	return buffer
}

func (b *Buffer) CR() {
	b.SetCursor(0, b.cursor.Y)
}
func (b *Buffer) LF() {
	b.nextWriteWraps = false
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
		b.lines[i] = b.newLine(b.size.Cols)
	}
}

// TODO maybe remove in favour of SetBrush(Brush{})
func (b *Buffer) ResetBrush() {
	b.brush = Brush{FG: DefaultFG, BG: DefaultBG}
}

func (b *Buffer) Brush() Brush {
	return b.brush
}

func (b *Buffer) SetBrush(br Brush) {
	b.brush = br
}

func (b *Buffer) newLine(cols int) []BrushedRune {
	line := make([]BrushedRune, cols)
	for c := range line {
		line[c] = b.MakeRune(' ')
	}
	return line
}

func (b *Buffer) SetScrollArea(start, end int) {
	b.scrollAreaStart = clamp(start, 0, b.size.Rows-1)
	b.scrollAreaEnd = clamp(end, b.scrollAreaStart+1, b.size.Rows)
	b.cursor = Cursor{X: 0, Y: b.scrollAreaStart}
}

func (b *Buffer) resetScrollArea() {
	b.scrollAreaStart = 0
	b.scrollAreaEnd = len(b.lines)

}

func (b *Buffer) MakeRune(r rune) BrushedRune {
	return BrushedRune{
		R:     r,
		Brush: b.brush,
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
	if b.cursor.X >= b.size.Cols {
		b.nextWriteWraps = true
	}
}

func (b *Buffer) Runes() []BrushedRune {
	out := make([]BrushedRune, 0, b.size.Rows*b.size.Cols) // extra space for new lines
	for ri, r := range b.lines {
		for ci, c := range r {
			// invert cursor every odd interval
			if (b.cursor.X == ci) && b.cursor.Y == ri {
				br := c.Brush
				br.Blink = true
				out = append(out, BrushedRune{
					R:     c.R,
					Brush: br,
				})
			} else {
				out = append(out, c)
			}
		}
	}

	return out
}

func (b *Buffer) Cursor() Cursor {
	return b.cursor
}

func (b *Buffer) Size() BufferSize {
	return b.size
}

func (b *Buffer) String() string {
	var sb strings.Builder
	for _, r := range b.lines {
		for _, c := range r {
			sb.WriteRune(c.R)
		}
		sb.WriteRune('\n')
	}
	return sb.String()
}

func (b *Buffer) ClearLines(start, end int) {
	// sanitize parameters
	s := clamp(start, 0, b.size.Rows)
	e := clamp(end, 0, b.size.Rows)

	toClean := b.lines[s:e]
	for r := range toClean {
		for c := range toClean[r] {
			toClean[r][c] = b.MakeRune(' ')
		}
	}
}

func (b *Buffer) ClearCurrentLine(start, end int) {
	// sanitize parameters
	s := clamp(start, 0, b.size.Cols)
	e := clamp(end, s, b.size.Cols)

	currentLineToClean := b.lines[b.cursor.Y][s:e]
	for i := range currentLineToClean {
		currentLineToClean[i] = b.MakeRune(' ')
	}
}

func (b *Buffer) Tab() {
	newX := (b.cursor.X / 8 * 8) + 8
	if newX < b.size.Cols {
		b.cursor.X = newX
	} else {
		b.cursor.X = b.size.Cols - 1 // if the tab can't be fully added, lets move the cursor to the last column
	}
}

func (b *Buffer) makeNewLines(size BufferSize) [][]BrushedRune {
	newLines := make([][]BrushedRune, size.Rows)
	for r := range newLines {
		newLines[r] = b.newLine(size.Cols)
	}
	return newLines
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
	fmt.Printf("buffer resized rows: %v, cols: %v\n", b.size.Rows, b.size.Cols)
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
	b.SetCursor(
		b.cursor.X+dx,
		clamp(b.cursor.Y+dy, b.scrollAreaStart, b.scrollAreaEnd),
	)
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
	b.ClearLines(0, b.size.Rows)
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
	return b.size.Rows
}

func (b *Buffer) SetCursor(x, y int) {
	b.cursor = Cursor{
		X: clamp(x, 0, b.size.Cols-1),
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
		// TODO this can be probably written nicer
		// I actually don't know what is the reverse index cursor up, should it
		// be like relative cursor movement (clamped by scrolling area) or should
		// U allow it to move outside of margins???
		b.cursor.Y = clamp(b.cursor.Y-1, 0, b.cursor.Y)
	}
}

func (b *Buffer) scrollDown(lines int) {
	for i := b.scrollAreaEnd - lines - 1; i >= b.scrollAreaStart; i-- {
		b.lines[i+lines] = b.lines[i]
	}
	for i := b.scrollAreaStart; i < b.scrollAreaStart+lines; i++ {
		b.lines[i] = b.newLine(b.size.Cols)
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
