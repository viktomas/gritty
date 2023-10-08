package buffer

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
	s := New(5, 2)
	if s.String() != "     \n     \n" {
		t.Fatalf("the buffer string is not equal to empty buffer 5x2:\n%q", s.String())
	}
}

func FuzzWriteRune(f *testing.F) {
	b := New(20, 10)
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
		b.ClearLines(0, b.size.Rows)
		if b.String() != expected {
			t.Fatalf("Buffer wasn't cleared successfully\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})

	t.Run("partial clear", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a___
		_b__
		__c_
		___d
		`, 0, 0)
		expected := trimExpectation(t, `
		a___
		____
		____
		___d
		`)
		b.ClearLines(1, 3)
		if b.String() != expected {
			t.Fatalf("Buffer wasn't cleared successfully\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})

	t.Run("clear with range too large", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a_
		_b
		`, 0, 0)
		expected := trimExpectation(t, `
		__
		__
		`)
		b.ClearLines(-11, 33)
		if b.String() != expected {
			t.Fatalf("Buffer wasn't cleared successfully\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})

	t.Run("clear with range too small", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a_
		_b
		`, 0, 0)
		expected := trimExpectation(t, `
		a_
		_b
		`)
		b.ClearLines(4, 3)
		if b.String() != expected {
			t.Fatalf("Buffer wasn't cleared successfully\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})
}

func TestClearCurrentLine(t *testing.T) {
	t.Run("clears full line", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a___
		_b__
		__c_
		___d
		`, 0, 1)
		expected := trimExpectation(t, `
		a___
		____
		__c_
		___d
		`)
		b.ClearCurrentLine(0, b.Size().Cols)
		if b.String() != expected {
			t.Fatalf("Line was not fully cleared:\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})
	t.Run("clears part of the line", func(t *testing.T) {
		b := makeTestBuffer(t, `12345`, 0, 0)
		expected := trimExpectation(t, `1___5`)
		b.ClearCurrentLine(1, 4)
		if b.String() != expected {
			t.Fatalf("Line was not cleared from 2 to 4:\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})
	t.Run("handles range out of bounds", func(t *testing.T) {
		b := makeTestBuffer(t, `12345`, 0, 0)
		expected := trimExpectation(t, `_____`)
		b.ClearCurrentLine(-10, 33)
		if b.String() != expected {
			t.Fatalf("Line was not fully cleared:\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})
	t.Run("handles too small range", func(t *testing.T) {
		b := makeTestBuffer(t, `12345`, 0, 0)
		expected := trimExpectation(t, `12345`)
		b.ClearCurrentLine(4, 3)
		if b.String() != expected {
			t.Fatalf("Line was changed but it shouldn't have:\nExpected:\n%s\nGot:\n%s", expected, b.String())
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
	t.Run("sets scroll area within the buffer", func(t *testing.T) {
		b := New(2, 5)
		b.WriteRune('a')
		b.SetScrollArea(1, 3)

		if b.Cursor() != (Cursor{X: 0, Y: 1}) {
			t.Fatalf("Cursor should be set to the top of the scroll region (0,1), but is (%d,%d)", b.cursor.X, b.cursor.Y)
		}
	})

	t.Run("clamps parameters so that the range is always within the buffer", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a
		b
		c
		`, 0, 0)
		expected := trimExpectation(t, `
		b
		c
		_
		`)
		b.SetScrollArea(-10, 20)

		if b.Cursor() != (Cursor{X: 0, Y: 0}) {
			t.Fatalf("Cursor should be set to the top of the scroll region (0,0), but is (%d,%d)", b.cursor.X, b.cursor.Y)
		}

		b.ScrollUp(1)
		if b.String() != expected {
			t.Fatalf("The whole buffer should have scrolled but it didn't:\n%s\nGot:\n%s", expected, b.String())
		}
	})
}

func TestWriteRune(t *testing.T) {
	t.Run("auto wraps", func(t *testing.T) {
		b := New(2, 2)
		b.WriteRune('a')
		b.WriteRune('a')
		b.WriteRune('a')
		if b.String() != "aa\na \n" {
			t.Fatalf("the character didn't auto wrap:\n%q", b.String())
		}
	})

	t.Run("wraps only with next write (doesn't wrap when EOL is reached)", func(t *testing.T) {
		b := New(2, 2)
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

func TestLFResetsWrappingNextLine(t *testing.T) {
	b := makeTestBuffer(t, `
	___
	___
	___
	`, 0, 0)
	expected := trimExpectation(t, `
	xxx
	z__
	___
	`)
	b.WriteRune('x')
	b.WriteRune('x')
	b.WriteRune('x')
	b.CR()
	b.LF()
	b.WriteRune('z')
	if b.String() != expected {
		t.Fatalf("The line feed didn't reset the line wrapping\nExpected:\n%s\nGot:\n%s", expected, b.String())
	}
}

func TestDeleteCharacter(t *testing.T) {
	t.Run("deletes from a middle of the line", func(t *testing.T) {
		b := makeTestBuffer(t, `
		hello_world
		___________
		`, 1, 0)
		expected := trimExpectation(t, `
		ho_world___
		___________
		`)
		b.DeleteCharacter(3)
		if b.String() != expected {
			t.Fatalf("DeleteCharacter didn't remove 3 characters from the first e\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})

	t.Run("handles when the parameter is too large", func(t *testing.T) {
		b := makeTestBuffer(t, `
		hello_world
		___________
		`, 6, 0)
		expected := trimExpectation(t, `
		hello______
		___________
		`)
		b.DeleteCharacter(10)
		if b.String() != expected {
			t.Fatalf("We didn't remove the 'world' correctly\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})

	t.Run("can delete only one character", func(t *testing.T) {
		b := makeTestBuffer(t, `
		hello_world
		___________
		`, 1, 0)
		expected := trimExpectation(t, `
		hllo_world_
		___________
		`)
		b.DeleteCharacter(1)
		if b.String() != expected {
			t.Fatalf("DeleteCharacter didn't remove 3 characters from the first e\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})
}

func TestDeleteLine(t *testing.T) {
	t.Run("deletes lines in the middle", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a
		b
		c
		d
		e
		`, 0, 1)
		expected := trimExpectation(t, `
		a
		d
		e
		_
		_
		`)
		b.DeleteLine(2)
		if b.String() != expected {
			t.Fatalf("DeleteLine didn't remove 2 lines after 'a'\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})

	t.Run("deletes only one line", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a
		b
		c
		d
		e
		`, 0, 1)
		expected := trimExpectation(t, `
		a
		c
		d
		e
		_
		`)
		b.DeleteLine(1)
		if b.String() != expected {
			t.Fatalf("DeleteLine didn't remove the line after 'a'\nExpected:\n%s\nGot:\n%s", expected, b.String())
		}
	})

	t.Run("deletes lines when the parameter is too large", func(t *testing.T) {
		b := makeTestBuffer(t, `
		a
		b
		c
		d
		e
		`, 0, 3)
		expected := trimExpectation(t, `
		a
		b
		c
		_
		_
		`)
		b.DeleteLine(20)
		if b.String() != expected {
			t.Fatalf("DeleteLine didn't remove the last 2 lines\nExpected:\n%s\nGot:\n%s", expected, b.String())
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
	b := New(len(trimmedRows[0]), len(trimmedRows))
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
