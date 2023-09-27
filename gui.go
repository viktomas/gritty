package main

import (
	"fmt"
	"image"
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
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/viktomas/gritty/buffer"
	"github.com/viktomas/gritty/controller"
	"golang.org/x/image/math/fixed"
)

const monoTypeface = "go mono, monospaced"
const fontSize = 16

func StartGui(shell string, controller *controller.Controller) {
	go func() {
		w := app.NewWindow(app.Title("Gritty"))
		if err := loop(w, shell, controller); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window, sh string, controller *controller.Controller) error {

	shaper := text.NewShaper(text.WithCollection(gofont.Collection()))

	var ops op.Ops

	var location = f32.Pt(300, 300)

	var windowSize image.Point

	cursorBlinkTicker := time.NewTicker(500 * time.Millisecond)

	for {
		select {
		case <-controller.Done:
			return nil
		case <-cursorBlinkTicker.C:
			w.Invalidate()
		case e := <-w.Events():
			switch e := e.(type) {
			case system.DestroyEvent:
				return e.Err
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)
				// paint the whole window with the background color
				// FIXME: This is a temporary heck, the ideal solution would be to
				// shrink the window to the exact character grid after each resize
				// (with some debouncing)
				paint.ColorOp{Color: convertColor(buffer.DefaultBG)}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)

				if e.Size != windowSize {
					windowSize = e.Size // make sure this code doesn't run until we resized again
					bufferSize := getBufferSize(gtx, fontSize, e.Size, shaper)
					if !controller.Started() {

						var err error
						err = controller.Start(sh, bufferSize.Cols, bufferSize.Rows)
						if err != nil {
							log.Fatalf("can't initialize PTY controller %v", err)
						}
						go func() {
							for range controller.Render() {
								w.Invalidate()
							}
						}()
					} else {
						controller.Resize(bufferSize.Cols, bufferSize.Rows)
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
							PxPerEm: fixed.I(gtx.Sp(fontSize)),
						}
						shaper.LayoutString(params, "Hello")
						l := Label{
							// we don't put new lines at the end of the line
							// so we need the layout mechanism to use a policy
							// to maximize the number of characters printed per line
							WrapPolicy: text.WrapGraphemes,
						}
						font := font.Font{
							Typeface: font.Typeface(monoTypeface),
						}

						return l.Layout(gtx, shaper, font, fontSize, controller.Runes())
						// screenSize := getScreenSize(gtx, fontSize, e.Size, th)
						// return l.Layout(gtx, th.Shaper, font, fontSize, generateTestContent(screenSize.rows, screenSize.cols))
					}),
				)
				e.Frame(gtx.Ops)
			}
		}

	}
}

func generateTestContent(rows, cols int) []buffer.BrushedRune {
	var screen []buffer.BrushedRune
	for r := 0; r < rows; r++ {
		ch := fmt.Sprintf("%d", r)
		for c := 0; c < cols; c++ {
			r := rune(ch[len(ch)-1])
			if c%4 == 0 {
				r = ' '
			}
			br := buffer.BrushedRune{
				R: r,
			}
			if c == 0 {
				br = buffer.BrushedRune{
					R: br.R,
					Brush: buffer.Brush{
						Invert: true,
					},
				}
			}
			if c == cols-2 {
				br = buffer.BrushedRune{
					R: br.R,
					Brush: buffer.Brush{
						Bold: true,
					},
				}
			}
			screen = append(screen, br)
		}
	}
	return screen
}

// div divides two int26_6 numberes
func div(a, b fixed.Int26_6) fixed.Int26_6 {
	return (a * (1 << 6)) / b
}

func getBufferSize(gtx layout.Context, textSize unit.Sp, windowSize image.Point, sh *text.Shaper) buffer.BufferSize {
	params := text.Parameters{
		Font: font.Font{
			Typeface: font.Typeface(monoTypeface),
		},
		PxPerEm: fixed.I(gtx.Sp(fontSize)),
	}
	sh.Layout(params, strings.NewReader("A"))
	g, ok := sh.NextGlyph()
	if !ok {
		log.Println("ok is false for the next glyph")
	}
	glyphWidth := g.Advance
	glyphHeight := g.Ascent + g.Descent + 1<<6 // TODO find out why the line height is higher than the glyph
	cols := div(fixed.I(windowSize.X), glyphWidth).Floor()
	rows := div(fixed.I(windowSize.Y), glyphHeight).Floor()
	return buffer.BufferSize{Rows: rows, Cols: cols}
}
