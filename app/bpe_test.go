package main

import (
	"slices"
	"strings"
	"testing"
)

func TestMostFrequentPair(t *testing.T) {
	tests := []struct {
		name string
		ids  []int
		want Pair
	}{
		{"stage example", []int{1, 2, 3, 1, 2}, Pair{1, 2}},
		{"all tie, first wins", []int{8, 9, 1, 2}, Pair{8, 9}},
		{"two-way tie, first wins", []int{1, 2, 3, 4, 3, 4, 1, 2}, Pair{1, 2}},
		{"tie goes to first occurrence, not last", []int{3, 4, 1, 2, 1, 2, 3, 4}, Pair{3, 4}},
		{"later pair is more frequent", []int{5, 6, 7, 8, 7, 8}, Pair{7, 8}},
		// Overlapping pairs count: (97, 97) appears twice and beats (5, 6).
		{"overlapping pairs count twice", []int{5, 6, 97, 97, 97}, Pair{97, 97}},
		{"long overlapping run", []int{1, 2, 1, 2, 3, 3, 3, 3}, Pair{3, 3}},
		{"aaabdaaabac", bytesToIDs([]byte("aaabdaaabac")), Pair{97, 97}},
		{"IDs above 255", []int{256, 300, 256, 300, 1}, Pair{256, 300}},
		{"exactly two IDs", []int{7, 7}, Pair{7, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := mostFrequentPair(tt.ids)
			if !ok || got != tt.want {
				t.Errorf("mostFrequentPair(%v) = %v, %v; want %v, true", tt.ids, got, ok, tt.want)
			}
		})
	}
}

func TestMostFrequentPairNeedsTwoIDs(t *testing.T) {
	for _, ids := range [][]int{nil, {}, {42}} {
		if got, ok := mostFrequentPair(ids); ok {
			t.Errorf("mostFrequentPair(%v) = %v, true; want ok = false", ids, got)
		}
	}
}

func TestMergePair(t *testing.T) {
	tests := []struct {
		name string
		ids  []int
		pair Pair
		want []int
	}{
		{"stage example", []int{1, 2, 3, 1, 2}, Pair{1, 2}, []int{256, 3, 256}},
		{"matches can't overlap", []int{97, 97, 97}, Pair{97, 97}, []int{256, 97}},
		{"back-to-back matches", []int{97, 97, 97, 97}, Pair{97, 97}, []int{256, 256}},
		{"pair never appears", []int{1, 2, 3}, Pair{4, 5}, []int{1, 2, 3}},
		{"order matters", []int{1, 2, 3}, Pair{2, 1}, []int{1, 2, 3}},
		{"match after a near miss", []int{2, 1, 2}, Pair{1, 2}, []int{2, 256}},
		{"match at the end", []int{3, 1, 2}, Pair{1, 2}, []int{3, 256}},
		{"empty list", []int{}, Pair{1, 2}, []int{}},
		{"single ID", []int{1}, Pair{1, 2}, []int{1}},
		{
			"aaabdaaabac",
			bytesToIDs([]byte("aaabdaaabac")),
			Pair{97, 97},
			[]int{256, 97, 98, 100, 256, 97, 98, 97, 99},
		},
		{
			// GPT-2's first merge, a space followed by "t", turns 22 bytes into 18 tokens.
			"progress sentence",
			bytesToIDs([]byte("to the top of the tree")),
			Pair{32, 116},
			[]int{116, 111, 256, 104, 101, 256, 111, 112, 32, 111, 102, 256, 104, 101, 256, 114, 101, 101},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := slices.Clone(tt.ids)
			got := mergePair(tt.ids, tt.pair, firstMergeID)
			if !slices.Equal(got, tt.want) {
				t.Errorf("mergePair(%v, %v) = %v, want %v", tt.ids, tt.pair, got, tt.want)
			}
			if !slices.Equal(tt.ids, before) {
				t.Errorf("mergePair modified its input: %v, was %v", tt.ids, before)
			}
		})
	}
}

// replayMerges applies merges to ids in training order: merge i creates ID
// 256 + i wherever its pair appears at that point.
func replayMerges(ids []int, merges []Pair) []int {
	for i, p := range merges {
		ids = mergePair(ids, p, firstMergeID+i)
	}
	return ids
}

func TestTrain(t *testing.T) {
	sentence := "to the top of the tree"
	tests := []struct {
		name      string
		text      string
		numMerges int
		want      []Pair
		wantIDs   []int // the training text's IDs after all merges
	}{
		{
			name:      "stage example",
			text:      "aaabdaaabac",
			numMerges: 3,
			want:      []Pair{{97, 97}, {256, 97}, {257, 98}},
			wantIDs:   []int{258, 100, 258, 97, 99},
		},
		{
			// Rounds 4-6 each find only pairs that appear once, so the first wins.
			name:      "keeps going on pairs that appear once",
			text:      "aaabdaaabac",
			numMerges: 6,
			want:      []Pair{{97, 97}, {256, 97}, {257, 98}, {258, 100}, {259, 258}, {260, 97}},
			wantIDs:   []int{261, 99},
		},
		{
			// Merge 2 is a tie between (256, 104), (104, 101), and (101, 256);
			// merge 4 joins "␠the" to the "␠t" of the next word.
			name:      "progress sentence",
			text:      sentence,
			numMerges: 5,
			want:      []Pair{{32, 116}, {256, 104}, {257, 101}, {258, 256}, {116, 111}},
			wantIDs:   []int{260, 259, 111, 112, 32, 111, 102, 259, 114, 101, 101},
		},
		{
			name:      "spaces and newlines are ordinary bytes",
			text:      "ab ab\nab ab\n",
			numMerges: 2,
			want:      []Pair{{97, 98}, {256, 32}},
			wantIDs:   []int{257, 256, 10, 257, 256, 10},
		},
		{
			name:      "zero merges",
			text:      "aaabdaaabac",
			numMerges: 0,
			want:      []Pair{},
			wantIDs:   bytesToIDs([]byte("aaabdaaabac")),
		},
		{
			name:      "stops when one token is left",
			text:      "abc",
			numMerges: 10,
			want:      []Pair{{97, 98}, {256, 99}},
			wantIDs:   []int{257},
		},
		{
			name:      "empty text",
			text:      "",
			numMerges: 3,
			want:      []Pair{},
			wantIDs:   []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := train([]byte(tt.text), tt.numMerges)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("train(%q, %d) = %v, want %v", tt.text, tt.numMerges, got, tt.want)
			}
			ids := replayMerges(bytesToIDs([]byte(tt.text)), got)
			if !slices.Equal(ids, tt.wantIDs) {
				t.Errorf("replaying the merges gives %v, want %v", ids, tt.wantIDs)
			}
		})
	}
}

func TestTrainProgressSentenceTokenCount(t *testing.T) {
	text := []byte("to the top of the tree")
	if len(text) != 22 {
		t.Fatalf("sentence has %d bytes, want 22", len(text))
	}
	if n := len(replayMerges(bytesToIDs(text), train(text, 5))); n != 11 {
		t.Errorf("after 5 merges the sentence is %d tokens, want 11", n)
	}
}

func TestEncode(t *testing.T) {
	aaabMerges := []Pair{{97, 97}, {256, 97}, {257, 98}}
	competing := []Pair{{98, 97}, {97, 98}}
	sentence := "to the top of the tree"
	tests := []struct {
		name   string
		text   string
		merges []Pair
		want   []int
	}{
		{"stage example", "aaab", aaabMerges, []int{258}},
		{"training text", "aaabdaaabac", aaabMerges, []int{258, 100, 258, 97, 99}},
		// (98, 97) is less frequent than (97, 98) but has the earlier rank.
		{"rank beats frequency", "ababab", competing, []int{97, 256, 256, 98}},
		{"competing merges on ababa", "ababa", competing, []int{97, 256, 256}},
		{"empty text", "", aaabMerges, []int{}},
		{"no merges", "abc", nil, []int{97, 98, 99}},
		{"no merge applies", "xyz", aaabMerges, []int{120, 121, 122}},
		{"single byte", "a", aaabMerges, []int{97}},
		{"multi-byte characters", "👋👋", []Pair{{240, 159}, {256, 145}, {257, 139}}, []int{258, 258}},
		{
			"progress sentence with its own merges",
			sentence,
			train([]byte(sentence), 5),
			[]int{260, 259, 111, 112, 32, 111, 102, 259, 114, 101, 101},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newTokenizer(tt.merges).encode([]byte(tt.text))
			if !slices.Equal(got, tt.want) {
				t.Errorf("encode(%q) with merges %v = %v, want %v", tt.text, tt.merges, got, tt.want)
			}
		})
	}
}

func TestEncodeMatchesTraining(t *testing.T) {
	// Encoding the training text with its own merges reproduces the IDs that
	// training ended with.
	texts := []string{
		"aaabdaaabac",
		"to the top of the tree",
		"hello hello hello world\nhello, world!\n",
		"🙂 naïve café 🙂 naïve café",
	}
	for _, text := range texts {
		for _, n := range []int{1, 3, 10, 25} {
			merges := train([]byte(text), n)
			want := replayMerges(bytesToIDs([]byte(text)), merges)
			got := newTokenizer(merges).encode([]byte(text))
			if !slices.Equal(got, want) {
				t.Errorf("text %q, %d merges: encode = %v, training ended with %v", text, n, got, want)
			}
		}
	}
}

func TestDecodeBytes(t *testing.T) {
	aaab := []Pair{{97, 97}, {256, 97}, {257, 98}}
	tests := []struct {
		name   string
		ids    []int
		merges []Pair
		want   string
	}{
		{"stage example", []int{258, 100}, aaab, "aaabd"},
		{"bytes only", []int{104, 105}, nil, "hi"},
		{"no IDs", nil, aaab, ""},
		{"character split across tokens", []int{257, 139}, []Pair{{240, 159}, {256, 145}}, "👋"},
		{"merge that refers to a later merge", []int{256}, []Pair{{97, 257}, {98, 99}}, "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeBytes(tt.ids, tt.merges)
			if err != nil || string(got) != tt.want {
				t.Errorf("decodeBytes(%v) = %q, %v; want %q", tt.ids, got, err, tt.want)
			}
		})
	}
}

func TestDecodeBytesErrors(t *testing.T) {
	aaab := []Pair{{97, 97}, {256, 97}, {257, 98}}
	tests := []struct {
		name    string
		ids     []int
		merges  []Pair
		wantErr string
	}{
		{"ID beyond the merges", []int{97, 259}, aaab, "unknown token ID 259"},
		{"negative ID", []int{-1}, aaab, "unknown token ID -1"},
		{"merge defined by itself", []int{256}, []Pair{{256, 97}}, "defined in terms of itself"},
		{"merges defined by each other", []int{256}, []Pair{{257, 97}, {256, 98}}, "defined in terms of itself"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeBytes(tt.ids, tt.merges)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("decodeBytes(%v) error = %v, want one containing %q", tt.ids, err, tt.wantErr)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	texts := []string{
		"",
		"a",
		"aaabdaaabac",
		"to the top of the tree",
		"hello hello hello world\nhello, world!\n",
		"🙂 naïve café 🙂 naïve café",
		"हिंदी में लिखा गया वाक्य",
		"\x00\xff\xfe not UTF-8 \x80\xc3",
	}
	unseen := []byte("the tree is on top 🙂")
	for _, text := range texts {
		for _, n := range []int{0, 1, 5, 50} {
			merges := train([]byte(text), n)
			tok := newTokenizer(merges)
			for _, data := range [][]byte{[]byte(text), unseen} {
				got, err := decodeBytes(tok.encode(data), merges)
				if err != nil {
					t.Fatalf("decoding %q: %v", data, err)
				}
				if string(got) != string(data) {
					t.Errorf("trained on %q with %d merges: %q round-trips to %q", text, n, data, got)
				}
			}
		}
	}
}

func TestMergePairUsesGivenID(t *testing.T) {
	got := mergePair([]int{256, 97, 98}, Pair{256, 97}, 257)
	if want := []int{257, 98}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
