package main

import (
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
