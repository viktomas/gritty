package controller

import (
	"strings"

	"gioui.org/io/key"
)

func keyToBytes(name string, mod key.Modifiers) []byte {
	if mod.Contain(key.ModCtrl) {
		if len(name) == 1 && name[0] >= 0x40 && name[0] <= 0x5f {
			return []byte{name[0] - 0x40}
		}
	}
	switch name {
	// Handle ANSI escape sequence for Enter key
	case key.NameReturn:
		return []byte("\r")
	case key.NameDeleteBackward:
		return []byte{asciiDEL}
	case key.NameSpace:
		return []byte(" ")
	case key.NameEscape:
		return []byte{asciiESC}
	case key.NameTab:
		return []byte{asciiHT}
	case key.NameUpArrow:
		return []byte("\x1b[A")
	case key.NameDownArrow:
		return []byte("\x1b[B")
	case key.NameRightArrow:
		return []byte("\x1b[C")
	case key.NameLeftArrow:
		return []byte("\x1b[D")
	default:
		// For normal characters, pass them through.
		var character string
		if mod.Contain(key.ModShift) {
			character = strings.ToUpper(name)
		} else {
			character = strings.ToLower(name)
		}
		return []byte(character)
	}
}
