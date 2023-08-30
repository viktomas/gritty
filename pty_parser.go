package main

import (
	"fmt"
)

type decoder struct {
	state        decoderState
	privateFlag  int
	intermediate []byte
	params       []byte
}

type instructionType uint32

type instruction struct {
	t            instructionType
	r            rune
	intermediate []byte
	params       []byte
}

var instructionTypeString = map[instructionType]string{
	iexecute: "execute",
	iprint:   "print",
	iesc:     "ESC",
	icsi:     "CSI",
}

func (i instruction) String() string {
	return fmt.Sprintf("%s: fc: %c, params: %s, inter: %s", instructionTypeString[i.t], i.r, i.params, i.intermediate)
}

const (
	iexecute instructionType = iota
	iprint
	iesc
	icsi
)

type decoderState int

const (
	sGround decoderState = iota
	sEscape
	sEscapeIntermediate
	sCSIEntry
	sCSIParam
	sCSIIgnore
	sCSIIntermediate
)

func NewDecoder() *decoder {
	return &decoder{
		state: sGround, // technically, this is not necessary because the sGround is 0
	}
}

func pExecute(b byte) instruction {
	return instruction{t: iexecute, r: rune(b)}
}

func pPrint(b byte) instruction {
	return instruction{t: iprint, r: rune(b)}
}

func (d *decoder) escDispatch(b byte) instruction {
	return instruction{t: iesc, r: rune(b), intermediate: d.intermediate, params: d.params}
}

func (d *decoder) clear() {
	d.privateFlag = 0
	d.intermediate = nil
	d.params = nil
}

func (d *decoder) collect(b byte) {
	d.intermediate = append(d.intermediate, b)
}

func (d *decoder) param(b byte) {
	d.params = append(d.params, b)
}

func (d *decoder) csiDispatch(b byte) instruction {
	return instruction{t: icsi, r: rune(b), params: d.params, intermediate: d.intermediate}
}

// btw (between) returns true if b >= start && b <= end
// in other words it checks whether b is in the boundaries set by Start and End *inclusive*
func btw(b, start, end byte) bool {
	return b >= start && b <= end
}

// in checks if the byte is in the set of values given by vals
func in(b byte, vals ...byte) bool {
	for _, v := range vals {
		if v == b {
			return true
		}
	}
	return false
}

func isControlChar(b byte) bool {
	return btw(b, 0x00, 0x17) || b == 0x19 || btw(b, 0x1c, 0x1f)
}

func (d *decoder) Parse(p []byte) []instruction {
	var result []instruction
	for i := 0; i < len(p); i++ {
		b := p[i]
		// Anywhere
		if b == 0x1b {
			d.state = sEscape
			d.clear()
			continue
		}
		switch d.state {
		case sGround:
			if isControlChar(b) {
				result = append(result, pExecute(b))
			}
			if b >= 0x20 && b <= 0x7f {
				result = append(result, pPrint(b))
			}
		case sEscape:
			if isControlChar(b) {
				result = append(result, pExecute(b))
			}
			if btw(b, 0x30, 0x4f) || btw(b, 0x51, 0x57) || in(b, 0x59, 0x5a, 0x5C) || btw(b, 0x60, 0x7e) {
				result = append(result, d.escDispatch(b))
				d.state = sGround
			}
			if btw(b, 0x20, 0x2f) {
				d.collect(b)
				d.state = sEscapeIntermediate
			}
			if b == 0x5b {
				d.clear()
				d.state = sCSIEntry
			}
			// 7f ignore
		case sEscapeIntermediate:
			if isControlChar(b) {
				result = append(result, pExecute(b))
			}
			if btw(b, 0x20, 0x2f) {
				d.collect(b)
			}
			if btw(b, 0x30, 0x7e) {
				result = append(result, d.escDispatch(b))
				d.state = sGround
			}
			// 7f ignore
		case sCSIEntry:
			if isControlChar(b) {
				result = append(result, pExecute(b))
			}
			if btw(b, 0x40, 0x7e) {
				result = append(result, d.csiDispatch(b))
			}
			if btw(b, 0x30, 0x39) || b == 0x3b {
				d.param(b)
				d.state = sCSIParam
			}
			if btw(b, 0x3c, 0x3f) {
				d.collect(b)
				d.state = sCSIParam
			}
			if b == 0x3a {
				d.state = sCSIIgnore
			}
			// 7f ignore
		case sCSIParam:
			if isControlChar(b) {
				result = append(result, pExecute(b))
			}
			if btw(b, 0x30, 0x39) || b == 0x3b {
				d.param(b)
			}
			if btw(b, 0x40, 0x7e) {
				result = append(result, d.csiDispatch(b))
				d.state = sGround
			}
			if btw(b, 0x20, 0x2f) {
				d.collect(b)
				d.state = sCSIIntermediate
			}
			if b == 0x3a || btw(b, 0x3c, 0x3f) {
				d.state = sCSIIgnore
			}
			// 7f ignore
		case sCSIIntermediate:
			if isControlChar(b) {
				result = append(result, pExecute(b))
			}
			if btw(b, 0x20, 0x2f) {
				d.collect(b)
			}
			if btw(b, 0x40, 0x7e) {
				result = append(result, d.csiDispatch(b))
				d.state = sGround
			}
			if btw(b, 0x30, 0x3f) {
				d.state = sCSIIgnore
			}
			// 7f ignore
		case sCSIIgnore:
			if isControlChar(b) {
				result = append(result, pExecute(b))
			}
			if btw(b, 0x40, 0x7e) {
				d.state = sGround
			}
			// 20-3f,7f ignore

		}
	}
	return result
}
