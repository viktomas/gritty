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
	s := NewScreen(2, 2)
	s.WriteRune('a')
	s.CR()
	s.LF()
	s.WriteRune('b')
	s.ScrollUp()
	if s.String() != "b \n  \n" {
		t.Fatalf("the a character was supposed to scroll of the screen:\n%q", s.String())
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
		s.CR()
		s.LF()
		s.WriteRune('2')
		s.CR()
		s.LF()
		s.WriteRune('3')
		s.cursor = cursor{x: 0, y: 0}
		s.ReverseIndex()
		if s.String() != "  \n1 \n2 \n" {
			t.Fatalf("the screen didn't scroll:\n%q", s.String())
		}

	})
}
