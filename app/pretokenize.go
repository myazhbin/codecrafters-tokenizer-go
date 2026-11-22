package main

import (
	"fmt"

	"github.com/dlclark/regexp2/v2"
)

// gpt2SplitPattern is the regular expression GPT-2 uses to split text into
// chunks before merging, from tiktoken's r50k_base encoding. In order, the
// alternatives match: contractions like 's and 'll; runs of letters, numbers,
// or other non-space characters, each with at most one leading U+0020 space;
// whitespace at the end of the text; and whitespace not followed by a
// non-space character.
const gpt2SplitPattern = `'(?:[sdmt]|ll|ve|re)| ?\p{L}++| ?\p{N}++| ?[^\s\p{L}\p{N}]++|\s++$|\s+(?!\S)|\s`

// gpt2Split matches GPT-2's chunks. regexp2.None keeps \s matching all
// Unicode whitespace, such as a non-breaking space; regexp2.RE2 would limit
// it to ASCII.
var gpt2Split = regexp2.MustCompile(gpt2SplitPattern, regexp2.None)

// splitGPT2 splits text into chunks the way GPT-2 does before merging. The
// chunks are slices of text: joined in order, they give back text byte for
// byte, including any bytes that aren't valid UTF-8.
func splitGPT2(text []byte) ([][]byte, error) {
	var chunks [][]byte
	m, err := gpt2Split.FindStringMatch(string(text))
	for ; m != nil && err == nil; m, err = gpt2Split.FindNextMatch(m) {
		start, length := m.ByteRange()
		chunks = append(chunks, text[start:start+length])
	}
	if err != nil {
		return nil, fmt.Errorf("splitting text into chunks: %w", err)
	}
	return chunks, nil
}
