package main

import (
	"fmt"
	"strings"
)

type cursor struct {
	x, y int
}

type bufferType int

const (
	bufPrimary = iota
	bufSecondary
)

type Screen struct {
	lines          [][]rune
	alternateLines [][]rune
	bufferType     bufferType
	size           ScreenSize
	cursor         cursor
	savedCursor    cursor
}

type ScreenSize struct {
	rows int
	cols int
}

func NewScreen(cols, rows int) *Screen {
	size := ScreenSize{rows: rows, cols: cols}
	screen := &Screen{size: size}
	screen.lines = makeNewLines(size)
	screen.alternateLines = makeNewLines(size)
	return screen
}

func (s *Screen) CR() {
	s.cursor.x = 0
}
func (s *Screen) LF() {
	s.cursor.y++
	if s.cursor.y >= s.size.rows {
		s.ScrollUp()
		s.cursor.y--
	}
}

func (s *Screen) ScrollUp() {
	for i := 1; i < len(s.lines); i++ {
		s.lines[i-1] = s.lines[i]
	}
	s.lines[len(s.lines)-1] = newLine(s.size.cols)
}

func newLine(cols int) []rune {
	line := make([]rune, cols)
	for c := range line {
		line[c] = ' '
	}
	return line
}

func (s *Screen) WriteRune(r rune) {
	s.lines[s.cursor.y][s.cursor.x] = r
	s.cursor.x++
	if s.cursor.x >= s.size.cols {
		//soft wrap
		s.CR()
		s.LF()
	}
}

func (s *Screen) String() string {
	lines := s.lines
	if len(s.lines) > s.size.rows {
		lines = s.lines[len(s.lines)-s.size.rows:]
	}

	var buf strings.Builder
	for _, line := range lines {
		runeLine := line
		if len(runeLine) > s.size.cols {
			buf.WriteString(string(runeLine[:s.size.cols]))
		} else {
			buf.WriteString(string(line))
		}
		buf.WriteString("\n")

	}
	return buf.String()
}

func (s *Screen) Clear() {
	for r := range s.lines {
		for c := range s.lines[r] {
			s.lines[r][c] = ' '
		}
	}
	s.cursor.x, s.cursor.y = 0, 0
}

func (s *Screen) Tab() {
	newX := (s.cursor.x / 8 * 8) + 8
	if newX < s.size.cols {
		s.cursor.x = newX
	} else {
		s.cursor.x = s.size.cols - 1 // if the tab can't be fully added, lets move the cursor to the last column
	}
}

func makeNewLines(size ScreenSize) [][]rune {
	newLines := make([][]rune, size.rows)
	for r := range newLines {
		newLines[r] = newLine(size.cols)
	}
	return newLines
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
	s.lines = makeNewLines(size)
	// s.lines = nil
	// for i := 0; i < size.rows; i++ {
	// 	s.lines = append(s.lines, make([]rune, size.cols))
	// }
	// for r := 0; r < oldSize.rows && r < size.rows; r++ {
	// 	for c := 0; c < oldSize.cols && c < size.cols; c++ {
	// 		s.lines[r][c] = oldLines[r][c]
	// 	}
	// }
	fmt.Printf("screen resized rows: %v, cols: %v\n", s.size.rows, s.size.cols)
	return true
}

func (s *Screen) Backspace() {
	x, y := s.cursor.x, s.cursor.y
	s.lines[y][x-1] = ' '
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
	s.Clear()
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

// LineOp is function that can change line content and cursor column position
type LineOp func(line []rune, cursorCol int) int

func (s *Screen) LineOp(lo LineOp) {
	newCol := lo(s.lines[s.cursor.y], s.cursor.x)
	s.cursor.x = newCol
}
