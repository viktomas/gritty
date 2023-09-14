package main

import "testing"

func FuzzController(f *testing.F) {
	f.Add([]byte{0x00, 0x01, 0x02, 0x04, 0x05, 0x06, 0x07, 0x08})
	f.Fuzz(func(t *testing.T, in []byte) {
		c := &Controller{buffer: NewBuffer(10, 10)}
		ops := NewDecoder().Parse(in)
		for _, op := range ops {
			c.handleOp(op)
		}
	})
}
