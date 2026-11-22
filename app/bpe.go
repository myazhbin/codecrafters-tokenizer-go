package main

// Pair is two neighboring token IDs, in order.
type Pair struct {
	First, Second int
}

// bytesToIDs returns the starting token IDs for data: one ID per byte, equal
// to the byte's value (0-255).
func bytesToIDs(data []byte) []int {
	ids := make([]int, len(data))
	for i, b := range data {
		ids[i] = int(b)
	}
	return ids
}

// mostFrequentPair returns the pair of neighboring IDs that occurs most often
// in ids. Overlapping pairs all count, so [97, 97, 97] has (97, 97) twice.
// On a tie, the pair whose first occurrence comes earliest wins. ok is false
// when ids has fewer than two IDs.
func mostFrequentPair(ids []int) (best Pair, ok bool) {
	counts := make(map[Pair]int)
	var order []Pair // distinct pairs, in order of first occurrence
	for i := 0; i+1 < len(ids); i++ {
		p := Pair{ids[i], ids[i+1]}
		if counts[p] == 0 {
			order = append(order, p)
		}
		counts[p]++
	}
	if len(order) == 0 {
		return Pair{}, false
	}
	best = order[0]
	for _, p := range order[1:] {
		if counts[p] > counts[best] { // strictly greater keeps the earliest on ties
			best = p
		}
	}
	return best, true
}
