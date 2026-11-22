package main

// bytesToIDs returns the starting token IDs for data: one ID per byte, equal
// to the byte's value (0-255).
func bytesToIDs(data []byte) []int {
	ids := make([]int, len(data))
	for i, b := range data {
		ids[i] = int(b)
	}
	return ids
}
