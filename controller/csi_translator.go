package controller

import (
	"fmt"
	"io"
	"log"
	"slices"

	"github.com/viktomas/gritty/buffer"
	"github.com/viktomas/gritty/parser"
)

// translateCSI will get a CSI (Control Sequence Introducer) sequence (operation) and enact it on the buffer
func translateCSI(op parser.Operation, b *buffer.Buffer, pty io.Writer) {
	if op.T != parser.OpCSI {
		log.Printf("operation %v is not CSI but it was passed to CSI translator.\n", op)
		return
	}

	// handle sequences that have the intermediate character (mostly private sequences)
	if op.Intermediate != "" {
		switch op.R {
		case 'c':
			if op.Intermediate == ">" {
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
			if op.Intermediate == "?" {
				switch op.Param(0, 0) {
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
			if op.Intermediate == "?" {
				switch op.Param(0, 0) {
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

	switch op.R {
	// CUU - Cursor up
	case 'A':
		dy := op.Param(0, 1)
		b.MoveCursorRelative(0, -dy)
	case 'e':
		fallthrough
	// CUD - Cursor down
	case 'B':
		dy := op.Param(0, 1)
		b.MoveCursorRelative(0, dy)
	case 'a': // a is also CUF
		fallthrough
	// CUF - cursor forward
	case 'C':
		dx := op.Param(0, 1)
		b.MoveCursorRelative(dx, 0)
	// CUB - Cursor bacward
	case 'D':
		dx := op.Param(0, 1)
		b.MoveCursorRelative(-dx, 0)
	case 'J':
		switch op.Param(0, 0) {
		case 0:
			b.ClearCurrentLine(b.Cursor().X, b.Size().Cols)
			b.ClearLines(b.Cursor().Y+1, b.Size().Rows)
		case 1:
			b.ClearCurrentLine(0, b.Cursor().X+1)
			b.ClearLines(0, b.Cursor().Y-1)
		case 2:
			b.ClearLines(0, b.Size().Rows)
			b.SetCursor(0, 0)
		default:
			log.Println("unknown CSI [J parameter: ", op.Params[0])
		}
	case 'K':
		switch op.Param(0, 0) {
		case 0:
			b.ClearCurrentLine(b.Cursor().X, b.Size().Cols)
		case 1:
			b.ClearCurrentLine(0, b.Cursor().X+1)
		case 2:
			b.ClearCurrentLine(0, b.Size().Cols)
		default:
			log.Println("unknown CSI K parameter: ", op.Params[0])
		}
	case 'f': // Horizontal and Vertical Position [row;column] (default = [1,1]) (HVP).
		fallthrough
	case 'H': // Cursor Position [row;column] (default = [1,1]) (CUP).
		b.SetCursor(op.Param(1, 1)-1, op.Param(0, 1)-1)
	case 'r':
		start := op.Param(0, 1)
		end := op.Param(1, b.Size().Rows)
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
		ps := op.Param(0, 0)
		if !slices.Contains([]int{0, 1, 7, 27}, ps) {
			log.Printf("unknown SGR instruction %v\n", op)
			return
		}
		switch ps {
		case 0:
			b.ResetBrush()
		case 1:
			br := b.Brush()
			br.Bold = true
			b.SetBrush(br)
		case 7:
			br := b.Brush()
			br.Invert = true
			b.SetBrush(br)
		case 27:
			br := b.Brush()
			br.Invert = false
			b.SetBrush(br)
		}
	default:
		log.Printf("Unknown CSI instruction %v", op)
	}
}