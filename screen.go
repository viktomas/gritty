package main

import (
	"fmt"
	"strings"
)

type Screen struct {
	lines   [][]rune
	size    ScreenSize
	cursorX int // cursor's X position
	cursorY int // cursor's Y position
}

type ScreenSize struct {
	rows int
	cols int
}

func NewScreen(rows, cols int) *Screen {
	screen := &Screen{size: ScreenSize{rows: rows, cols: cols}}
	screen.Clear()
	return screen
}

func (s *Screen) CR() {
	s.cursorX = 0
}
func (s *Screen) LF() {
	s.cursorY++
}

func (s *Screen) WriteRune(r rune) {
	for s.cursorY >= len(s.lines) {
		s.lines = append(s.lines, []rune{})
	}
	for s.cursorX >= len(s.lines[s.cursorY]) {
		s.lines[s.cursorY] = append(s.lines[s.cursorY], ' ')
	}
	s.lines[s.cursorY][s.cursorX] = r
	s.cursorX++
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
	s.lines = [][]rune{}
	s.cursorX, s.cursorY = 0, 0
}

func (s *Screen) Tab() {
	newX := (s.cursorX / 8 * 8) + 8
	if newX < s.size.cols {
		s.cursorX = newX
	} else {
		s.cursorX = s.size.cols - 1 // if the tab can't be fully added, lets move the cursor to the last column
	}
}

// Resize changes ensures that the dimensions are rows x cols
// returns true if the dimensions changed, otherwise returns false
func (s *Screen) Resize(size ScreenSize) bool {
	fmt.Printf("resizing screen : %+v\n", size)
	if s.size.rows == size.rows && s.size.cols == size.cols {
		fmt.Println("ignoring resize")
		return false
	}
	// oldSize := s.size
	// oldLines := s.lines
	s.size = size
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
	x, y := s.cursorX, s.cursorY
	s.lines[y][x-1] = ' '
	s.cursorX = x - 1
}

func (s *Screen) MoveCursor(dx, dy int) {
	s.cursorX += dx
	s.cursorY += dy

	if s.cursorX < 0 {
		s.cursorX = 0
	} else if s.cursorX >= s.size.cols {
		s.cursorX = s.size.cols - 1
	}

	if s.cursorY < 0 {
		s.cursorY = 0
	} else if s.cursorY >= s.size.rows {
		s.cursorY = s.size.rows - 1
	}
}
