package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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

func escapeControlSequences(p []byte) []byte {
	var buf bytes.Buffer
	for _, b := range p {
		if b < 32 || b == 127 {
			// This escapes the control character to a visible representation.
			// You can change the format to your liking.
			buf.WriteString(fmt.Sprintf("^%c", b+'@'))
		} else {
			buf.WriteByte(b)
		}
	}
	return buf.Bytes()
}

func copyAndEscapeControlSequences(dst io.Writer, src io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		escaped := escapeControlSequences(buf[:n])
		_, _ = dst.Write(escaped)
	}
}

func test(textBuf *bytes.Buffer) error {
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
	go copyAndEscapeControlSequences(textBuf, ptmx)
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
	var buf bytes.Buffer

	test(&buf)

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
					l := material.Label(th, 16, buf.String())
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
