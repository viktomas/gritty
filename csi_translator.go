package main

import (
	"image/color"
	"io"
	"log"
	"slices"
)

type bufferOP func(b *Buffer, pty io.Writer)

func translateCSI(op operation) bufferOP {
	if op.t != icsi {
		log.Printf("operation %v is not CSI but it was passed to CSI translator.\n", op)
		return nil
	}

	switch op.r {
	case 'A':
		dy := op.param(0, 1)
		return func(s *Buffer, _ io.Writer) {
			s.MoveCursorRelative(0, -dy)
		}
		// FIXME fix bug in https://github.com/asciinema/avt/blob/main/src/vt.rs#L548C37-L548C37, where it is implemented as up, but should be down based on
		// - http://xtermjs.org/docs/api/vtfeatures/
		// - https://invisible-island.net/xterm/ctlseqs/ctlseqs.html
	case 'e':
		fallthrough
	case 'B':
		dy := op.param(0, 1)
		return func(s *Buffer, _ io.Writer) {
			s.MoveCursorRelative(0, dy)
		}
	case 'a': // a is also CUF
		fallthrough
	// CUF
	case 'C':
		dx := op.param(0, 1)
		return func(s *Buffer, _ io.Writer) {
			s.MoveCursorRelative(dx, 0)
		}
	case 'D':
		dx := op.param(0, 1)
		return func(s *Buffer, _ io.Writer) {
			s.MoveCursorRelative(-dx, 0)
		}
	case 'J':
		return func(s *Buffer, _ io.Writer) {
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
		return func(s *Buffer, _ io.Writer) {
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
		return func(s *Buffer, _ io.Writer) {
			// FIXME: check bounds, don't use private fields
			s.cursor = cursor{y: op.param(0, 1) - 1, x: op.param(1, 1) - 1}
		}
	case 'r':
		return func(s *Buffer, w io.Writer) {
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
		return func(s *Buffer, _ io.Writer) {
			s.SaveCursor()
		}
	case 'u':
		if op.intermediate == "" {
			return func(s *Buffer, _ io.Writer) {
				s.RestoreCursor()
			}
		}
	case 'h':
		if len(op.params) == 1 && op.params[0] == 1049 && op.intermediate == "?" {
			return func(s *Buffer, _ io.Writer) {
				s.SaveCursor()
				s.SwitchToAlternateBuffer()
			}
		}
	case 'l':
		if len(op.params) == 1 && op.params[0] == 1049 && op.intermediate == "?" {
			return func(s *Buffer, _ io.Writer) {
				s.SwitchToPrimaryBuffer()
				s.RestoreCursor()
			}
		}
	case 'c':
		return func(s *Buffer, pty io.Writer) {
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
		// SGR https://vt100.net/docs/vt510-rm/SGR.html
	case 'm':
		ps := op.param(0, 0)
		if !slices.Contains([]int{0, 1, 7, 27}, ps) {
			log.Printf("unknown SGR instruction %v\n", op)
		}
		return func(s *Buffer, w io.Writer) {
			switch ps {
			case 0:
				s.ResetBrush()
			case 1:
				// poor man's bold because I can't change the font
				s.brush = brush{fg: defaultFG, bg: color.NRGBA{A: 16}}
			case 7:
				s.brush = brush{fg: defaultBG, bg: defaultFG}
			case 27:
				s.brush = brush{fg: defaultFG, bg: defaultBG}

			}
		}
	}
	log.Printf("Unknown CSI instruction %v", op)

	return nil
}
