package main

import (
	"strings"
	"unicode/utf8"
)

// decodeUTF8 decodes buf as UTF-8 text and replaces invalid bytes with
// U+FFFD, the replacement character.
//
// It follows the Unicode Standard's "U+FFFD Substitution of Maximal
// Subparts" (chapter 3.9): each maximal subpart of an ill-formed sequence
// becomes one U+FFFD. A cut-off character such as [240, 159] is one U+FFFD,
// and a byte that can't begin a character, like 255 or a stray continuation
// byte, is one U+FFFD by itself. Ranging over a string gives one U+FFFD per
// invalid byte instead, and strings.ToValidUTF8 gives one per run of them.
func decodeUTF8(buf []byte) string {
	var sb strings.Builder
	sb.Grow(len(buf))
	for len(buf) > 0 {
		r, size := utf8.DecodeRune(buf)
		if r == utf8.RuneError && size == 1 { // size 3 would be a real U+FFFD
			size = maximalSubpart(buf)
		}
		sb.WriteRune(r)
		buf = buf[size:]
	}
	return sb.String()
}

// maximalSubpart returns the length of the longest prefix of buf that could
// begin a well-formed UTF-8 character, or 1 if buf[0] can't begin one. It is
// for a buf that does not begin with a complete, valid character.
func maximalSubpart(buf []byte) int {
	// The allowed range for the byte after the lead byte. Every later
	// continuation byte must be in 0x80-0xBF.
	lo, hi := byte(0x80), byte(0xBF)
	var need int // continuation bytes the lead byte calls for
	switch lead := buf[0]; {
	case lead >= 0xC2 && lead <= 0xDF:
		need = 1
	case lead >= 0xE0 && lead <= 0xEF:
		need = 2
		if lead == 0xE0 {
			lo = 0xA0 // lower values would be overlong encodings
		} else if lead == 0xED {
			hi = 0x9F // higher values would be UTF-16 surrogates
		}
	case lead >= 0xF0 && lead <= 0xF4:
		need = 3
		if lead == 0xF0 {
			lo = 0x90 // lower values would be overlong encodings
		} else if lead == 0xF4 {
			hi = 0x8F // higher values would be above U+10FFFF
		}
	default: // 0x80-0xC1 and 0xF5-0xFF never begin a character
		return 1
	}
	n := 1
	for n <= need && n < len(buf) && buf[n] >= lo && buf[n] <= hi {
		lo, hi = 0x80, 0xBF
		n++
	}
	return n
}
