package main

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDecodeUTF8(t *testing.T) {
	const r = "�"
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"ASCII", []byte("aaabd"), "aaabd"},
		{"empty", nil, ""},
		{"full wave emoji", []byte{240, 159, 145, 139}, "👋"},
		{"half wave emoji is one replacement", []byte{240, 159, 145}, r},
		{"two bytes of a four-byte character", []byte{240, 159}, r},
		{"bytes that never begin a character", []byte{255, 255}, r + r},
		{"stray continuation byte", []byte{139}, r},
		{"cut-off character between ASCII", []byte{'a', 0xF0, 0x9F, 0x91, 'b'}, "a" + r + "b"},
		{"next character begins at the byte that didn't fit", []byte{0xF0, 0x9F, 0x91, 0xF0, 0x9F, 0x91, 0x8B}, r + "👋"},
		{"two-byte lead followed by ASCII", []byte{0xC3, '('}, r + "("},
		{"cut-off three-byte character", []byte{0xE2, 0x82}, r},
		{"overlong two-byte encoding", []byte{0xC0, 0xAF}, r + r},
		{"overlong three-byte encoding", []byte{0xE0, 0x80, 0xAF}, r + r + r},
		{"overlong four-byte encoding", []byte{0xF0, 0x80, 0x80, 0xAF}, r + r + r + r},
		{"UTF-16 surrogate", []byte{0xED, 0xA0, 0x80}, r + r + r},
		{"above U+10FFFF", []byte{0xF4, 0x90, 0x80, 0x80}, r + r + r + r},
		{"F5 and above", []byte{0xF5, 0xFE, 0xFF}, r + r + r},
		{"a real U+FFFD stays one character", []byte("a�b"), "a" + r + "b"},
		{"mixed scripts", []byte("naïve café हिंदी 🙂"), "naïve café हिंदी 🙂"},
		{
			// Table 3-8 in chapter 3.9 of the Unicode Standard.
			"Unicode Standard example",
			[]byte{0x61, 0xF1, 0x80, 0x80, 0xE1, 0x80, 0xC2, 0x62, 0x80, 0x63, 0x80, 0xBF, 0x64},
			"a" + r + r + r + "b" + r + "c" + r + r + "d",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeUTF8(tt.in); got != tt.want {
				t.Errorf("decodeUTF8(% x) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecodeUTF8MatchesReference(t *testing.T) {
	// Mostly bytes at the edges of UTF-8's ranges, plus some random ones.
	edges := []byte{
		0x00, 0x41, 0x7F, 0x80, 0x8F, 0x90, 0x9F, 0xA0, 0xBF, 0xC0, 0xC1, 0xC2,
		0xDF, 0xE0, 0xE1, 0xEC, 0xED, 0xEE, 0xEF, 0xF0, 0xF1, 0xF3, 0xF4, 0xF5, 0xFF,
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for range 50000 {
		buf := make([]byte, rng.IntN(12))
		for i := range buf {
			if rng.IntN(4) == 0 {
				buf[i] = byte(rng.IntN(256))
			} else {
				buf[i] = edges[rng.IntN(len(edges))]
			}
		}
		got := decodeUTF8(buf)
		if want := whatwgDecode(buf); got != want {
			t.Fatalf("decodeUTF8(% x) = %q, want %q", buf, got, want)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("decodeUTF8(% x) = %q is not valid UTF-8", buf, got)
		}
		if utf8.Valid(buf) && got != string(buf) {
			t.Fatalf("decodeUTF8 changed valid UTF-8 % x into %q", buf, got)
		}
	}
}

// whatwgDecode is an independent reference: a literal transcription of the
// UTF-8 decoder state machine in the WHATWG Encoding Standard, which also
// replaces each maximal subpart of an invalid sequence with one U+FFFD.
func whatwgDecode(buf []byte) string {
	var sb strings.Builder
	var codePoint rune
	needed, seen := 0, 0
	lower, upper := byte(0x80), byte(0xBF)
	for i := 0; i < len(buf); i++ {
		b := buf[i]
		if needed == 0 {
			switch {
			case b <= 0x7F:
				sb.WriteByte(b)
			case b >= 0xC2 && b <= 0xDF:
				needed, codePoint = 1, rune(b&0x1F)
			case b >= 0xE0 && b <= 0xEF:
				if b == 0xE0 {
					lower = 0xA0
				}
				if b == 0xED {
					upper = 0x9F
				}
				needed, codePoint = 2, rune(b&0xF)
			case b >= 0xF0 && b <= 0xF4:
				if b == 0xF0 {
					lower = 0x90
				}
				if b == 0xF4 {
					upper = 0x8F
				}
				needed, codePoint = 3, rune(b&0x7)
			default:
				sb.WriteRune(utf8.RuneError)
			}
			continue
		}
		if b < lower || b > upper {
			codePoint, needed, seen = 0, 0, 0
			lower, upper = 0x80, 0xBF
			sb.WriteRune(utf8.RuneError)
			i-- // reprocess this byte as the start of a new character
			continue
		}
		lower, upper = 0x80, 0xBF
		codePoint = codePoint<<6 | rune(b&0x3F)
		seen++
		if seen == needed {
			sb.WriteRune(codePoint)
			codePoint, needed, seen = 0, 0, 0
		}
	}
	if needed != 0 {
		sb.WriteRune(utf8.RuneError)
	}
	return sb.String()
}
