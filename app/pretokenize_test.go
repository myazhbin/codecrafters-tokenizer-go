package main

import (
	"bytes"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestSplitGPT2(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"stage example", "Hello world's 123!!", []string{"Hello", " world", "'s", " 123", "!!"}},
		{"punctuation is its own chunk", "dog. dog! dog?", []string{"dog", ".", " dog", "!", " dog", "?"}},
		// Only the last space of a run joins the next word.
		{"run of spaces", "foo   bar", []string{"foo", "  ", " bar"}},
		{"non-breaking space can't join a word", "foo bar", []string{"foo", " ", "bar"}},
		{"trailing whitespace stays together", "hi  \n", []string{"hi", "  \n"}},
		{"trailing spaces", "end   ", []string{"end", "   "}},
		{"newlines are separate chunks", "Hello\n\nworld", []string{"Hello", "\n", "\n", "world"}},
		{"contraction", "don't", []string{"don", "'t"}},
		{"contractions are case sensitive", "DON'T", []string{"DON", "'", "T"}},
		{
			"all contractions",
			"we'll they've you're I'm he'd it's can't",
			[]string{"we", "'ll", " they", "'ve", " you", "'re", " I", "'m", " he", "'d", " it", "'s", " can", "'t"},
		},
		// Devanagari vowel signs are marks, not letters, so they split off.
		{"Hindi", "हिंदी", []string{"ह", "िं", "द", "ी"}},
		{"letters and numbers split", "abc123def", []string{"abc", "123", "def"}},
		{"decimal number", "pi is 3.14", []string{"pi", " is", " 3", ".", "14"}},
		{
			"indented code",
			"def f():\n    return 1\n",
			[]string{"def", " f", "():", "\n   ", " return", " 1", "\n"},
		},
		{"emoji", "hi 👋👋", []string{"hi", " 👋👋"}},
		{"empty text", "", nil},
		{"invalid UTF-8 bytes are kept", "a\xffb", []string{"a", "\xff", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := splitGPT2([]byte(tt.text))
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(chunks))
			for i, c := range chunks {
				got[i] = string(c)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("splitGPT2(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestSplitGPT2KeepsEveryByte(t *testing.T) {
	pieces := []string{
		"a", "Z", "é", "ह", "ि", "7", "٣", " ", "  ", "\n", "\t", "\r\n", " ", "　",
		"'", "'s", "'ll", "!", "...", "👋", "\xff", "\xf0\x9f", "\x80",
	}
	rng := rand.New(rand.NewPCG(3, 4))
	for range 5000 {
		var text []byte
		for range rng.IntN(20) {
			text = append(text, pieces[rng.IntN(len(pieces))]...)
		}
		chunks, err := splitGPT2(text)
		if err != nil {
			t.Fatal(err)
		}
		if joined := bytes.Join(chunks, nil); !bytes.Equal(joined, text) {
			t.Fatalf("chunks of %q join to %q", text, joined)
		}
		for _, c := range chunks {
			if len(c) == 0 {
				t.Fatalf("splitGPT2(%q) produced an empty chunk", text)
			}
		}
	}
}

func TestEncodeChunks(t *testing.T) {
	// "ll" inside "Hello" merges, but "o" and the following space are in
	// different chunks, so (111, 32) never applies.
	tok := newTokenizer([]Pair{{108, 108}, {111, 32}})
	text := []byte("Hello world's 123!!")
	chunks, err := splitGPT2(text)
	if err != nil {
		t.Fatal(err)
	}
	got := tok.encodeChunks(chunks)
	want := []int{72, 101, 256, 111, 32, 119, 111, 114, 108, 100, 39, 115, 32, 49, 50, 51, 33, 33}
	if !slices.Equal(got, want) {
		t.Errorf("with pretokenization: %v, want %v", got, want)
	}

	// Without pretokenization, the text is one chunk and (111, 32) merges.
	got = tok.encodeChunks([][]byte{text})
	want = []int{72, 101, 256, 257, 119, 111, 114, 108, 100, 39, 115, 32, 49, 50, 51, 33, 33}
	if !slices.Equal(got, want) {
		t.Errorf("without pretokenization: %v, want %v", got, want)
	}
}

func TestPretokenizedProgressSentence(t *testing.T) {
	// The five merges learned from the sentence give 11 tokens without
	// splitting. Merge 3 joins "␠the" to the "␠t" of the next word, which
	// splitting puts in a different chunk, so the result is 13 tokens.
	text := []byte("to the top of the tree")
	tok := newTokenizer(train(text, 5))
	chunks, err := splitGPT2(text)
	if err != nil {
		t.Fatal(err)
	}
	got := tok.encodeChunks(chunks)
	want := []int{260, 258, 256, 111, 112, 32, 111, 102, 258, 256, 114, 101, 101}
	if !slices.Equal(got, want) {
		t.Errorf("got %v (%d tokens), want %v (13 tokens)", got, len(got), want)
	}
}
