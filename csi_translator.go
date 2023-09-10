package main

import (
	"fmt"
	"log"
)

type screenOp func(*Screen)

func translateCSI(op operation) screenOp {
	fmt.Println("executing CSI op: ", op)
	if op.t != icsi {
		log.Printf("operation %v is not CSI but it was passed to CSI translator.\n", op)
		return nil
	}

	switch op.r {
	case 'A':
		dy := op.param(0, 1)
		return func(s *Screen) {
			s.MoveCursor(0, -dy)
		}
		// FIXME fix bug in https://github.com/asciinema/avt/blob/main/src/vt.rs#L548C37-L548C37, where it is implemented as up, but should be down based on
		// - http://xtermjs.org/docs/api/vtfeatures/
		// - https://invisible-island.net/xterm/ctlseqs/ctlseqs.html
	case 'e':
		fallthrough
	case 'B':
		dy := op.param(0, 1)
		return func(s *Screen) {
			s.MoveCursor(0, dy)
		}
	case 'a': // a is also CUF
		fallthrough
	// CUF
	case 'C':
		dx := op.param(0, 1)
		return func(s *Screen) {
			s.MoveCursor(dx, 0)
		}
	case 'D':
		dx := op.param(0, 1)
		return func(s *Screen) {
			s.MoveCursor(-dx, 0)
		}
	case 'J':
		return func(s *Screen) {
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
		return func(s *Screen) {
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
	case 'H':
		return func(s *Screen) {
			// FIXME: check bounds, don't use private fields
			s.cursor = cursor{y: op.param(0, 1) - 1, x: op.param(1, 1) - 1}
		}
	case 's':
		return func(s *Screen) {
			s.SaveCursor()
		}
	case 'u':
		if op.intermediate == "" {
			return func(s *Screen) {
				s.RestoreCursor()
			}
		}
	case 'h':
		if len(op.params) == 1 && op.params[0] == 1049 && op.intermediate == "?" {
			return func(s *Screen) {
				s.SaveCursor()
				s.SwitchToAlternateBuffer()
				s.AdjustToNewSize()
			}
		}
	case 'l':
		if len(op.params) == 1 && op.params[0] == 1049 && op.intermediate == "?" {
			return func(s *Screen) {
				s.SwitchToPrimaryBuffer()
				s.RestoreCursor()
				s.AdjustToNewSize()
			}
		}
	}
	log.Printf("Unknown CSI instruction %v", op)

	// CSI: fc: "v", params: [], inter:
	// CSI: fc: "c", params: [], inter:
	// CSI: fc: "o", params: [], inter:
	// CSI: fc: "n", params: [], inter:
	// CSI: fc: "r", params: [], inter:
	// CSI: fc: "t", params: [], inter:
	return nil
}
