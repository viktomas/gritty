package main

import (
	"fmt"
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
			input rune
		}{
			{desc: "cursor up", input: 'A'},
			{desc: "cursor down", input: 'B'},
			{desc: "cursor forward", input: 'C'},
			{desc: "cursor back", input: 'D'},
			{desc: "cursor next line", input: 'E'},
			{desc: "cursor previous line", input: 'F'},
			{desc: "cursor horizontal absolute", input: 'G'},
		}
		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				input := []byte(fmt.Sprintf("\x1b[%[1]c\x1b[39%[1]c", tc.input))
				instructions := NewDecoder().Parse(input)
				if len(instructions) != 2 {
					t.Fatalf("The parser should have returned 2 instruction but returned %d for byte %v", len(instructions), input)
				}

				i0, i1 := instructions[0], instructions[1]
				expected0 := operation{t: icsi, r: rune(tc.input)}
				expected1 := operation{t: icsi, r: rune(tc.input), params: []int{39}}
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

	t.Run("parse", func(t *testing.T) {
		tests := []struct {
			input    []byte
			expected operation
		}{
			{[]byte("\x1b[1m"), operation{t: icsi, params: []int{1}, r: 'm'}}, // Bold
			{[]byte("\x1b[4m"), operation{t: icsi, params: []int{4}, r: 'm'}}, // Underline
			{[]byte("\x1b[H"), operation{t: icsi, r: 'H'}},                    // Cursor Home
			{[]byte("\x1b[J"), operation{t: icsi, r: 'J'}},                    // Erase display
			{[]byte("\x1b[K"), operation{t: icsi, r: 'K'}},                    // Erase line
			{[]byte("\x1b[0H"), operation{t: icsi, params: []int{0}, r: 'H'}}, // Erase line
		}

		for _, test := range tests {
			output := NewDecoder().Parse(test.input) // Assuming `parse` is your parsing function
			if !reflect.DeepEqual(test.expected, output[0]) {
				t.Fatalf("parsed as %v, but should have been %v", output[0], test.expected)
			}
		}
	})

	t.Run("goes to ground from CSI entry", func(t *testing.T) {
		output := NewDecoder().Parse([]byte{0x1b, 0x5b, 0x4b, 0x61})
		if len(output) != 2 {
			t.Fatalf("the input should have been parsed into 2 operations")
		}
		expected1 := operation{t: icsi, r: 'K'}
		if !reflect.DeepEqual(expected1, output[0]) {
			t.Fatalf("first operation should have been %v, but was %v", expected1, output[0])
		}
		expected2 := operation{t: iprint, r: 'a'}
		if !reflect.DeepEqual(expected2, output[1]) {
			t.Fatalf("second operation should have been %v, but was %v", expected2, output[1])
		}
	})
}

func TestParam(t *testing.T) {
	testCases := []struct {
		desc     string
		params   []int
		index    int
		dflt     int
		expected int
	}{
		{
			desc:     "returns default if params is empty",
			params:   []int{},
			index:    0,
			dflt:     10,
			expected: 10,
		},
		{
			desc:     "returns default if params is too short",
			params:   []int{1},
			index:    1,
			dflt:     10,
			expected: 10,
		},
		{
			desc:     "returns default if the parsed param is 0",
			params:   []int{0},
			index:    0,
			dflt:     10,
			expected: 10,
		},
		{
			desc:     "returns the value on the index",
			params:   []int{1, 2},
			index:    1,
			dflt:     10,
			expected: 2,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			op := operation{t: icsi, params: tc.params, r: 'A'}
			result := op.param(tc.index, tc.dflt)
			if result != tc.expected {
				t.Fatalf("the result should have been %d, but was %d", tc.expected, result)
			}
		})
	}
}
