package buffer

import "fmt"

// FIXME: Ideally, the buffer would represent the colors the same way that the
// SGR instruction does and the GUI layer would then translate them
// that would allow for easy changes to the color theme
var (
	DefaultFG = NewColor(0xeb, 0xdb, 0xb2)
	DefaultBG = NewColor(0x28, 0x28, 0x28)
)

type Color struct {
	R uint8
	G uint8
	B uint8
}

func NewColor(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b}
}

func (c Color) String() string {
	return fmt.Sprintf("#%x%x%x", c.R, c.G, c.B)
}
