package main

import (
	"fmt"
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
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/creack/pty"
)

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

const (
	width  = 80
	height = 25
)

type Screen struct {
	rows    [height][width]rune // characters on screen
	cursorX int                 // cursor's X position
	cursorY int                 // cursor's Y position
}

func NewScreen() *Screen {
	screen := &Screen{}
	screen.Clear()
	return screen
}

func (s *Screen) WriteRune(r rune) {
	if s.cursorX >= width {
		s.cursorX = 0
		s.cursorY++
	}
	if s.cursorY >= height {
		log.Printf("resetting screen\n %s", s.String())
		// Scroll or handle overflow. For simplicity, we're resetting here.
		s.cursorY = 0
	}
	s.rows[s.cursorY][s.cursorX] = r
	s.cursorX++
}

func (s *Screen) String() string {
	var buf strings.Builder
	for _, row := range s.rows {
		for _, r := range row {
			buf.WriteRune(r)
		}
		buf.WriteRune('\n')
	}
	return buf.String()
}

func (s *Screen) Clear() {
	for y := range s.rows {
		for x := range s.rows[y] {
			s.rows[y][x] = ' ' // replace with space
		}
	}
	s.cursorX, s.cursorY = 0, 0
}

func (s *Screen) Tab() {
	newX := (s.cursorX / 8 * 8) + 8
	if newX < width {
		s.cursorX = newX
	} else {
		s.cursorX = width - 1 // if the tab can't be fully added, lets move the cursor to the last column
	}
}

func (s *Screen) MoveCursor(dx, dy int) {
	s.cursorX += dx
	s.cursorY += dy

	if s.cursorX < 0 {
		s.cursorX = 0
	} else if s.cursorX >= width {
		s.cursorX = width - 1
	}

	if s.cursorY < 0 {
		s.cursorY = 0
	} else if s.cursorY >= height {
		s.cursorY = height - 1
	}
}

func handleControlSequences(screen *Screen, p []byte) {
	fmt.Println("Handling control sequences.")
	for i := 0; i < len(p); i++ {
		b := p[i]
		fmt.Printf("0x%x,", b)
		switch {
		case b == tab:
			screen.Tab()
		case b == '\n':
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

	screen := NewScreen()
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
	// // Wait for 2 seconds.
	// time.Sleep(2 * time.Second)
	//
	// // Send CTRL+D control character.
	// _, _ = ptmx.Write([]byte{0x04})

	var location = f32.Pt(300, 300)
	// var arrowKeys = key.Set(strings.Join([]string{key.NameLeftArrow, key.NameUpArrow, key.NameRightArrow, key.NameDownArrow}, "|"))

	const (
		minLinesRange = 1
		maxLinesRange = 25
	)
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)
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
						// Handle ANSI escape sequence for Enter key
						if ke.Name == key.NameReturn {
							_, err := ptmx.Write([]byte("\n")) // Line Feed
							if err != nil {
								return err
							}
						} else if ke.Name == key.NameSpace {
							_, err := ptmx.Write([]byte(" "))
							if err != nil {
								return err
							}
						} else {
							// For normal characters, pass them through.
							var character string
							if ke.Modifiers.Contain(key.ModShift) {
								character = strings.ToUpper(ke.Name)
							} else {
								character = strings.ToLower(ke.Name)
							}
							_, err := ptmx.Write([]byte(character))
							if err != nil {
								return err
							}
						}
					}
				}
			}
			inset := layout.UniformInset(5)
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(th, 16, screen.String())
					l.Font.Typeface = font.Typeface("go mono, monospaced")
					l.MaxLines = 24
					l.Truncator = ""
					l.State = &sel
					return inset.Layout(gtx, l.Layout)
				}),
			)
			e.Frame(gtx.Ops)
		}
	}
}
