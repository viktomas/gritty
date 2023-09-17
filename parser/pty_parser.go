package parser

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type Parser struct {
	state        parserState
	privateFlag  int
	buf          []byte
	intermediate []byte
	params       []byte
	osc          []byte
}

type OperationType uint32

type Operation struct {
	T            OperationType
	R            rune
	Intermediate string
	Params       []int
	Osc          string
}

// Param returns parameter on the i index or def(ault) value if the Param is missing or 0
func (o Operation) Param(i int, def int) int {
	if len(o.Params) == 0 || len(o.Params) <= i {
		return def
	}
	if o.Params[i] == 0 {
		return def
	}
	return o.Params[i]
}

func (o Operation) String() string {
	switch o.T {
	case OpOSC:
		return fmt.Sprintf("OSC: %q", o.Osc)
	case OpPrint:
		return fmt.Sprintf("print: %q", string(o.R))
	case OpExecute:
		return fmt.Sprintf("execute: %q", string(o.R))
	case OpESC:
		return fmt.Sprintf("ESC: %s %q", o.Intermediate, string(o.R))
	case OpCSI:
		return fmt.Sprintf("CSI: %s %v %q", o.Intermediate, o.Params, string(o.R))
	default:
		log.Fatalln("Unknown operation type: ", o.T)
		return ""
	}
}

const (
	OpExecute OperationType = iota
	OpPrint
	OpESC
	OpCSI
	OpOSC
)

type parserState int

const (
	sGround parserState = iota
	sEscape
	sEscapeIntermediate
	sCSIEntry
	sCSIParam
	sCSIIgnore
	sCSIIntermediate
	sOSC
)

func New() *Parser {
	return &Parser{
		state: sGround, // technically, this is not necessary because the sGround is 0
	}
}

func (d *Parser) pExecute(b byte) Operation {
	logDebug("Executing: %v\n", hex.EncodeToString(d.buf))
	d.buf = nil
	return Operation{T: OpExecute, R: rune(b)}
}

func (d *Parser) pPrint(b byte) Operation {
	logDebug("Printing: %v\n", hex.EncodeToString(d.buf))
	d.buf = nil
	return Operation{T: OpPrint, R: rune(b)}
}

func (d *Parser) escDispatch(b byte) Operation {
	logDebug("ESC: %v\n", hex.EncodeToString(d.buf))
	d.buf = nil
	return Operation{T: OpESC, R: rune(b), Intermediate: string(d.intermediate)}
}

func (d *Parser) csiDispatch(b byte) Operation {
	logDebug("CSI: %v\n", hex.EncodeToString(d.buf))
	d.buf = nil
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
	return Operation{T: OpCSI, R: rune(b), Params: params, Intermediate: string(d.intermediate)}
}

func (d *Parser) oscDispatch() Operation {
	logDebug("OSC: %v\n", hex.EncodeToString(d.buf))
	d.buf = nil
	return Operation{T: OpOSC, Osc: string(d.osc)}
}

func (d *Parser) clear() {
	d.privateFlag = 0
	d.intermediate = nil
	d.params = nil
}

func (d *Parser) collect(b byte) {
	d.intermediate = append(d.intermediate, b)
}

func (d *Parser) param(b byte) {
	d.params = append(d.params, b)
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

// Parse parses bytes received from PTY based on the excellent state diagram by Paul Williams https://www.vt100.net/emu/dec_ansi_parser
func (d *Parser) Parse(p []byte) []Operation {
	var result []Operation
	for i := 0; i < len(p); i++ {
		b := p[i]
		d.buf = append(d.buf, b)
		// Anywhere
		if b == 0x1b {
			d.state = sEscape
			d.clear()
			continue
		}
		if b == 0x18 || b == 0x1a || btw(b, 0x80, 0x8F) || btw(b, 0x91, 0x97) || b == 0x99 || b == 0x9a {
			d.state = sGround
			result = append(result, d.pExecute(b))
			continue

		}
		if b == 0x9D {
			d.state = sOSC
			d.osc = nil
			continue
		}
		switch d.state {
		case sGround:
			if isControlChar(b) {
				result = append(result, d.pExecute(b))
			}
			if b >= 0x20 && b <= 0x7f {
				result = append(result, d.pPrint(b))
			}
		case sEscape:
			if isControlChar(b) {
				result = append(result, d.pExecute(b))
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
				result = append(result, d.pExecute(b))
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
				result = append(result, d.pExecute(b))
			}
			if btw(b, 0x40, 0x7e) {
				result = append(result, d.csiDispatch(b))
				d.state = sGround
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
				result = append(result, d.pExecute(b))
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
				result = append(result, d.pExecute(b))
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
				result = append(result, d.pExecute(b))
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
			if b == 0x9c {
				d.state = sGround
			}
		}
	}
	return result
}

func logDebug(f string, vars ...any) {
	if os.Getenv("gritty_debug") != "" {
		fmt.Printf(f, vars...)
	}
}
