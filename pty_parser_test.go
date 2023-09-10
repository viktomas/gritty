package main

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("it parses control characters", func(t *testing.T) {
		for i := byte(0x00); i < 0x20; i++ {
			if i <= 0x17 || i == 0x19 || (i >= 0x1c && i <= 0x1f) {
				instructions := NewDecoder().Parse([]byte{i})
				if len(instructions) != 1 {
					t.Fatalf("The parser should have returned 1 instruction but returned %d for byte 0x%x", len(instructions), i)
				}
				if instructions[0].t != iexecute {
					t.Fatalf("The type of the instruction was supposed to be iexecute(%d), but was %d", iexecute, instructions[0].t)
				}
				if instructions[0].r != rune(i) {
					t.Fatalf("The rune in the instruction was supposed to be 0x%x, but was 0x%x", i, instructions[0].r)
				}
			}
		}
	})

}

func compInst(t testing.TB, expected, actual operation) {
	if expected.t != actual.t {
		t.Fatalf("instruction type is different, expected: %v, actual: %v", expected.t, actual.t)
	}
	if expected.r != actual.r {
		t.Fatalf("instruction final character is different, expected: %c, actual: %c", expected.r, actual.r)
	}
	if !reflect.DeepEqual(expected.params, actual.params) {
		t.Fatalf("instruction params are different, expected: %v, actual: %v", expected.params, actual.params)
	}
	if !reflect.DeepEqual(expected.intermediate, actual.intermediate) {
		t.Fatalf("instruction intermediate chars are different, expected: %s, actual: %s", expected.intermediate, actual.intermediate)
	}

}
func TestParseCSI(t *testing.T) {
	t.Run("parses cursor movements", func(t *testing.T) {
		testCases := []struct {
			desc  string
			input byte
		}{
			{desc: "cursor up", input: 'A'},
			{desc: "cursor down", input: 'B'},
			{desc: "cursor forward", input: 'C'},
			{desc: "cursor back", input: 'D'},
			{desc: "cursor next line", input: 'E'},
			{desc: "cursor previous line", input: 'F'},
			{desc: "cursor horizontal absolute", input: 'G'},
		}
		for _, tC := range testCases {
			t.Run(tC.desc, func(t *testing.T) {
				input := []byte{asciiESC, '[', tC.input, asciiESC, '[', '3', '9', tC.input}
				instructions := NewDecoder().Parse(input)
				if len(instructions) != 2 {
					t.Fatalf("The parser should have returned 2 instruction but returned %d for byte %v", len(instructions), input)
				}

				i0, i1 := instructions[0], instructions[1]
				expected0 := operation{t: icsi, r: rune(tC.input)}
				expected1 := operation{t: icsi, r: rune(tC.input), params: []int{39}}
				compInst(t, expected0, i0)
				compInst(t, expected1, i1)
			})
		}

	})

	t.Run("parse private sequences", func(t *testing.T) {
		instructions := NewDecoder().Parse([]byte{asciiESC, '[', '?', '1', '0', '4', '9', 'h'})
		compInst(
			t,
			operation{t: icsi, r: 'h', intermediate: "?", params: []int{1049}},
			instructions[0],
		)
	})

}
