package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"gioui.org/io/key"
	"github.com/creack/pty"
)

type Controller struct {
	screen *Screen
	ptmx   *os.File
}

func (c *Controller) Started() bool {
	return c.screen != nil
}

func (c *Controller) Start(shell string, cols, rows int) (<-chan struct{}, error) {
	cmd := exec.Command(shell)
	cmd.Env = append(cmd.Env, "TERM=xterm")
	c.screen = NewScreen(cols, rows)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY %w", err)
	}
	invalidate := make(chan struct{})
	// TODO when should I close this? I need to have some channel coming in
	// defer ptmx.Close()
	c.ptmx = ptmx
	go copyAndHandleControlSequences(invalidate, c.screen, c.ptmx)
	return invalidate, nil

}

func (c *Controller) Resize(cols, rows int) {
	c.screen.Resize(ScreenSize{cols: cols, rows: rows})
	pty.Setsize(c.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})

}

func (c *Controller) KeyPressed(name string, mod key.Modifiers) {
	_, err := c.ptmx.Write(keyToBytes(name, mod))
	if err != nil {
		log.Printf("writing key into PTY failed with error: %v", err)
	}
}

func (c *Controller) Runes() []paintedRune {
	return c.screen.Runes()
}

func handleControlSequences(screen *Screen, p []byte) {
	for _, op := range NewDecoder().Parse(p) {
		switch op.t {
		case iexecute:
			fmt.Println("exec: ", op)
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
			fmt.Println("print: ", op)
			screen.WriteRune(op.r)
		case icsi:
			fn := translateCSI(op)
			if fn != nil {
				fn(screen)
			}
		}
	}
}

func copyAndHandleControlSequences(invalidate chan<- struct{}, screen *Screen, src io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		handleControlSequences(screen, buf[:n])
		invalidate <- struct{}{}
	}
}
