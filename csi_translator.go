package main

import (
	"fmt"
	"io"
	"log"
	"slices"
)

// translateCSI will get a CSI (Control Sequence Introducer) sequence (operation) and enact it on the buffer
func translateCSI(op operation, b *Buffer, pty io.Writer) {
	if op.t != icsi {
		log.Printf("operation %v is not CSI but it was passed to CSI translator.\n", op)
		return
	}

	// handle sequences that have the intermediate character (mostly private sequences)
	if op.intermediate != "" {
		switch op.r {
		case 'c':
			if op.intermediate == ">" {
				// inspired by https://github.com/liamg/darktile/blob/159932ff3ecdc9f7d30ac026480587b84edb895b/internal/app/darktile/termutil/csi.go#L305
				// we are VT100
				// for DA2 we'll respond >0;0;0
				_, err := pty.Write([]byte("\x1b[>0;0;0c"))
				if err != nil {
					log.Printf("Error when writing device information to PTY: %v", err)
				}
			}

		case 'h':
			// DEC Private Mode Set (DECSET).
			// source https://invisible-island.net/xterm/ctlseqs/ctlseqs.html
			if op.intermediate == "?" {
				switch op.param(0, 0) {
				// Origin Mode (DECOM), VT100.
				case 6:
					b.SetOriginMode(true)
				// Save cursor as in DECSC, After saving the cursor, switch to the Alternate Screen Buffer,
				case 1049:
					b.SaveCursor()
					b.SwitchToAlternateBuffer()
				default:
					log.Println("unknown DEC Private mode set parameter: ", op)
				}
			}
		case 'l':
			if op.intermediate == "?" {
				switch op.param(0, 0) {
				// Normal Cursor Mode (DECOM)
				case 6:
					b.SetOriginMode(false)
				// Use Normal Screen Buffer and restore cursor as in DECRC
				case 1049:
					b.SwitchToPrimaryBuffer()
					b.RestoreCursor()
				default:
					log.Println("unknown DEC Private mode set parameter: ", op)
				}
			}
		default:
			fmt.Printf("unknown CSI sequence with intermediate char %v\n", op)
		}
		return
	}

	switch op.r {
	// CUU - Cursor up
	case 'A':
		dy := op.param(0, 1)
		b.MoveCursorRelative(0, -dy)
	case 'e':
		fallthrough
	// CUD - Cursor down
	case 'B':
		dy := op.param(0, 1)
		b.MoveCursorRelative(0, dy)
	case 'a': // a is also CUF
		fallthrough
	// CUF - cursor forward
	case 'C':
		dx := op.param(0, 1)
		b.MoveCursorRelative(dx, 0)
	// CUB - Cursor bacward
	case 'D':
		dx := op.param(0, 1)
		b.MoveCursorRelative(-dx, 0)
	case 'J':
		switch op.param(0, 0) {
		case 0:
			b.ClearCurrentLine(b.Cursor().X, b.size.cols)
			b.ClearLines(b.Cursor().Y+1, b.size.rows)
		case 1:
			b.ClearCurrentLine(0, b.Cursor().X+1)
			b.ClearLines(0, b.Cursor().Y-1)
		case 2:
			b.ClearLines(0, b.size.rows)
			b.SetCursor(0, 0)
		default:
			log.Println("unknown CSI [J parameter: ", op.params[0])
		}
	case 'K':
		switch op.param(0, 0) {
		case 0:
			b.ClearCurrentLine(b.Cursor().X, b.size.cols)
		case 1:
			b.ClearCurrentLine(0, b.Cursor().X+1)
		case 2:
			b.ClearCurrentLine(0, b.size.cols)
		default:
			log.Println("unknown CSI K parameter: ", op.params[0])
		}
	case 'f': // Horizontal and Vertical Position [row;column] (default = [1,1]) (HVP).
		fallthrough
	case 'H': // Cursor Position [row;column] (default = [1,1]) (CUP).
		b.SetCursor(op.param(1, 1)-1, op.param(0, 1)-1)
	case 'r':
		start := op.param(0, 1)
		end := op.param(1, b.size.rows)
		// the DECSTBM docs https://vt100.net/docs/vt510-rm/DECSTBM.html
		// say that the index you get starts with 1 (first line)
		// and ends with len(lines)-1 (last line)
		// but the scroll area takes the index of the first line (starts with 0)
		// and index (starting from zero) of the last line + 1
		b.SetScrollArea(start-1, end)
	case 's':
		b.SaveCursor()
	case 'u':
		b.RestoreCursor()
	case 'c':
		// inspired by https://github.com/liamg/darktile/blob/159932ff3ecdc9f7d30ac026480587b84edb895b/internal/app/darktile/termutil/csi.go#L305
		// we are VT100
		// for DA1 we'll respond ?1;2
		_, err := pty.Write([]byte("\x1b[?1;2c"))
		if err != nil {
			log.Printf("Error when writing device information to PTY: %v", err)
		}
		// SGR https://vt100.net/docs/vt510-rm/SGR.html
	case 'm':
		ps := op.param(0, 0)
		if !slices.Contains([]int{0, 1, 7, 27}, ps) {
			log.Printf("unknown SGR instruction %v\n", op)
			return
		}
		switch ps {
		case 0:
			b.ResetBrush()
		case 1:
			br := b.Brush()
			br.bold = true
			b.SetBrush(br)
		case 7:
			br := b.Brush()
			br.invert = true
			b.SetBrush(br)
		case 27:
			br := b.Brush()
			br.invert = false
			b.SetBrush(br)
		}
	default:
		log.Printf("Unknown CSI instruction %v", op)
	}
}
