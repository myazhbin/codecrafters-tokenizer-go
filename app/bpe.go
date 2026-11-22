package main

import "fmt"

// firstMergeID is the ID of the first token a merge creates. IDs 0-255 stand
// for single bytes, so merge i (counting from 0) creates ID firstMergeID + i.
const firstMergeID = 256

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

// mergePair returns a copy of ids with each occurrence of pair replaced by
// newID. It reads left to right, and matches can't overlap: merging (97, 97)
// in [97, 97, 97] gives [newID, 97].
func mergePair(ids []int, pair Pair, newID int) []int {
	merged := make([]int, 0, len(ids))
	for i := 0; i < len(ids); i++ {
		if i+1 < len(ids) && ids[i] == pair.First && ids[i+1] == pair.Second {
			merged = append(merged, newID)
			i++ // the pair's second ID is consumed too
		} else {
			merged = append(merged, ids[i])
		}
	}
	return merged
}

// mergeRule says what a learned pair becomes during encoding.
type mergeRule struct {
	rank int // position in the merge list; lower ranks are merged first
	id   int // ID of the merged token
}

// Tokenizer encodes bytes into token IDs by replaying learned merges.
type Tokenizer struct {
	byteIDs [256]int           // ID of each single-byte token
	rules   map[Pair]mergeRule // learned pairs and what they merge into
}

// newTokenizer returns a Tokenizer for merges in the format train produces:
// byte b has ID b, and merges[i] creates ID 256 + i.
func newTokenizer(merges []Pair) *Tokenizer {
	t := &Tokenizer{rules: make(map[Pair]mergeRule, len(merges))}
	for b := range t.byteIDs {
		t.byteIDs[b] = b
	}
	for i, p := range merges {
		if _, seen := t.rules[p]; !seen { // a repeated pair keeps its earliest rank
			t.rules[p] = mergeRule{rank: i, id: firstMergeID + i}
		}
	}
	return t
}

// encode converts data to token IDs. It starts with one ID per byte, then
// repeatedly finds the learned pair with the earliest rank that is present
// and merges every occurrence of it, until no neighboring pair was learned.
// Rank decides the order, not how often a pair appears in data.
func (t *Tokenizer) encode(data []byte) []int {
	ids := make([]int, len(data))
	for i, b := range data {
		ids[i] = t.byteIDs[b]
	}
	for len(ids) >= 2 {
		pair, rule, found := t.earliestRankedPair(ids)
		if !found {
			break
		}
		ids = mergePair(ids, pair, rule.id)
	}
	return ids
}

// earliestRankedPair returns the neighboring pair in ids with the lowest
// merge rank. found is false if no neighboring pair has a merge rule.
func (t *Tokenizer) earliestRankedPair(ids []int) (best Pair, bestRule mergeRule, found bool) {
	for i := 0; i+1 < len(ids); i++ {
		p := Pair{ids[i], ids[i+1]}
		if rule, ok := t.rules[p]; ok && (!found || rule.rank < bestRule.rank) {
			best, bestRule, found = p, rule, true
		}
	}
	return best, bestRule, found
}

// decodeBytes returns the bytes that ids stand for, joined in order, given
// the merges that created them. ID b below 256 is the single byte b, and ID
// 256 + i is the bytes of merges[i].First followed by those of its Second.
func decodeBytes(ids []int, merges []Pair) ([]byte, error) {
	var buf []byte
	for _, id := range ids {
		var err error
		if buf, err = appendTokenBytes(buf, merges, id, 0); err != nil {
			return nil, err
		}
	}
	return buf, nil
}

// appendTokenBytes appends the bytes of token id to dst, expanding merges
// recursively. depth counts the merges already being expanded above id.
func appendTokenBytes(dst []byte, merges []Pair, id, depth int) ([]byte, error) {
	if id >= 0 && id < firstMergeID {
		return append(dst, byte(id)), nil
	}
	i := id - firstMergeID
	if i < 0 || i >= len(merges) {
		return nil, fmt.Errorf("unknown token ID %d: there are %d merges, so the largest ID is %d",
			id, len(merges), firstMergeID+len(merges)-1)
	}
	// Expanding a token passes through each merge at most once, unless a
	// merge is (indirectly) defined in terms of itself.
	if depth >= len(merges) {
		return nil, fmt.Errorf("token ID %d is defined in terms of itself", id)
	}
	dst, err := appendTokenBytes(dst, merges, merges[i].First, depth+1)
	if err != nil {
		return nil, err
	}
	return appendTokenBytes(dst, merges, merges[i].Second, depth+1)
}

// train learns up to numMerges merges from data with byte-pair encoding.
// Each round merges the most frequent pair in the current IDs into the next
// new ID: the first merge creates 256, the second 257, and so on. Training
// stops early only if the text shrinks to a single token.
func train(data []byte, numMerges int) []Pair {
	ids := bytesToIDs(data)
	merges := make([]Pair, 0, min(numMerges, len(ids)))
	for len(merges) < numMerges {
		pair, ok := mostFrequentPair(ids)
		if !ok {
			break
		}
		ids = mergePair(ids, pair, firstMergeID+len(merges))
		merges = append(merges, pair)
	}
	return merges
}
