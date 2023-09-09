package main

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

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
	"gioui.org/widget/material"
	"github.com/creack/pty"
	"golang.org/x/image/math/fixed"
)

const monoTypeface = "go mono, monospaced"

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

func handleControlSequences(screen *Screen, p []byte) {
	for _, op := range NewDecoder().Parse(p) {
		switch op.t {
		case iexecute:
			// fmt.Println("exec: ", op)
			switch op.r {
			case asciiHT:
				screen.Tab()
			case asciiBS:
				screen.Backspace()
			case asciiCR:
				screen.CR()
			case asciiLF:
				screen.LF()
			default:
				fmt.Printf("Unknown control character 0x%x", op.r)
			}
		case iprint:
			// fmt.Println("print: ", op)
			screen.WriteRune(op.r)
		case icsi:
			fn := translateCSI(op)
			if fn != nil {
				fn(screen)
			}
		}
	}
}

func copyAndHandleControlSequences(w *app.Window, screen *Screen, src io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		handleControlSequences(screen, buf[:n])
		w.Invalidate()
	}
}

func loop(w *app.Window) error {

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	var ops op.Ops
	// var sel widget.Selectable

	var screen *Screen
	// defaultShell, exists := os.LookupEnv("SHELL")
	// if !exists {
	// 	log.Fatal("could not find default shell from $SHELL")
	// }
	defaultShell := "/bin/sh"

	c := exec.Command(defaultShell)
	c.Env = append(c.Env, "TERM=vt100")

	var location = f32.Pt(300, 300)

	var windowSize image.Point
	var ptmx *os.File
	ticker := time.NewTicker(500 * time.Millisecond)

	for {
		select {
		case <-ticker.C:
			w.Invalidate()
		case e := <-w.Events():
			switch e := e.(type) {
			case system.DestroyEvent:
				return e.Err
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)
				if e.Size != windowSize {
					windowSize = e.Size // make sure this code doesn't run until we resized again
					screenSize := getScreenSize(gtx, 16, e.Size, th)
					if screen == nil {
						screen = NewScreen(screenSize.cols, screenSize.rows)
						var err error
						// Start the command with a pty.
						ptmx, err = pty.StartWithSize(c, &pty.Winsize{Cols: uint16(screenSize.cols), Rows: uint16(screenSize.rows)})
						if err != nil {
							return err
						}
						// Make sure to close the pty at the end.
						defer func() {
							_ = ptmx.Close()
						}() // Best effort.
						go copyAndHandleControlSequences(w, screen, ptmx)
						// TODO constructor accepts only screenSize
					} else {
						// TODO doesn't have to return boolean
						screen.Resize(screenSize)
						pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(screenSize.rows), Cols: uint16(screenSize.cols)})
					}
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
								return fmt.Errorf("writing key into PTY failed with error: %w", err)
							}

						}
					}
				}
				// inset := layout.UniformInset(5)
				layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						params := text.Parameters{
							Font: font.Font{
								Typeface: font.Typeface(monoTypeface),
							},
							PxPerEm: fixed.I(gtx.Sp(16)),
						}
						th.Shaper.LayoutString(params, "Hello")
						l := Label{}
						font := font.Font{
							Typeface: font.Typeface(monoTypeface),
						}

						return l.Layout(gtx, th.Shaper, font, 16, screen.Runes())
						// return l.Layout(gtx, th.Shaper, font, 16, generateTestContent(screen.size.rows, screen.size.cols))
					}),
				)
				e.Frame(gtx.Ops)
			}
		}

	}
}

func generateTestContent(rows, cols int) []paintedRune {
	var screen []paintedRune
	for r := 0; r < rows; r++ {
		ch := fmt.Sprintf("%d", r)
		for c := 0; c < cols; c++ {
			pr := paintedRune{
				r:  rune(ch[len(ch)-1]),
				fg: color.NRGBA{A: 255},
				bg: color.NRGBA{A: 255, R: 255, G: 255, B: 255},
			}
			if c == 0 {
				pr.bg = color.NRGBA{A: 255, R: 255}
			}
			if c == cols-1 {
				pr.bg = color.NRGBA{A: 255, B: 255}
			}
			screen = append(screen, pr)
		}
	}
	return screen
}

func div(a, b fixed.Int26_6) fixed.Int26_6 {
	return (a * (1 << 6)) / b
}

func getScreenSize(gtx layout.Context, textSize unit.Sp, windowSize image.Point, th *material.Theme) ScreenSize {
	params := text.Parameters{
		Font: font.Font{
			Typeface: font.Typeface(monoTypeface),
		},
		PxPerEm: fixed.I(gtx.Sp(16)),
	}
	th.Shaper.Layout(params, strings.NewReader("A"))
	g, ok := th.Shaper.NextGlyph()
	if !ok {
		log.Println("ok is false for the next glyph")
	}
	glyphWidth := g.Advance
	glyphHeight := g.Ascent + g.Descent + 1<<6 // TODO find out why the line height is higher than the glyph
	cols := div(fixed.I(windowSize.X), glyphWidth).Floor()
	rows := div(fixed.I(windowSize.Y), glyphHeight).Floor()
	return ScreenSize{rows: rows, cols: cols}
}
