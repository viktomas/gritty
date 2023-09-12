package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"gioui.org/io/key"
	"github.com/creack/pty"
)

type Controller struct {
	buffer *Buffer
	ptmx   *os.File
	mu     sync.RWMutex
	render <-chan struct{}
	in     chan []byte
	done   chan struct{}
}

func (c *Controller) Started() bool {
	return c.buffer != nil
}

func (c *Controller) Start(shell string, cols, rows int) error {
	cmd := exec.Command(shell)
	cmd.Env = append(cmd.Env, "TERM=vt100")
	c.buffer = NewBuffer(cols, rows)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return fmt.Errorf("failed to start PTY %w", err)
	}
	render := make(chan struct{})
	c.render = render
	c.ptmx = ptmx
	c.done = make(chan struct{})
	ops := processPTY(c.ptmx)
	go func() {
		for op := range ops {
			c.handleOp(op)
		}
		close(c.done)
	}()
	return nil

}

func (c *Controller) Resize(cols, rows int) {
	c.mu.Lock()
	c.buffer.Resize(BufferSize{cols: cols, rows: rows})
	c.mu.Unlock()
	pty.Setsize(c.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})

}

func (c *Controller) KeyPressed(name string, mod key.Modifiers) {
	_, err := c.ptmx.Write(keyToBytes(name, mod))
	if err != nil {
		log.Fatalf("writing key into PTY failed with error: %v", err)
		return
	}
}

func (c *Controller) Runes() []paintedRune {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffer.Runes()
}

func (c *Controller) Render() <-chan struct{} {
	return c.render
}

func (c *Controller) executeOp(r rune) {

	switch r {
	case asciiHT:
		c.buffer.Tab()
	case asciiBS:
		c.buffer.Backspace()
	case asciiCR:
		c.buffer.CR()
	case asciiLF:
		c.buffer.LF()
	case 0x8d:
		c.buffer.ReverseIndex()
	default:
		fmt.Printf("Unknown control character 0x%x", r)
	}
}

func (c *Controller) handleOp(op operation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logDebug("%v\n", op)
	switch op.t {
	case iexecute:
		c.executeOp(op.r)
	case iprint:
		c.buffer.WriteRune(op.r)
	case icsi:
		fn := translateCSI(op)
		if fn != nil {
			fn(c.buffer, c.ptmx)
		}
	case iosc:
		fmt.Println("unhandled OSC instruction: ", op)
	case iesc:
		if op.r >= '@' && op.r <= '_' {
			c.executeOp(op.r + 0x40)
		} else {
			fmt.Println("Unknown ESC op: ", op)
		}
	default:
		fmt.Printf("unhandled op type %v\n", op)
	}

}

func processPTY(ptmx *os.File) <-chan operation {
	out := make(chan operation)
	buf := make([]byte, 1024)
	decoder := NewDecoder()
	go func() {
		defer func() {
			close(out)
			ptmx.Close()
		}()
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				// if the error is io.EOF, then the PTY got closed and that most likely means that the shell exited
				if !errors.Is(io.EOF, err) {
					log.Printf("exiting copyAndHandleControlSequences because reader error %v", err)
				}
				return
			}
			for _, op := range decoder.Parse(buf[:n]) {
				out <- op
			}
		}
	}()
	return out
}
