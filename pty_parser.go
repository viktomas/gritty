package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

type decoder struct {
	state        decoderState
	privateFlag  int
	intermediate []byte
	params       []byte
	osc          []byte
}

type operationType uint32

type operation struct {
	t            operationType
	r            rune
	intermediate string
	params       []int
	osc          string
}

// param returns parameter on the i index or def(ault) value if the param is missing or 0
func (o operation) param(i int, def int) int {
	if len(o.params) == 0 || len(o.params) < i {
		return def
	}
	if o.params[i] == 0 {
		return def
	}
	return o.params[i]
}

var opTypeString = map[operationType]string{
	iexecute: "execute",
	iprint:   "print",
	iesc:     "ESC",
	icsi:     "CSI",
	iosc:     "OSC",
}

func (i operation) String() string {
	return fmt.Sprintf("%s: fc: %q, params: %v, inter: %s", opTypeString[i.t], string(i.r), i.params, i.intermediate)
}

const (
	iexecute operationType = iota
	iprint
	iesc
	icsi
	iosc
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
	sOSC
)

func NewDecoder() *decoder {
	return &decoder{
		state: sGround, // technically, this is not necessary because the sGround is 0
	}
}

func pExecute(b byte) operation {
	return operation{t: iexecute, r: rune(b)}
}

func pPrint(b byte) operation {
	return operation{t: iprint, r: rune(b)}
}

func (d *decoder) escDispatch(b byte) operation {
	return operation{t: iesc, r: rune(b), intermediate: string(d.intermediate)}
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

func (d *decoder) csiDispatch(b byte) operation {
	var params []int
	if len(d.params) > 0 {
		stringNumbers := strings.Split(string(d.params), ";")
		for _, sn := range stringNumbers {
			i, err := strconv.ParseInt(sn, 10, 32)
			if err != nil {
				log.Printf("tried to parse params %s but it doesn't contain only numbers and ;", d.params)
				continue
			}
			params = append(params, int(i))
		}
	}
	return operation{t: icsi, r: rune(b), params: params, intermediate: string(d.intermediate)}
}

func (d *decoder) oscDispatch() operation {
	return operation{t: iosc, osc: string(d.osc)}
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

func (d *decoder) Parse(p []byte) []operation {
	var result []operation
	for i := 0; i < len(p); i++ {
		b := p[i]
		// Anywhere
		if b == 0x1b {
			d.state = sEscape
			d.clear()
			continue
		}
		if b == 0x18 || b == 0x1a || btw(b, 0x80, 0x8F) || btw(b, 0x91, 0x97) || b == 0x99 || b == 0x9a {
			d.state = sGround
			result = append(result, pExecute(b))
			continue

		}
		if b == 0x9c {
			d.state = sGround
			continue
		}

		if b == 0x9D {
			d.state = sOSC
			d.osc = nil
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
			if b == 0x5d {
				d.osc = nil
				d.state = sOSC
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
		case sOSC:
			if isControlChar(b) {
				// ignore
			}
			if btw(b, 0x20, 0x7f) {
				d.osc = append(d.osc, b)
			}
			// 0x07 is xterm non-ANSI variant of transition to ground
			// taken from https://github.com/asciinema/avt/blob/main/src/vt.rs#L423C17-L423C74
			if b == 0x07 || b == 0x9c {
				result = append(result, d.oscDispatch())
				d.state = sGround
			}

		}
	}
	return result
}
