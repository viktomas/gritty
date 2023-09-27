package controller

import (
	"fmt"
	"io"
	"log"

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
		// 4bit color
		if ps >= 30 && ps <= 37 {
			color := get3bitNormalColor(uint8(ps - 30))
			br := b.Brush()
			br.FG = color
			b.SetBrush(br)
			return
		}
		if ps >= 90 && ps <= 97 {
			color := get3bitBrightColor(uint8(ps - 90))
			br := b.Brush()
			br.FG = color
			b.SetBrush(br)
			return
		}
		if ps >= 40 && ps <= 47 {
			color := get3bitNormalColor(uint8(ps - 40))
			br := b.Brush()
			br.BG = color
			b.SetBrush(br)
			return
		}
		if ps >= 100 && ps <= 107 {
			color := get3bitNormalColor(uint8(ps - 100))
			br := b.Brush()
			br.BG = color
			b.SetBrush(br)
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
		case 38:
			switch op.Param(1, 0) {
			case 5:
				color := get256Color(uint8(op.Param(2, 0)))
				br := b.Brush()
				br.FG = color
				b.SetBrush(br)
			case 2:
				color := buffer.NewColor(uint8(op.Param(2, 0)), uint8(op.Param(3, 0)), uint8(op.Param(4, 0)))
				br := b.Brush()
				br.FG = color
				b.SetBrush(br)

			default:
				log.Printf("unknown SGR instruction %v\n", op)
			}
		case 48:
			switch op.Param(1, 0) {
			case 5:
				color := get256Color(uint8(op.Param(2, 0)))
				br := b.Brush()
				br.BG = color
				b.SetBrush(br)
			case 2:
				color := buffer.NewColor(uint8(op.Param(2, 0)), uint8(op.Param(3, 0)), uint8(op.Param(4, 0)))
				br := b.Brush()
				br.BG = color
				b.SetBrush(br)
			default:
				log.Printf("unknown SGR instruction %v\n", op)
			}
		default:
			log.Printf("unknown SGR instruction %v\n", op)
		}
	default:
		log.Printf("Unknown CSI instruction %v", op)
	}
}

// get3bitNormalColor returns a color based on the SGR 30-37
// I used the VS Code color palette
// more info here https://en.wikipedia.org/wiki/ANSI_escape_code#3-bit_and_4-bit
func get3bitNormalColor(n uint8) buffer.Color {
	if n > 7 {
		log.Printf("get3BitNormalColor was invoked with incorrect number %d\n", n)
		n = 0
	}
	colors := []buffer.Color{
		{R: 0, G: 0, B: 0},       // black
		{R: 205, G: 49, B: 49},   // red
		{R: 13, G: 188, B: 121},  // green
		{R: 229, G: 229, B: 16},  // yellow
		{R: 36, G: 114, B: 200},  // blue
		{R: 188, G: 63, B: 188},  // magenta
		{R: 17, G: 168, B: 205},  // cyan
		{R: 229, G: 229, B: 229}, // white
	}
	return colors[n]
}

// get3bitBrightColor returns a color based on the SGR 30-37 and 90-97 scale
// I used the VS Code color palette
// more info here https://en.wikipedia.org/wiki/ANSI_escape_code#3-bit_and_4-bit
func get3bitBrightColor(n uint8) buffer.Color {
	if n > 7 {
		log.Printf("get3BitBrightColor was invoked with incorrect number %d\n", n)
		n = 0
	}
	colors := []buffer.Color{
		{R: 102, G: 102, B: 102}, // bright black
		{R: 241, G: 76, B: 76},   // bright red
		{R: 35, G: 209, B: 139},  // bright green
		{R: 245, G: 245, B: 67},  // bright yellow
		{R: 59, G: 142, B: 234},  // bright blue
		{R: 214, G: 112, B: 214}, // bright magenta
		{R: 41, G: 184, B: 219},  // bright cyan
		{R: 229, G: 229, B: 229}, // bright white
	}
	return colors[n]
}

func get256Color(n uint8) buffer.Color {
	switch {
	case n < 8:
		return get3bitNormalColor(n)
	case n < 16:
		return get3bitBrightColor(n - 8)
	case n < 232:
		// 6x6x6 color cube
		n -= 16
		levels := []uint8{0, 95, 135, 175, 215, 255}
		return buffer.NewColor(levels[(n/36)%6], levels[(n/6)%6], levels[n%6])
	default:
		// grayscale
		level := 8 + (n-232)*10
		return buffer.NewColor(level, level, level)
	}
}
