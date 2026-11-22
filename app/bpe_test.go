package main

import (
	"slices"
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

func TestMergePairUsesGivenID(t *testing.T) {
	got := mergePair([]int{256, 97, 98}, Pair{256, 97}, 257)
	if want := []int{257, 98}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
