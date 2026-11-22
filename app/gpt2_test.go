package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGPT2ByteRunes(t *testing.T) {
	runes := gpt2ByteRunes()

	// The table from the stage instructions.
	for b, want := range map[byte]rune{'!': '!', 'a': 'a', '\t': 'ĉ', '\n': 'Ċ', ' ': 'Ġ'} {
		if runes[b] != want {
			t.Errorf("byte %d maps to %U, want %U", b, runes[b], want)
		}
	}

	// Printable bytes map to themselves; the other 68 map to U+0100-U+0143
	// in increasing byte order.
	next := rune(0x100)
	for b, r := range runes {
		printable := (b >= 33 && b <= 126) || (b >= 161 && b <= 172) || b >= 174
		switch {
		case printable && r != rune(b):
			t.Errorf("printable byte %d maps to %U, want itself", b, r)
		case !printable && r != next:
			t.Errorf("byte %d maps to %U, want %U", b, r, next)
		case !printable:
			next++
		}
	}
	if next != 0x144 {
		t.Errorf("%d bytes were remapped, want 68", next-0x100)
	}
	if runes[127] != 0x121 || runes[173] != 0x143 {
		t.Errorf("bytes 127 and 173 map to %U and %U, want U+0121 and U+0143", runes[127], runes[173])
	}

	seen := make(map[rune]bool)
	for _, r := range runes {
		if seen[r] {
			t.Errorf("two bytes map to %U", r)
		}
		seen[r] = true
	}
}

func TestReadGPT2Merges(t *testing.T) {
	want := [][2]string{{"Ġ", "t"}, {"Ġ", "a"}, {"#", "#"}, {"h", "e"}}
	tests := []struct {
		name    string
		content string
		want    [][2]string
	}{
		{"GPT-2's format", "#version: 0.2\nĠ t\nĠ a\n\n# #\nh e\n", want},
		{"Windows line endings", "#version: 0.2\r\nĠ t\r\nĠ a\r\n\r\n# #\r\nh e\r\n", want},
		{"no trailing newline", "#version: 0.2\nĠ t\nĠ a\n# #\nh e", want},
		{"no version line", "Ġ t\nĠ a\n# #\nh e\n", want},
		{"empty file", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readGPT2Merges(writeTempFile(t, "merges.txt", tt.content))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}

	for _, bad := range []string{"#version: 0.2\nĠt\n", "#version: 0.2\na b c\n", "#version: 0.2\n a\n"} {
		if _, err := readGPT2Merges(writeTempFile(t, "merges.txt", bad)); err == nil {
			t.Errorf("readGPT2Merges(%q) succeeded, want an error", bad)
		}
	}
}

// testGPT2Vocab returns a GPT-2-style vocabulary: the 256 byte tokens,
// numbered like GPT-2's (in code point order of their characters, which puts
// printable bytes first), then one token per merge, numbered from 256.
func testGPT2Vocab(merges [][2]string) map[string]int {
	runes := gpt2ByteRunes()
	sorted := slices.Clone(runes[:])
	slices.Sort(sorted)
	vocab := make(map[string]int, 256+len(merges))
	for id, r := range sorted {
		vocab[string(r)] = id
	}
	for i, m := range merges {
		vocab[m[0]+m[1]] = firstMergeID + i
	}
	return vocab
}

func TestGPT2TokenizerSmallVocab(t *testing.T) {
	merges := [][2]string{{"Ġ", "t"}, {"h", "e"}, {"Ġt", "he"}}
	vocab := testGPT2Vocab(merges)

	// The helper numbers byte tokens the way GPT-2 does.
	for token, want := range map[string]int{"!": 0, "a": 64, "ĉ": 197, "Ċ": 198, "Ġ": 220, "t": 83} {
		if vocab[token] != want {
			t.Fatalf("test vocab gives %q ID %d, want %d", token, vocab[token], want)
		}
	}

	tok, err := newGPT2Tokenizer(vocab, merges)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		text string
		want []int
	}{
		{"the the", []int{83, 257, 258}},    // "t", "he", "Ġthe"
		{" the  the", []int{258, 220, 258}}, // "Ġthe", "Ġ", "Ġthe"
		{"the\n\nthe", []int{83, 257, 198, 198, 83, 257}},
		{"\t\n !", []int{197, 198, 220, 0}}, // every token is a single byte
		{"", []int{}},
	}
	for _, tt := range tests {
		chunks, err := splitGPT2([]byte(tt.text))
		if err != nil {
			t.Fatal(err)
		}
		if got := tok.encodeChunks(chunks); !slices.Equal(got, tt.want) {
			t.Errorf("encoding %q gave %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestNewGPT2TokenizerMissingToken(t *testing.T) {
	merges := [][2]string{{"Ġ", "t"}, {"h", "e"}, {"Ġt", "he"}}
	for _, token := range []string{"Ġthe", "Ġ", "he"} {
		vocab := testGPT2Vocab(merges)
		delete(vocab, token)
		_, err := newGPT2Tokenizer(vocab, merges)
		if err == nil || !strings.Contains(err.Error(), "vocab.json is missing") {
			t.Errorf("without %q: error = %v, want a missing-token error", token, err)
		}
	}
}

// writeGPT2Files writes vocab.json and merges.txt for merges into a new
// directory and returns the directory.
func writeGPT2Files(t *testing.T, merges [][2]string) string {
	t.Helper()
	dir := t.TempDir()
	vocab, err := json.Marshal(testGPT2Vocab(merges))
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{"#version: 0.2"}
	for _, m := range merges {
		lines = append(lines, m[0]+" "+m[1])
	}
	for name, content := range map[string]string{
		"vocab.json": string(vocab),
		"merges.txt": strings.Join(lines, "\n") + "\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestEncodeCommandGPT2Vocab(t *testing.T) {
	text := writeTempFile(t, "text.txt", "the the")

	dir := writeGPT2Files(t, [][2]string{{"Ġ", "t"}, {"h", "e"}, {"Ġt", "he"}})
	if got, want := runCLI(t, "encode", text, "--vocab", dir), "[83, 257, 258]\n"; got != want {
		t.Errorf("encode --vocab printed %q, want %q", got, want)
	}

	// --vocab always splits like GPT-2: "e" and the next "Ġ" are in
	// different chunks, so the merge (e, Ġ) can't apply.
	dir = writeGPT2Files(t, [][2]string{{"e", "Ġ"}})
	if got, want := runCLI(t, "encode", text, "--vocab", dir), "[83, 71, 68, 220, 83, 71, 68]\n"; got != want {
		t.Errorf("encode --vocab printed %q, want %q", got, want)
	}
}

// TestGPT2RealVocab checks exact GPT-2 token IDs with GPT-2's own files. It
// needs vocab.json and merges.txt from https://huggingface.co/gpt2 in the
// directory named by GPT2_VOCAB_DIR, and is skipped otherwise.
//
// The first five cases come from the stage instructions; the rest were
// checked against Hugging Face's tokenizers library on the same files.
func TestGPT2RealVocab(t *testing.T) {
	dir := os.Getenv("GPT2_VOCAB_DIR")
	if dir == "" {
		t.Skip("set GPT2_VOCAB_DIR to a directory holding GPT-2's vocab.json and merges.txt")
	}
	tests := []struct {
		name string
		text string
		want string
	}{
		{"stage example", "Hello world", "[15496, 995]"},
		{"leading space", " Hello world", "[18435, 995]"},
		{"strawberry", "strawberry strawberry", "[301, 1831, 8396, 41236]"},
		{"newlines stay separate", "Hello\n\nworld", "[15496, 198, 198, 6894]"},
		{"progress sentence", "to the top of the tree", "[1462, 262, 1353, 286, 262, 5509]"},
		{"emoji split across tokens", "👋", "[41840, 233]"},
		{"emoji after a space", "hi 👋", "[5303, 50169, 233]"},
		{"contractions", "DON'T don't", "[41173, 6, 51, 836, 470]"},
		{"non-breaking space", "foo bar", "[21943, 1849, 5657]"},
		{
			"Hindi sentence",
			"मुझे हिंदी में लिखना पसंद है।",
			"[11976, 106, 24231, 223, 11976, 251, 24231, 229, 28225, 117, 11976, 123, 11976, 224, 11976, 99, " +
				"24231, 222, 28225, 106, 24231, 229, 11976, 224, 28225, 110, 11976, 123, 11976, 244, 11976, 101, " +
				"48077, 28225, 103, 11976, 116, 11976, 224, 11976, 99, 28225, 117, 24231, 230, 24231, 97]",
		},
		{
			"indented code",
			"def greet(name):\n    if name:\n        return f\"Hello, {name}!\"\n    return None\n",
			"[4299, 12589, 7, 3672, 2599, 198, 220, 220, 220, 611, 1438, 25, 198, 220, 220, 220, 220, 220, 220, " +
				"220, 1441, 277, 1, 15496, 11, 1391, 3672, 92, 2474, 198, 220, 220, 220, 1441, 6045, 198]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := writeTempFile(t, "text.txt", tt.text)
			if got := runCLI(t, "encode", text, "--vocab", dir); got != tt.want+"\n" {
				t.Errorf("encode %q printed %s, want %s", tt.text, strings.TrimSpace(got), tt.want)
			}
		})
	}
}
