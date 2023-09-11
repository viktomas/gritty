package main

import (
	"io"
	"log"
)

type screenOp func(*Screen, io.Writer)

func translateCSI(op operation) screenOp {
	if op.t != icsi {
		log.Printf("operation %v is not CSI but it was passed to CSI translator.\n", op)
		return nil
	}

	switch op.r {
	case 'A':
		dy := op.param(0, 1)
		return func(s *Screen, _ io.Writer) {
			s.MoveCursor(0, -dy)
		}
		// FIXME fix bug in https://github.com/asciinema/avt/blob/main/src/vt.rs#L548C37-L548C37, where it is implemented as up, but should be down based on
		// - http://xtermjs.org/docs/api/vtfeatures/
		// - https://invisible-island.net/xterm/ctlseqs/ctlseqs.html
	case 'e':
		fallthrough
	case 'B':
		dy := op.param(0, 1)
		return func(s *Screen, _ io.Writer) {
			s.MoveCursor(0, dy)
		}
	case 'a': // a is also CUF
		fallthrough
	// CUF
	case 'C':
		dx := op.param(0, 1)
		return func(s *Screen, _ io.Writer) {
			s.MoveCursor(dx, 0)
		}
	case 'D':
		dx := op.param(0, 1)
		return func(s *Screen, _ io.Writer) {
			s.MoveCursor(-dx, 0)
		}
	case 'J':
		return func(s *Screen, _ io.Writer) {
			switch op.param(0, 0) {
			case 0:
				s.CleanForward()
			case 1:
				s.CleanBackward()
			case 2:
				s.ClearFull()
			default:
				log.Println("unknown CSI [J parameter: ", op.params[0])
			}
		}
	case 'K':
		return func(s *Screen, _ io.Writer) {
			s.LineOp(func(line []paintedRune, cursorCol int) int {
				var toClear []paintedRune
				switch op.param(0, 0) {
				case 0:
					toClear = line[cursorCol:] // erase from cursor to the end of the line
				case 1:
					toClear = line[:cursorCol+1] // erase from cursor to the start of the line
				case 2:
					toClear = line // erase the whole line
				}
				for i := range toClear {
					toClear[i] = s.makeRune(' ')
				}
				return cursorCol
			})
		}
	case 'f': // Horizontal and Vertical Position [row;column] (default = [1,1]) (HVP).
		fallthrough
	case 'H': // Cursor Position [row;column] (default = [1,1]) (CUP).
		return func(s *Screen, _ io.Writer) {
			// FIXME: check bounds, don't use private fields
			s.cursor = cursor{y: op.param(0, 1) - 1, x: op.param(1, 1) - 1}
		}
	case 'r':
		return func(s *Screen, w io.Writer) {
			start := op.param(0, 1)
			end := op.param(1, len(s.lines))
			// the DECSTBM docs https://vt100.net/docs/vt510-rm/DECSTBM.html
			// say that the index you get starts with 1 (first line)
			// and ends with len(lines)-1 (last line)
			// but the scroll area takes the index of the first line (starts with 0)
			// and index (starting from zero) of the last line + 1
			s.SetScrollArea(start-1, end)
		}
	case 's':
		return func(s *Screen, _ io.Writer) {
			s.SaveCursor()
		}
	case 'u':
		if op.intermediate == "" {
			return func(s *Screen, _ io.Writer) {
				s.RestoreCursor()
			}
		}
	case 'h':
		if len(op.params) == 1 && op.params[0] == 1049 && op.intermediate == "?" {
			return func(s *Screen, _ io.Writer) {
				s.SaveCursor()
				s.SwitchToAlternateBuffer()
				s.AdjustToNewSize()
			}
		}
	case 'l':
		if len(op.params) == 1 && op.params[0] == 1049 && op.intermediate == "?" {
			return func(s *Screen, _ io.Writer) {
				s.SwitchToPrimaryBuffer()
				s.RestoreCursor()
				s.AdjustToNewSize()
			}
		}
	case 'c':
		return func(s *Screen, pty io.Writer) {
			// inspired by https://github.com/liamg/darktile/blob/159932ff3ecdc9f7d30ac026480587b84edb895b/internal/app/darktile/termutil/csi.go#L305
			// we are VT100
			// for DA1 we'll respond ?1;2
			// for DA2 we'll respond >0;0;0
			response := "?1;2"
			if op.intermediate == ">" {
				response = ">0;0;0"
			}

			// write response to source pty
			_, err := pty.Write([]byte("\x1b[" + response + "c"))
			if err != nil {
				log.Printf("Error when writing device information to PTY: %v", err)
			}
		}
	}
	log.Printf("Unknown CSI instruction %v", op)

	// CSI: fc: "m", params: [], inter:
	// CSI: fc: "o", params: [], inter:
	// CSI: fc: "r", params: [], inter:
	// CSI: fc: "o", params: [], inter:
	// CSI: fc: "n", params: [], inter:
	// CSI: fc: "l", params: [], inter:
	// CSI: fc: "i", params: [], inter:
	// CSI: fc: "n", params: [], inter:
	return nil
}
