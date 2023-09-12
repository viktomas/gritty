package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
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
	"golang.org/x/image/math/fixed"
)

const monoTypeface = "go mono, monospaced"

func logDebug(f string, vars ...any) {
	if os.Getenv("gritty_debug") != "" {
		fmt.Printf(f, vars...)
	}
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

func loop(w *app.Window) error {

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	var ops op.Ops
	defaultShell := "/bin/sh"

	var location = f32.Pt(300, 300)

	controller := &Controller{}

	var windowSize image.Point
	ticker := time.NewTicker(500 * time.Millisecond)

	for {
		select {
		case <-controller.done:
			return nil
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
					if !controller.Started() {

						var err error
						err = controller.Start(defaultShell, screenSize.cols, screenSize.rows)
						if err != nil {
							log.Fatalf("can't initialize PTY controller %v", err)
						}
						go func() {
							for range controller.Render() {
								w.Invalidate()
							}
						}()
					} else {
						controller.Resize(screenSize.cols, screenSize.rows)
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
							controller.KeyPressed(ke.Name, ke.Modifiers)
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

						return l.Layout(gtx, th.Shaper, font, 16, controller.Runes())
						// screenSize := getScreenSize(gtx, 16, e.Size, th)
						// return l.Layout(gtx, th.Shaper, font, 16, generateTestContent(screenSize.rows, screenSize.cols))
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
				pr = paintedRune{
					r:  rune(ch[len(ch)-1]),
					fg: color.NRGBA{A: 255, R: 255, G: 255, B: 255},
					bg: color.NRGBA{A: 255, R: 255},
				}
			}
			if c == cols-2 {
				pr = paintedRune{r: pr.r, fg: pr.bg, bg: pr.fg}
			}
			screen = append(screen, pr)
		}
	}
	return screen
}

// div divides two int26_6 numberes
func div(a, b fixed.Int26_6) fixed.Int26_6 {
	return (a * (1 << 6)) / b
}

func getScreenSize(gtx layout.Context, textSize unit.Sp, windowSize image.Point, th *material.Theme) BufferSize {
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
	return BufferSize{rows: rows, cols: cols}
}
