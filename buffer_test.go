package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestMakeTestBuffer(t *testing.T) {
	b := makeTestBuffer(t, `
	a__
	_b_
	__c
	`, 0, 0)
	expected := trimExpectation(t, `
	a__
	_b_
	__c
	`)
	if b.String() != expected {
		t.Fatalf("Buffer wasn't created successfully\nExpected:\n%s\nGot:\n%s", expected, b.String())
	}

}

func TestNewBuffer(t *testing.T) {
	s := NewBuffer(5, 2)
	if s.String() != "     \n     \n" {
		t.Fatalf("the buffer string is not equal to empty buffer 5x2:\n%q", s.String())
	}
}

func FuzzWriteRune(f *testing.F) {
	b := NewBuffer(20, 10)
	f.Add('A')
	f.Fuzz(func(t *testing.T, r rune) {
		b.WriteRune(r)
	})
}

func TestClearLines(t *testing.T) {
	t.Run("full clear", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a__
		_b_
		__c
		`, 0, 0)
		expected := trimExpectation(t, `
		___
		___
		___
		`)
		b.ClearLines(0, b.size.rows)
		if b.String() != expected {
			t.Fatalf("Buffer wasn't cleared successfully\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})
}

func TestScrollUp(t *testing.T) {
	t.Run("without margins", func(t *testing.T) {
		b := makeTestBuffer(t, `
		ab
		cd
		`, 0, 0)
		expected := trimExpectation(t, `
		cd
		__
		`)
		b.ScrollUp(1)
		if b.String() != expected {
			t.Fatalf("Buffer didn't scroll up\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})
	t.Run("with margins", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a
		b
		c
		d
		e
		`, 0, 0)
		expected := trimExpectation(t, `
		a
		d
		_
		_
		e
		`)
		b.SetScrollArea(1, 4)
		b.ScrollUp(2)
		if b.String() != expected {
			t.Fatalf("Buffer didn't scroll up within margin\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})
}

func TestSetScrollArea(t *testing.T) {
	b := NewBuffer(2, 5)
	b.WriteRune('a')
	b.SetScrollArea(1, 3)

	if b.Cursor() != (Cursor{X: 0, Y: 1}) {
		t.Fatalf("Cursor should be set to the top of the scroll region (0,1), but is (%d,%d)", b.cursor.X, b.cursor.Y)
	}
}

func TestWriteRune(t *testing.T) {
	t.Run("auto wraps", func(t *testing.T) {
		b := NewBuffer(2, 2)
		b.WriteRune('a')
		b.WriteRune('a')
		b.WriteRune('a')
		if b.String() != "aa\na \n" {
			t.Fatalf("the character didn't auto wrap:\n%q", b.String())
		}
	})

	t.Run("wraps only with next write (doesn't wrap when EOL is reached)", func(t *testing.T) {
		b := NewBuffer(2, 2)
		b.WriteRune('a')
		b.WriteRune('a')
		b.WriteRune('a')
		b.WriteRune('a')
		expected := "aa\naa\n"
		if b.String() != expected {
			t.Fatalf("buffer was supposed to be filled with a's:\nexpected:%s\ngot:\n%s", expected, b.String())
		}
		b.WriteRune('b')
		expected = "aa\nb \n"
		if b.String() != expected {
			t.Fatalf("next rune (b) was supposed to trigger autowrap (and scroll):\nexpected:%s\ngot:\n%s", expected, b.String())
		}

	})
}

func TestReverseIndex(t *testing.T) {
	t.Run("auto wraps", func(t *testing.T) {
		b := makeTestBuffer(t, `
		aa
		bb
		`, 0, 0)
		expected := trimExpectation(t, `
		__
		aa
		`)
		b.ReverseIndex()
		if b.String() != expected {
			t.Fatalf("Buffer didn't scroll down\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})

	t.Run("works with scroll region", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a
		b
		c
		d
		e
		`, 0, 0)
		expected := trimExpectation(t, `
		a
		_
		b
		c
		e
		`)
		b.SetScrollArea(1, 4)
		b.ReverseIndex()
		if b.String() != expected {
			t.Fatalf("Buffer didn't scroll down within the scroll region\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})
}

func makeTestBuffer(t testing.TB, content string, x, y int) *Buffer {
	t.Helper()
	rows := strings.Split(content, "\n")
	trimmedRows := make([]string, 0, len(rows))
	for _, r := range rows {
		line := strings.TrimSpace(r)
		if line != "" {
			trimmedRows = append(trimmedRows, line)
		}
	}
	if len(trimmedRows) == 0 {
		t.Fatal("the make test buffer input is empty")
	}
	for _, r := range trimmedRows {
		if len(r) != len(trimmedRows[0]) {
			t.Fatal("test buffer has lines with different length")
		}
	}
	b := NewBuffer(len(trimmedRows[0]), len(trimmedRows))
	for _, r := range trimmedRows {
		for _, c := range r {
			if c == '_' {
				c = ' '
			}
			b.WriteRune(c)
		}
	}
	b.SetCursor(x, y)
	return b
}

func trimExpectation(t testing.TB, expected string) string {
	rows := strings.Split(expected, "\n")
	trimmedRows := make([]string, 0, len(rows))
	for _, r := range rows {
		line := strings.TrimSpace(r)
		if line != "" {
			trimmedRows = append(
				trimmedRows,
				strings.ReplaceAll(line, "_", " "),
			)
		}
	}
	// adds trailing new line because that's what the buffer.String() method does
	return fmt.Sprintf("%s\n", strings.Join(trimmedRows, "\n"))
}
