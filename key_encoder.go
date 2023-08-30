package main

import (
	"strings"

	"gioui.org/io/key"
)

func keyToBytes(name string, mod key.Modifiers) []byte {
	if mod.Contain(key.ModCtrl) {
		switch name {
		case "C":
			return []byte{asciiETX} // return ETX (end of text, ^C)
		case "D":
			return []byte{asciiEOT} // return EOT (end of transmission)
		}
	}
	switch name {
	// Handle ANSI escape sequence for Enter key
	case key.NameReturn:
		return []byte("\r")
	case key.NameDeleteBackward:
		return []byte{asciiBS}
	case key.NameSpace:
		return []byte(" ")
	case key.NameEscape:
		return []byte{0x1B}
	case key.NameTab:
		return []byte{0x09}
	case key.NameUpArrow:
		return []byte{0x1b, '[', 'A'}
	case key.NameDownArrow:
		return []byte{0x1b, '[', 'B'}
	case key.NameRightArrow:
		return []byte{0x1b, '[', 'C'}
	case key.NameLeftArrow:
		return []byte{0x1b, '[', 'D'}
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
