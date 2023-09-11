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

type Screen struct {
	lines           [][]paintedRune
	alternateLines  [][]paintedRune
	bufferType      bufferType
	size            ScreenSize
	cursor          cursor
	savedCursor     cursor
	scrollAreaStart int
	scrollAreaEnd   int
	brush
}

type ScreenSize struct {
	rows int
	cols int
}

func NewScreen(cols, rows int) *Screen {
	size := ScreenSize{rows: rows, cols: cols}
	screen := &Screen{size: size}
	screen.ResetBrush()
	screen.lines = screen.makeNewLines(size)
	screen.alternateLines = screen.makeNewLines(size)
	screen.resetScrollArea()
	return screen
}

func (s *Screen) CR() {
	s.cursor.x = 0
}
func (s *Screen) LF() {
	s.cursor.y++
	if s.cursor.y >= s.scrollAreaEnd {
		s.scrollUp()
		s.cursor.y--
	}
}

func (s *Screen) scrollUp() {
	for i := s.scrollAreaStart + 1; i < s.scrollAreaEnd; i++ {
		s.lines[i-1] = s.lines[i]
	}
	s.lines[s.scrollAreaEnd-1] = s.newLine(s.size.cols)
}

func (s *Screen) ResetBrush() {
	s.brush = brush{fg: defaultFG, bg: defaultBG}
}

func (s *Screen) newLine(cols int) []paintedRune {
	line := make([]paintedRune, cols)
	for c := range line {
		line[c] = s.makeRune(' ')
	}
	return line
}

func (s *Screen) SetScrollArea(start, end int) {
	s.scrollAreaStart = start
	s.scrollAreaEnd = end
	s.cursor = cursor{x: 0, y: start}
}

func (s *Screen) resetScrollArea() {
	s.scrollAreaStart = 0
	s.scrollAreaEnd = len(s.lines)

}

func (s *Screen) makeRune(r rune) paintedRune {
	return paintedRune{
		r:  r,
		fg: s.brush.fg,
		bg: s.brush.bg,
	}
}

func (s *Screen) WriteRune(r rune) {
	s.lines[s.cursor.y][s.cursor.x] = s.makeRune(r)
	s.cursor.x++
	if s.cursor.x >= s.size.cols {
		//soft wrap
		s.CR()
		s.LF()
	}
}

func (s *Screen) Runes() []paintedRune {
	out := make([]paintedRune, 0, s.size.rows*s.size.cols) // extra space for new lines
	for ri, r := range s.lines {
		for ci, c := range r {
			// invert cursor every odd interval
			if (s.cursor.x == ci) && s.cursor.y == ri && shouldInvertCursor() {
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
		out = append(out, s.makeRune('\n'))
	}

	return out
}

func (s *Screen) String() string {
	var sb strings.Builder
	for _, r := range s.lines {
		for _, c := range r {
			sb.WriteRune(c.r)
		}
		sb.WriteRune('\n')
	}
	return sb.String()
}

func (s *Screen) ClearFull() {
	for r := range s.lines {
		for c := range s.lines[r] {
			s.lines[r][c] = s.makeRune(' ')
		}
	}
	s.cursor.x, s.cursor.y = 0, s.scrollAreaStart
}

func (s *Screen) CleanForward() {
	currentLineToClean := s.lines[s.cursor.y][s.cursor.x:]
	for i := range currentLineToClean {
		currentLineToClean[i] = s.makeRune(' ')
	}
	for r := s.cursor.y + 1; r < len(s.lines); r++ {
		for c := range s.lines[r] {
			s.lines[r][c] = s.makeRune(' ')
		}
	}
}

func (s *Screen) CleanBackward() {
	currentLineToClean := s.lines[s.cursor.y][:s.cursor.x+1]
	for i := range currentLineToClean {
		currentLineToClean[i] = s.makeRune(' ')
	}
	for r := 0; r < s.cursor.y-1; r++ {
		for c := range s.lines[r] {
			s.lines[r][c] = s.makeRune(' ')
		}
	}
}

func (s *Screen) Tab() {
	newX := (s.cursor.x / 8 * 8) + 8
	if newX < s.size.cols {
		s.cursor.x = newX
	} else {
		s.cursor.x = s.size.cols - 1 // if the tab can't be fully added, lets move the cursor to the last column
	}
}

func (s *Screen) makeNewLines(size ScreenSize) [][]paintedRune {
	newLines := make([][]paintedRune, size.rows)
	for r := range newLines {
		newLines[r] = s.newLine(size.cols)
	}
	return newLines
}

func shouldInvertCursor() bool {
	currentTime := time.Now()
	return (currentTime.UnixNano()/int64(time.Millisecond)/500)%2 == 0
}

// Resize changes ensures that the dimensions are rows x cols
// returns true if the dimensions changed, otherwise returns false
func (s *Screen) Resize(size ScreenSize) bool {
	// TODO maybe I should use the size of the lines slice for this comparison
	if s.size.rows == size.rows && s.size.cols == size.cols {
		fmt.Println("ignoring resize")
		return false
	}
	s.size = size
	s.lines = s.makeNewLines(size)
	s.resetScrollArea()
	fmt.Printf("screen resized rows: %v, cols: %v\n", s.size.rows, s.size.cols)
	return true
}

func (s *Screen) Backspace() {
	x := s.cursor.x
	if x == 0 {
		return
	}
	// s.lines[y][x-1] = s.makeRune(' ')
	s.cursor.x = x - 1
}

func (s *Screen) MoveCursor(dx, dy int) {
	s.cursor.x += dx
	s.cursor.y += dy

	if s.cursor.x < 0 {
		s.cursor.x = 0
	} else if s.cursor.x >= s.size.cols {
		s.cursor.x = s.size.cols - 1
	}

	if s.cursor.y < 0 {
		s.cursor.y = 0
	} else if s.cursor.y >= s.size.rows {
		s.cursor.y = s.size.rows - 1
	}
}

func (s *Screen) SaveCursor() {
	s.savedCursor = s.cursor
}

func (s *Screen) SwitchToAlternateBuffer() {
	if s.bufferType == bufSecondary {
		return
	}
	primaryLines := s.lines
	s.lines = s.alternateLines
	s.alternateLines = primaryLines
	s.bufferType = bufSecondary
	s.ClearFull()
}
func (s *Screen) AdjustToNewSize() {
	// TODO the screen size might have changed between buffer

	oldSize := s.size
	s.size = ScreenSize{
		rows: len(s.lines),
		cols: len(s.lines[0]),
	}
	s.Resize(oldSize)
}

func (s *Screen) SwitchToPrimaryBuffer() {
	if s.bufferType == bufPrimary {
		return
	}
	alternateLines := s.lines
	s.lines = s.alternateLines
	s.alternateLines = alternateLines
	s.bufferType = bufPrimary
}

func (s *Screen) RestoreCursor() {
	s.cursor = s.savedCursor
}

func (s *Screen) ReverseIndex() {
	if s.cursor.y == s.scrollAreaStart {
		s.scrollDown(1)
	} else {
		s.cursor.y = s.cursor.y - 1
	}
}

func (s *Screen) scrollDown(lines int) {
	for i := s.scrollAreaEnd - lines - 1; i >= s.scrollAreaStart; i-- {
		s.lines[i+lines] = s.lines[i]
	}
	for i := s.scrollAreaStart; i < s.scrollAreaStart+lines; i++ {
		s.lines[i] = s.newLine(s.size.cols)
	}
}

// LineOp is function that can change line content and cursor column position
type LineOp func(line []paintedRune, cursorCol int) int

func (s *Screen) LineOp(lo LineOp) {
	newCol := lo(s.lines[s.cursor.y], s.cursor.x)
	s.cursor.x = newCol
}
