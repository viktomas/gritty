package main

import "testing"

func TestNewScreen(t *testing.T) {
	s := NewScreen(5, 2)
	if s.String() != "     \n     \n" {
		t.Fatalf("the screen string is not equal to empty screen 5x2:\n%q", s.String())
	}
}

func TestClearFull(t *testing.T) {
	s := NewScreen(2, 2)
	s.WriteRune('a')
	s.ClearFull()
	if s.String() != "  \n  \n" {
		t.Fatalf("the screen has not been cleared and it is:\n%q", s.String())
	}
}

func TestScrollUp(t *testing.T) {
	t.Run("without margins", func(t *testing.T) {
		s := NewScreen(2, 2)
		s.WriteRune('a')
		s.CR()
		s.LF()
		s.WriteRune('b')
		s.scrollUp()
		if s.String() != "b \n  \n" {
			t.Fatalf("the a character was supposed to scroll of the screen:\n%q", s.String())
		}
	})
	t.Run("with margins", func(t *testing.T) {
		s := NewScreen(1, 5)
		s.WriteRune('a')
		s.WriteRune('b')
		s.WriteRune('c')
		s.WriteRune('d')
		s.SetScrollArea(1, 3)
		s.scrollUp()
		if s.String() != "a\nc\n \nd\n \n" {
			t.Fatalf("the b character was supposed to scroll of the margin:\n%q", s.String())
		}
	})
}

func TestSetScrollArea(t *testing.T) {
	s := NewScreen(2, 5)
	s.WriteRune('a')
	s.SetScrollArea(1, 3)

	if s.cursor.x != 0 || s.cursor.y != 1 {
		t.Fatalf("Cursor should be set to the top of the scroll region (0,1), but is (%d,%d)", s.cursor.x, s.cursor.y)
	}
}

func TestWriteRune(t *testing.T) {
	t.Run("auto wraps", func(t *testing.T) {
		s := NewScreen(2, 2)
		s.WriteRune('a')
		s.WriteRune('a')
		s.WriteRune('a')
		if s.String() != "aa\na \n" {
			t.Fatalf("the character didn't auto wrap:\n%q", s.String())
		}

	})
}

func TestReverseIndex(t *testing.T) {
	t.Run("auto wraps", func(t *testing.T) {
		s := NewScreen(2, 3)
		s.WriteRune('1')
		s.WriteRune('1')
		s.WriteRune('2')
		s.WriteRune('2')
		s.WriteRune('3')
		s.cursor = cursor{x: 0, y: 0}
		s.ReverseIndex()
		if s.String() != "  \n11\n22\n" {
			t.Fatalf("the screen didn't scroll:\n%q", s.String())
		}
	})

	t.Run("works with scroll region", func(t *testing.T) {
		s := NewScreen(1, 5)
		s.WriteRune('0')
		s.WriteRune('1')
		s.WriteRune('2')
		s.WriteRune('3')
		s.SetScrollArea(1, 3)
		s.ReverseIndex()
		if s.String() != "0\n \n1\n3\n \n" {
			t.Fatalf("the screen didn't scroll:\n%q", s.String())
		}

	})
}
