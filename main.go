package main

import (
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/creack/pty"
	"golang.org/x/image/math/fixed"
)

func fixedToFloat(i fixed.Int26_6) float64 {
	return float64(i) / 64.0
}

func div(a, b fixed.Int26_6) fixed.Int26_6 {
	return (a * (1 << 6)) / b
}

func main() {
	go func() {
		w := app.NewWindow()
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

type Screen struct {
	lines   [][]rune // characters on screen
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
	for i := 0; i < rows; i++ {
		screen.lines = append(screen.lines, make([]rune, cols))
	}
	screen.Clear()
	return screen
}

func (s *Screen) WriteRune(r rune) {
	if s.cursorX >= s.size.cols {
		s.cursorX = 0
		s.cursorY++
	}
	if s.cursorY >= s.size.rows {
		log.Printf("resetting screen\n %s", s.String())
		// Scroll or handle overflow. For simplicity, we're resetting here.
		s.cursorY = 0
	}
	s.lines[s.cursorY][s.cursorX] = r
	s.cursorX++
}

func (s *Screen) String() string {
	var buf strings.Builder
	for _, row := range s.lines {
		for _, r := range row {
			buf.WriteRune(r)
		}
		buf.WriteRune('\n')
	}
	return buf.String()
}

func (s *Screen) Clear() {
	for y := range s.lines {
		for x := range s.lines[y] {
			s.lines[y][x] = ' ' // replace with space
		}
	}
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
	oldSize := s.size
	oldLines := s.lines
	s.size = size
	s.lines = nil
	for i := 0; i < size.rows; i++ {
		s.lines = append(s.lines, make([]rune, size.cols))
	}
	for r := 0; r < oldSize.rows && r < size.rows; r++ {
		for c := 0; c < oldSize.cols && c < size.cols; c++ {
			s.lines[r][c] = oldLines[r][c]
		}
	}
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

func handleControlSequences(screen *Screen, p []byte) {
	fmt.Println("Handling control sequences.")
	for i := 0; i < len(p); i++ {
		b := p[i]
		fmt.Printf("0x%x,", b)
		switch {
		case b == asciiTAB:
			screen.Tab()
		case b == asciiBS:
			screen.Backspace()
		case b == '\n':
			// This really should be more complex. There is a tty setting `onlcr` that instructs tty to give me CR-LF for every LF sent to it
			// and I should somehow find out if this setting is enabled and parse CR-LF based on that
			// also, it could happen that I get the CR at the end of one buffer and LF at the start of other ¯\_(ツ)_/¯
			fmt.Println("encountered LF character -ignoring")
		case b == '\r' && i+1 < len(p) && p[i+1] == '\n':
			fmt.Println("encountered CR character")
			screen.cursorX = 0
			screen.cursorY++
		case b == 27 && i+1 < len(p) && p[i+1] == '[':
			// Move the index past the '[' character
			i += 2

			// Now let's read the integer parameter if available
			var numBuf []byte
			for ; i < len(p) && p[i] >= '0' && p[i] <= '9'; i++ {
				numBuf = append(numBuf, p[i])
			}

			// If no parameter is provided, assume it's 1
			n := 1
			if len(numBuf) > 0 {
				n, _ = strconv.Atoi(string(numBuf))
			}

			// Check the command character
			if i < len(p) {
				switch p[i] {
				case 'A':
					screen.MoveCursor(0, -n) // Move cursor up
				case 'B':
					screen.MoveCursor(0, n) // Move cursor down
				case 'C':
					screen.MoveCursor(n, 0) // Move cursor right
				case 'D':
					screen.MoveCursor(-n, 0) // Move cursor left
				case 'J':
					if n == 2 {
						screen.Clear()
					}
					// Note: We handle only the "clear entire screen" case here.
					// Other modes like "clear to end of screen" can be added similarly.
				}
			}
			// Graphic Left (GR) ascii area
		case b >= 0x20 && b < 0x7f:
			screen.WriteRune(rune(b))
		default:
			fmt.Printf("unknown non-printable character 0x%x\n", b)
		}
	}
	fmt.Println("\nFinished handling")
}

func copyAndHandleControlSequences(screen *Screen, src io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		handleControlSequences(screen, buf[:n])
	}
}

func loop(w *app.Window) error {

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	var ops op.Ops
	var sel widget.Selectable

	screen := NewScreen(25, 80)
	// defaultShell, exists := os.LookupEnv("SHELL")
	// if !exists {
	// 	log.Fatal("could not find default shell from $SHELL")
	// }
	defaultShell := "/bin/sh"

	c := exec.Command(defaultShell)

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.
	go copyAndHandleControlSequences(screen, ptmx)

	var location = f32.Pt(300, 300)
	// var arrowKeys = key.Set(strings.Join([]string{key.NameLeftArrow, key.NameUpArrow, key.NameRightArrow, key.NameDownArrow}, "|"))

	for {
		e := <-w.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)
			windowSize := getScreenSize(gtx, 16, e.Size, th)
			resized := screen.Resize(windowSize)
			if resized {
				pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(windowSize.rows), Cols: uint16(windowSize.cols)})
			}
			// keep the focus, since only one thing can
			key.FocusOp{Tag: &location}.Add(&ops)
			// register tag &location as reading input
			key.InputOp{
				Tag: &location,
				// Keys: arrowKeys,
			}.Add(&ops)

			// Capture and handle keyboard input
			for _, ev := range gtx.Events(&location) {
				if ke, ok := ev.(key.Event); ok {
					fmt.Println("key pressed", ke)
					if ke.State == key.Press {
						_, err := ptmx.Write(keyToBytes(ke.Name, ke.Modifiers))
						if err != nil {
							return err
						}

					}
				}
			}
			inset := layout.UniformInset(5)
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(th, 16, screen.String())
					l.Font.Typeface = font.Typeface("go mono, monospace")
					l.Truncator = ""
					l.State = &sel
					return inset.Layout(gtx, l.Layout)
				}),
			)
			e.Frame(gtx.Ops)
		}
	}
}

func keyToBytes(name string, mod key.Modifiers) []byte {
	if mod.Contain(key.ModCtrl) {
		switch name {
		case "C":
			return []byte{asciiETX} // return ETX (end of text, ^C)
		case "D":
			return []byte{asciiEOT} // return EOT (end of transmission)
		}
	}
	switch name {
	// Handle ANSI escape sequence for Enter key
	case key.NameReturn:
		return []byte("\r")
	case key.NameDeleteBackward:
		return []byte{asciiBS}
	case key.NameSpace:
		return []byte(" ")
	case key.NameEscape:
		return []byte{0x1B}
	case key.NameTab:
		return []byte{0x09}
	case key.NameUpArrow:
		return []byte{0x1b, '[', 'A'}
	case key.NameDownArrow:
		return []byte{0x1b, '[', 'B'}
	case key.NameRightArrow:
		return []byte{0x1b, '[', 'C'}
	case key.NameLeftArrow:
		return []byte{0x1b, '[', 'D'}
	default:
		// For normal characters, pass them through.
		var character string
		if mod.Contain(key.ModShift) {
			character = strings.ToUpper(name)
		} else {
			character = strings.ToLower(name)
		}
		return []byte(character)
	}
}

func generateTestContent(rows, cols int) string {
	makeRow := func(char, size int) string {
		ch := fmt.Sprintf("%d", char)
		var sb strings.Builder
		for i := 0; i < cols; i++ {
			sb.Write([]byte{ch[len(ch)-1]})
		}
		return sb.String()
	}
	var rb strings.Builder
	for i := 0; i < rows; i++ {
		rb.WriteString(makeRow(i, cols))
	}
	return rb.String()
}

func getScreenSize(gtx layout.Context, textSize unit.Sp, windowSize image.Point, th *material.Theme) ScreenSize {
	params := text.Parameters{
		Font: font.Font{
			Typeface: font.Typeface("go mono, monospace"),
		},
		PxPerEm: fixed.I(gtx.Sp(16)),
	}
	th.Shaper.Layout(params, strings.NewReader("A"))
	g, ok := th.Shaper.NextGlyph()
	if !ok {
		log.Println("ok is false for the next glyph")
	}
	glyphWidth := g.Advance
	glyphHeight := g.Ascent + g.Descent
	cols := div(fixed.I(windowSize.X-20), glyphWidth).Floor()
	rows := div(fixed.I(windowSize.Y-60), glyphHeight).Floor()
	return ScreenSize{rows: rows, cols: cols}

}
