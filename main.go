package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
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
	return &Screen{}
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

func handleControlSequences(screen *Screen, p []byte) {
	for i := 0; i < len(p); i++ {
		b := p[i]
		switch {
		case b == '\n':
			screen.cursorX = 0
			screen.cursorY++
		case b == 27 && i+1 < len(p) && p[i+1] == '[': // Escape sequence starts with \x1B[
			// Implement more sequences as needed.
			// For instance: "\x1B[2J" clears the screen.
			// But for now, we'll skip the sequence for simplicity.
			i += 2
		default:
			screen.WriteRune(rune(b))
		}
	}
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
func test(screen *Screen) error {
	defaultShell, exists := os.LookupEnv("SHELL")
	if !exists {
		log.Fatal("could not find default shell from $SHELL")
	}

	c := exec.Command(defaultShell)

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.
	go copyAndHandleControlSequences(screen, ptmx)
	// Wait for 2 seconds.
	time.Sleep(2 * time.Second)

	// Send CTRL+D control character.
	_, _ = ptmx.Write([]byte{0x04})

	return nil
}

func loop(w *app.Window) error {

	th := material.NewTheme()
	var ops op.Ops
	var sel widget.Selectable

	screen := NewScreen()
	test(screen)

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
			inset := layout.UniformInset(5)
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Label(th, 16, screen.String())
					l.Font.Typeface = font.Typeface("Go Mono")
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
