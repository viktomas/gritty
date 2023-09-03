package main

import "log"

type screenOp func(*Screen)

func translateCSI(op operation) screenOp {
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
	case 'B':
		dy := op.param(0, 1)
		return func(s *Screen) {
			s.MoveCursor(0, dy)
		}
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
			s.LineOp(func(line []rune, cursorCol int) int {
				var toClear []rune
				switch op.param(0, 0) {
				case 0:
					toClear = line[cursorCol:] // erase from cursor to the end of the line
				case 1:
					toClear = line[:cursorCol+1] // erase from cursor to the start of the line
				case 2:
					toClear = line // erase the whole line
				}
				for i := range toClear {
					toClear[i] = ' '
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

	// DEC Private Mode Set (DECSET).
	// received op:  CSI: fc: "h", params: [1], inter: ?
	// received op:  CSI: fc: "h", params: [2004], inter: ?

	// received op:  CSI: fc: "r", params: [1 31], inter:
	// received op:  CSI: fc: "m", params: [27], inter:
	// received op:  CSI: fc: "m", params: [24], inter:
	// received op:  CSI: fc: "m", params: [23], inter:
	// received op:  CSI: fc: "m", params: [0], inter:
	// received op:  CSI: fc: "l", params: [25], inter: ?
	return nil
}
