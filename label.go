// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/viktomas/gritty/buffer"

	"golang.org/x/image/math/fixed"
)

var (
	defaultFG = color.NRGBA{A: 0xff, R: 0xeb, G: 0xdb, B: 0xb2}
	defaultBG = color.NRGBA{A: 0xff, R: 0x28, G: 0x28, B: 0x28}
)

// Label is a widget code copied from https://git.sr.ht/~eliasnaur/gio/tree/313c488ec356872a14dab0c0ac0fd73b45a596cf/item/widget/label.go
// and changed so it can render grid of characters. Each character can have
// different background and foreground colors and a few other attributes
// see (buffer.BrushedRune)
type Label struct {
	// Alignment specifies the text alignment.
	Alignment text.Alignment
	// MaxLines limits the number of lines. Zero means no limit.
	MaxLines int
	// Truncator is the text that will be shown at the end of the final
	// line if MaxLines is exceeded. Defaults to "…" if empty.
	Truncator string
	// WrapPolicy configures how displayed text will be broken into lines.
	WrapPolicy text.WrapPolicy
	// LineHeight controls the distance between the baselines of lines of text.
	// If zero, a sensible default will be used.
	LineHeight unit.Sp
	// LineHeightScale applies a scaling factor to the LineHeight. If zero, a
	// sensible default will be used.
	LineHeightScale float32
}

type paintedGlyph struct {
	g      text.Glyph
	r      rune
	fg, bg color.NRGBA
}

// Layout the label with the given shaper, font, size, text, and material.
func (l Label) Layout(gtx layout.Context, lt *text.Shaper, font font.Font, size unit.Sp, txt []buffer.BrushedRune) layout.Dimensions {
	dims, _ := l.LayoutDetailed(gtx, lt, font, size, txt)
	return dims
}

// TextInfo provides metadata about shaped text.
type TextInfo struct {
	// Truncated contains the number of runes of text that are represented by a truncator
	// symbol in the text. If zero, there is no truncator symbol.
	Truncated int
}

// Layout the label with the given shaper, font, size, text, and material, returning metadata about the shaped text.
func (l Label) LayoutDetailed(gtx layout.Context, lt *text.Shaper, font font.Font, size unit.Sp, txt []buffer.BrushedRune) (layout.Dimensions, TextInfo) {
	cs := gtx.Constraints
	textSize := fixed.I(gtx.Sp(size))
	lineHeight := fixed.I(gtx.Sp(l.LineHeight))
	var str strings.Builder
	for _, pr := range txt {
		str.WriteRune(pr.R)
	}
	lt.LayoutString(text.Parameters{
		Font:            font,
		PxPerEm:         textSize,
		MaxLines:        l.MaxLines,
		Truncator:       l.Truncator,
		Alignment:       l.Alignment,
		WrapPolicy:      l.WrapPolicy,
		MaxWidth:        cs.Max.X,
		MinWidth:        cs.Min.X,
		Locale:          gtx.Locale,
		LineHeight:      lineHeight,
		LineHeightScale: l.LineHeightScale,
	}, str.String())
	m := op.Record(gtx.Ops)
	viewport := image.Rectangle{Max: cs.Max}
	it := textIterator{
		viewport: viewport,
		maxLines: l.MaxLines,
	}
	semantic.LabelOp(str.String()).Add(gtx.Ops)
	var paintedGlyphs [32]paintedGlyph
	line := paintedGlyphs[:0]
	pos := 0
	for g, ok := lt.NextGlyph(); ok; g, ok = lt.NextGlyph() {
		// if txt[pos].r == '\n' {
		// 	pos++
		// }
		var ok bool
		if line, ok = it.paintGlyph(gtx, lt, g, line, txt[pos]); !ok {
			break
		}
		if pos+1 >= len(txt) {
			// log.Printf("incrementing pos will cause problems, txt length is %v, position is %v", len(txt), pos)
		} else {
			pos++
		}
	}
	call := m.Stop()
	viewport.Min = viewport.Min.Add(it.padding.Min)
	viewport.Max = viewport.Max.Add(it.padding.Max)
	clipStack := clip.Rect(viewport).Push(gtx.Ops)
	call.Add(gtx.Ops)
	dims := layout.Dimensions{Size: it.bounds.Size()}
	dims.Size = cs.Constrain(dims.Size)
	dims.Baseline = dims.Size.Y - it.baseline
	clipStack.Pop()
	return dims, TextInfo{Truncated: it.truncated}
}

func r2p(r clip.Rect) clip.Op {
	return clip.Stroke{Path: r.Path(), Width: 1}.Op()
}

// textIterator computes the bounding box of and paints text.
type textIterator struct {
	// viewport is the rectangle of document coordinates that the iterator is
	// trying to fill with text.
	viewport image.Rectangle
	// maxLines is the maximum number of text lines that should be displayed.
	maxLines int
	// truncated tracks the count of truncated runes in the text.
	truncated int
	// linesSeen tracks the quantity of line endings this iterator has seen.
	linesSeen int
	// lineOff tracks the origin for the glyphs in the current line.
	lineOff f32.Point
	// padding is the space needed outside of the bounds of the text to ensure no
	// part of a glyph is clipped.
	padding image.Rectangle
	// bounds is the logical bounding box of the text.
	bounds image.Rectangle
	// visible tracks whether the most recently iterated glyph is visible within
	// the viewport.
	visible bool
	// first tracks whether the iterator has processed a glyph yet.
	first bool
	// baseline tracks the location of the first line of text's baseline.
	baseline int
}

// processGlyph checks whether the glyph is visible within the iterator's configured
// viewport and (if so) updates the iterator's text dimensions to include the glyph.
func (it *textIterator) processGlyph(g text.Glyph, ok bool) (_ text.Glyph, visibleOrBefore bool) {
	if it.maxLines > 0 {
		if g.Flags&text.FlagTruncator != 0 && g.Flags&text.FlagClusterBreak != 0 {
			// A glyph carrying both of these flags provides the count of truncated runes.
			it.truncated = g.Runes
		}
		if g.Flags&text.FlagLineBreak != 0 {
			it.linesSeen++
		}
		if it.linesSeen == it.maxLines && g.Flags&text.FlagParagraphBreak != 0 {
			return g, false
		}
	}
	// Compute the maximum extent to which glyphs overhang on the horizontal
	// axis.
	if d := g.Bounds.Min.X.Floor(); d < it.padding.Min.X {
		// If the distance between the dot and the left edge of this glyph is
		// less than the current padding, increase the left padding.
		it.padding.Min.X = d
	}
	if d := (g.Bounds.Max.X - g.Advance).Ceil(); d > it.padding.Max.X {
		// If the distance between the dot and the right edge of this glyph
		// minus the logical advance of this glyph is greater than the current
		// padding, increase the right padding.
		it.padding.Max.X = d
	}
	if d := (g.Bounds.Min.Y + g.Ascent).Floor(); d < it.padding.Min.Y {
		// If the distance between the dot and the top of this glyph is greater
		// than the ascent of the glyph, increase the top padding.
		it.padding.Min.Y = d
	}
	if d := (g.Bounds.Max.Y - g.Descent).Ceil(); d > it.padding.Max.Y {
		// If the distance between the dot and the bottom of this glyph is greater
		// than the descent of the glyph, increase the bottom padding.
		it.padding.Max.Y = d
	}
	logicalBounds := image.Rectangle{
		Min: image.Pt(g.X.Floor(), int(g.Y)-g.Ascent.Ceil()),
		Max: image.Pt((g.X + g.Advance).Ceil(), int(g.Y)+g.Descent.Ceil()),
	}
	if !it.first {
		it.first = true
		it.baseline = int(g.Y)
		it.bounds = logicalBounds
	}

	above := logicalBounds.Max.Y < it.viewport.Min.Y
	below := logicalBounds.Min.Y > it.viewport.Max.Y
	left := logicalBounds.Max.X < it.viewport.Min.X
	right := logicalBounds.Min.X > it.viewport.Max.X
	it.visible = !above && !below && !left && !right
	if it.visible {
		it.bounds.Min.X = min(it.bounds.Min.X, logicalBounds.Min.X)
		it.bounds.Min.Y = min(it.bounds.Min.Y, logicalBounds.Min.Y)
		it.bounds.Max.X = max(it.bounds.Max.X, logicalBounds.Max.X)
		it.bounds.Max.Y = max(it.bounds.Max.Y, logicalBounds.Max.Y)
	}
	return g, ok && !below
}

func fixedToFloat(i fixed.Int26_6) float32 {
	return float32(i) / 64.0
}

func shouldBlinkInvert() bool {
	currentTime := time.Now()
	return (currentTime.UnixNano()/int64(time.Millisecond)/500)%2 == 0
}

// toPaintedGlyph transfers GUI-agnostic BrushedRune into a specific way
// we render the characters in Gio
func toPaintedGlyph(g text.Glyph, br buffer.BrushedRune) paintedGlyph {
	defaultGlyph := paintedGlyph{
		r:  br.R,
		g:  g,
		fg: defaultFG,
		bg: defaultBG,
	}

	if br.Brush.FG.A != 0 {
		defaultGlyph.fg = br.Brush.FG
	}
	if br.Brush.BG.A != 0 {
		defaultGlyph.bg = br.Brush.BG
	}

	if br.Brush.Bold {
		defaultGlyph.bg = color.NRGBA{A: 255, R: 0, G: 0, B: 0}
	}

	if br.Brush.Invert {
		fg := defaultGlyph.fg
		defaultGlyph.fg = defaultGlyph.bg
		defaultGlyph.bg = fg
	}

	if br.Brush.Blink && shouldBlinkInvert() {
		fg := defaultGlyph.fg
		defaultGlyph.fg = defaultGlyph.bg
		defaultGlyph.bg = fg
	}

	return defaultGlyph
}

// paintGlyph buffers up and paints text glyphs. It should be invoked iteratively upon each glyph
// until it returns false. The line parameter should be a slice with
// a backing array of sufficient size to buffer multiple glyphs.
// A modified slice will be returned with each invocation, and is
// expected to be passed back in on the following invocation.
// This design is awkward, but prevents the line slice from escaping
// to the heap.
//
// this function has been heavily modified from the original in
// https://git.sr.ht/~eliasnaur/gio/tree/313c488ec356872a14dab0c0ac0fd73b45a596cf/item/widget/label.go
// to render grid of characters where each character can have a different color
func (it *textIterator) paintGlyph(gtx layout.Context, shaper *text.Shaper, glyph text.Glyph, line []paintedGlyph, br buffer.BrushedRune) ([]paintedGlyph, bool) {
	_, visibleOrBefore := it.processGlyph(glyph, true)
	if it.visible {
		if len(line) == 0 {
			it.lineOff = f32.Point{X: fixedToFloat(glyph.X), Y: float32(glyph.Y)}.Sub(layout.FPt(it.viewport.Min))
		}

		// we processed the glyph and now we take parameters from the brushed rune
		// these parameters are then used in the next step (after we processed the whole line)
		line = append(line, toPaintedGlyph(glyph, br))
	}
	// this section gets executed only at the end, after we filled our line with glyphs
	// by repeatedly calling the it.ProcessGlyph
	if glyph.Flags&text.FlagLineBreak != 0 || cap(line)-len(line) == 0 || !visibleOrBefore {
		t := op.Affine(f32.Affine2D{}.Offset(it.lineOff)).Push(gtx.Ops)
		var glyphLine []text.Glyph
		for _, pg := range line {
			// minX is where the glyph character starts
			// thanks to setting an offset, the rectangle and the glyph can be drawn from X: 0
			minX := pg.g.X.Floor() - it.lineOff.Round().X
			glyphOffset := op.Affine(f32.Affine2D{}.Offset(f32.Point{X: float32(minX)})).Push(gtx.Ops)

			// draw background
			rect := clip.Rect{
				Min: image.Point{X: 0, Y: 0 - glyph.Ascent.Ceil()},
				Max: image.Point{X: pg.g.X.Floor() + pg.g.Advance.Ceil() - it.lineOff.Round().X, Y: 0 + glyph.Descent.Ceil()},
			}
			paint.FillShape(
				gtx.Ops,
				pg.bg,
				rect.Op(),
			)

			// draw glyph
			path := shaper.Shape([]text.Glyph{pg.g})
			outline := clip.Outline{Path: path}.Op().Push(gtx.Ops)
			paint.ColorOp{Color: pg.fg}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			outline.Pop()
			if call := shaper.Bitmaps(glyphLine); call != (op.CallOp{}) {
				call.Add(gtx.Ops)
			}

			glyphOffset.Pop()
		}
		t.Pop()
		line = line[:0]
	}
	return line, visibleOrBefore
}
