package parser

import (
	"fmt"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("it parses control characters", func(t *testing.T) {
		for i := byte(0x00); i < 0x20; i++ {
			if i <= 0x17 || i == 0x19 || (i >= 0x1c && i <= 0x1f) {
				instructions := New().Parse([]byte{i})
				if len(instructions) != 1 {
					t.Fatalf("The parser should have returned 1 instruction but returned %d for byte 0x%x", len(instructions), i)
				}
				if instructions[0].T != OpExecute {
					t.Fatalf("The type of the instruction was supposed to be iexecute(%d), but was %d", OpExecute, instructions[0].T)
				}
				if instructions[0].R != rune(i) {
					t.Fatalf("The rune in the instruction was supposed to be 0x%x, but was 0x%x", i, instructions[0].R)
				}
			}
		}
	})

}

func compInst(t testing.TB, expected, actual Operation) {
	if expected.T != actual.T {
		t.Fatalf("instruction type is different, expected: %v, actual: %v", expected.T, actual.T)
	}
	if expected.R != actual.R {
		t.Fatalf("instruction final character is different, expected: %c, actual: %c", expected.R, actual.R)
	}
	if !reflect.DeepEqual(expected.Params, actual.Params) {
		t.Fatalf("instruction params are different, expected: %v, actual: %v", expected.Params, actual.Params)
	}
	if !reflect.DeepEqual(expected.Intermediate, actual.Intermediate) {
		t.Fatalf("instruction intermediate chars are different, expected: %s, actual: %s", expected.Intermediate, actual.Intermediate)
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
				instructions := New().Parse(input)
				if len(instructions) != 2 {
					t.Fatalf("The parser should have returned 2 instruction but returned %d for byte %v", len(instructions), input)
				}

				i0, i1 := instructions[0], instructions[1]
				expected0 := Operation{T: OpCSI, R: rune(tc.input)}
				expected1 := Operation{T: OpCSI, R: rune(tc.input), Params: []int{39}}
				compInst(t, expected0, i0)
				compInst(t, expected1, i1)
			})
		}

	})

	t.Run("parse private sequences", func(t *testing.T) {
		instructions := New().Parse([]byte("\x1b[?1049h]"))
		compInst(
			t,
			Operation{T: OpCSI, R: 'h', Intermediate: "?", Params: []int{1049}},
			instructions[0],
		)
	})

	t.Run("parse", func(t *testing.T) {
		tests := []struct {
			input    []byte
			expected Operation
		}{
			{[]byte("\x1b[1m"), Operation{T: OpCSI, Params: []int{1}, R: 'm'}}, // Bold
			{[]byte("\x1b[4m"), Operation{T: OpCSI, Params: []int{4}, R: 'm'}}, // Underline
			{[]byte("\x1b[H"), Operation{T: OpCSI, R: 'H'}},                    // Cursor Home
			{[]byte("\x1b[J"), Operation{T: OpCSI, R: 'J'}},                    // Erase display
			{[]byte("\x1b[K"), Operation{T: OpCSI, R: 'K'}},                    // Erase line
			{[]byte("\x1b[0H"), Operation{T: OpCSI, Params: []int{0}, R: 'H'}}, // Erase line
		}

		for _, test := range tests {
			output := New().Parse(test.input) // Assuming `parse` is your parsing function
			if !reflect.DeepEqual(test.expected, output[0]) {
				t.Fatalf("parsed as %v, but should have been %v", output[0], test.expected)
			}
		}
	})

	t.Run("goes to ground from CSI entry", func(t *testing.T) {
		output := New().Parse([]byte{0x1b, 0x5b, 0x4b, 0x61})
		if len(output) != 2 {
			t.Fatalf("the input should have been parsed into 2 operations")
		}
		expected1 := Operation{T: OpCSI, R: 'K'}
		if !reflect.DeepEqual(expected1, output[0]) {
			t.Fatalf("first operation should have been %v, but was %v", expected1, output[0])
		}
		expected2 := Operation{T: OpPrint, R: 'a'}
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
			op := Operation{T: OpCSI, Params: tc.params, R: 'A'}
			result := op.Param(tc.index, tc.dflt)
			if result != tc.expected {
				t.Fatalf("the result should have been %d, but was %d", tc.expected, result)
			}
		})
	}
}
