package main

import (
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"os/exec"
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
		fmt.Println("received op: ", op)
		switch op.t {
		case iexecute:
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
			screen.WriteRune(op.r)
		case icsi:
			switch op.r {
			case 'A':
				dy := 1
				if len(op.params) == 1 {
					dy = op.params[0]
				}
				screen.MoveCursor(0, -dy)
			case 'B':
				dy := 1
				if len(op.params) == 1 {
					dy = op.params[0]
				}
				screen.MoveCursor(0, dy)
			case 'C':
				dx := 1
				if len(op.params) == 1 {
					dx = op.params[0]
				}
				screen.MoveCursor(dx, 0)
			case 'D':
				dx := 1
				if len(op.params) == 1 {
					dx = op.params[0]
				}
				screen.MoveCursor(-dx, 0)
			case 'J':
				screen.Clear()
			case 'h':
				if len(op.params) == 1 && op.params[0] == 1049 && op.intermediate == "?" {
					screen.SaveCursor()
					screen.SwitchToAlternateBuffer()
					screen.AdjustToNewSize()
				}
			case 'l':
				if len(op.params) == 1 && op.params[0] == 1049 && op.intermediate == "?" {
					screen.SwitchToPrimaryBuffer()
					screen.RestoreCursor()
					screen.AdjustToNewSize()
				}
			}
			// received op:  CSI: fc: "u", params: [1 1], inter: =
			// received op:  CSI: fc: "h", params: [1], inter: ?
			// received op:  CSI: fc: "h", params: [2004], inter: ?
			// received op:  CSI: fc: "r", params: [1 31], inter:
			// received op:  CSI: fc: "m", params: [27], inter:
			// received op:  CSI: fc: "m", params: [24], inter:
			// received op:  CSI: fc: "m", params: [23], inter:
			// received op:  CSI: fc: "m", params: [0], inter:
			// received op:  CSI: fc: "H", params: [], inter:
			// received op:  CSI: fc: "J", params: [2], inter:
			// received op:  CSI: fc: "l", params: [25], inter: ?
			// received op:  CSI: fc: "H", params: [31 1], inter:
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

func loop(w *app.Window) error {

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	var ops op.Ops
	var sel widget.Selectable

	screen := NewScreen(80, 25)
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
					// l := material.Label(th, 16, generateTestContent(windowSize.rows, windowSize.cols))
					l.Font.Typeface = font.Typeface(monoTypeface)
					l.Truncator = ""
					l.State = &sel
					return inset.Layout(gtx, l.Layout)
				}),
			)
			e.Frame(gtx.Ops)
		}
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
	cols := div(fixed.I(windowSize.X-20), glyphWidth).Floor()
	rows := div(fixed.I(windowSize.Y-20), glyphHeight).Floor()
	return ScreenSize{rows: rows, cols: cols}
}
