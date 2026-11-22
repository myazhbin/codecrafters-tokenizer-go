package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// gpt2ByteRunes returns the character GPT-2's vocabulary files use for each
// byte, like bytes_to_unicode in GPT-2's encoder.py. Printable bytes 33-126,
// 161-172, and 174-255 stand for themselves. The other 68 bytes map, in
// increasing order, to U+0100 and up, so byte n below 33 is U+0100 + n.
func gpt2ByteRunes() [256]rune {
	var runes [256]rune
	next := rune(0x100)
	for b := range runes {
		if printable := (b >= 33 && b <= 126) || (b >= 161 && b <= 172) || b >= 174; printable {
			runes[b] = rune(b)
		} else {
			runes[b] = next
			next++
		}
	}
	return runes
}

// loadGPT2Tokenizer builds a Tokenizer from GPT-2's vocab.json and
// merges.txt in dir.
func loadGPT2Tokenizer(dir string) (*Tokenizer, error) {
	var vocab map[string]int
	if err := readJSON(filepath.Join(dir, "vocab.json"), &vocab); err != nil {
		return nil, err
	}
	merges, err := readGPT2Merges(filepath.Join(dir, "merges.txt"))
	if err != nil {
		return nil, err
	}
	return newGPT2Tokenizer(vocab, merges)
}

// newGPT2Tokenizer builds a Tokenizer from GPT-2-style vocabulary data.
// vocab maps every token, spelled with gpt2ByteRunes characters, to its ID.
// merges lists pairs of symbols in rank order, and each pair merges into the
// token spelled by joining the two symbols.
//
// Each symbol is a token with an ID in vocab, so the Tokenizer merges IDs
// rather than strings: pair (vocab[a], vocab[b]) merges into vocab[a+b].
func newGPT2Tokenizer(vocab map[string]int, merges [][2]string) (*Tokenizer, error) {
	var missing []string
	idOf := func(token string) int {
		id, ok := vocab[token]
		if !ok {
			missing = append(missing, token)
		}
		return id
	}

	t := &Tokenizer{rules: make(map[Pair]mergeRule, len(merges))}
	for b, r := range gpt2ByteRunes() {
		t.byteIDs[b] = idOf(string(r))
	}
	for rank, m := range merges {
		pair := Pair{idOf(m[0]), idOf(m[1])}
		t.addMerge(pair, rank, idOf(m[0]+m[1]))
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("vocab.json is missing %d tokens that bytes or merges need, such as %q",
			len(missing), missing[0])
	}
	return t, nil
}

// readGPT2Merges parses GPT-2's merges.txt: a "#version" line, then one
// merge per line in rank order, written as two symbols separated by a space.
// Blank lines are skipped. Real merges such as "# #" also start with "#", so
// only the first line can be the version line.
func readGPT2Merges(path string) ([][2]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var merges [][2]string
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if (i == 0 && strings.HasPrefix(line, "#version")) || strings.TrimSpace(line) == "" {
			continue
		}
		first, second, found := strings.Cut(line, " ")
		if !found || first == "" || second == "" || strings.Contains(second, " ") {
			return nil, fmt.Errorf("%s:%d: %q is not two symbols separated by a space", path, i+1, line)
		}
		merges = append(merges, [2]string{first, second})
	}
	return merges, nil
}
