package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI runs the program with args and returns what it printed to stdout.
func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	if err := run(args, &out); err != nil {
		t.Fatalf("run(%q) failed: %v", args, err)
	}
	return out.String()
}

// writeTempFile writes content to a new file in a per-test directory and
// returns its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBytesCommand(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"stage example", "hi 👋", "[104, 105, 32, 240, 159, 145, 139]\n"},
		{"empty file", "", "[]\n"},
		{"only an emoji", "👋", "[240, 159, 145, 139]\n"},
		{"several lines", "one\ntwo\n", "[111, 110, 101, 10, 116, 119, 111, 10]\n"},
		{"spaces and tabs", "a b\tc", "[97, 32, 98, 9, 99]\n"},
		{"two-byte characters", "é€", "[195, 169, 226, 130, 172]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempFile(t, "text.txt", tt.text)
			got := runCLI(t, "bytes", path)
			if got != tt.want {
				t.Errorf("bytes %q printed %q, want %q", tt.text, got, tt.want)
			}

			// The output must be a JSON array with one value per byte, each 0-255.
			var ids []int
			if err := json.Unmarshal([]byte(got), &ids); err != nil {
				t.Fatalf("output is not a JSON array of integers: %v", err)
			}
			if len(ids) != len(tt.text) {
				t.Errorf("got %d IDs, want one per byte (%d)", len(ids), len(tt.text))
			}
			for i, id := range ids {
				if id < 0 || id > 255 || id != int(tt.text[i]) {
					t.Errorf("ID %d is %d, want byte value %d", i, id, tt.text[i])
				}
			}
		})
	}
}

func TestBytesCommandKeepsEveryByte(t *testing.T) {
	// All 256 byte values, including ones that are not valid UTF-8 on their own.
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	got := runCLI(t, "bytes", writeTempFile(t, "all.bin", string(data)))
	var ids []int
	if err := json.Unmarshal([]byte(got), &ids); err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		if id != i {
			t.Fatalf("ID %d is %d, want %d", i, id, i)
		}
	}
	if len(ids) != 256 {
		t.Fatalf("got %d IDs, want 256", len(ids))
	}
}

func TestTopPairCommand(t *testing.T) {
	tests := []struct {
		name string
		ids  string
		want string
	}{
		{"stage example", "[1, 2, 3, 1, 2]", "[1, 2]\n"},
		{"tie", "[8, 9, 1, 2]", "[8, 9]\n"},
		{"two-way tie", "[1, 2, 3, 4, 3, 4, 1, 2]", "[1, 2]\n"},
		{"overlapping", "[5, 6, 97, 97, 97]", "[97, 97]\n"},
		{"aaabdaaabac", "[97, 97, 97, 98, 100, 97, 97, 97, 98, 97, 99]", "[97, 97]\n"},
		{"compact JSON", "[300,301,300,301]", "[300, 301]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runCLI(t, "top-pair", writeTempFile(t, "ids.json", tt.ids))
			if got != tt.want {
				t.Errorf("top-pair %s printed %q, want %q", tt.ids, got, tt.want)
			}
		})
	}
}

func TestMergeCommand(t *testing.T) {
	tests := []struct {
		name string
		ids  string
		pair string
		want string
	}{
		{"stage example", "[1, 2, 3, 1, 2]", "1,2", "[256, 3, 256]\n"},
		{"overlapping", "[97, 97, 97]", "97,97", "[256, 97]\n"},
		{"pair never appears", "[1, 2, 3]", "3,1", "[1, 2, 3]\n"},
		{"empty list", "[]", "1,2", "[]\n"},
		{"aaabdaaabac", "[97, 97, 97, 98, 100, 97, 97, 97, 98, 97, 99]", "97,97", "[256, 97, 98, 100, 256, 97, 98, 97, 99]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runCLI(t, "merge", writeTempFile(t, "ids.json", tt.ids), "--pair", tt.pair)
			if got != tt.want {
				t.Errorf("merge %s --pair %s printed %q, want %q", tt.ids, tt.pair, got, tt.want)
			}
		})
	}
}

func TestTrainCommand(t *testing.T) {
	corpus := writeTempFile(t, "corpus.txt", "aaabdaaabac")
	got := runCLI(t, "train", corpus, "--num-merges", "3")
	want := "[\n  [97, 97],\n  [256, 97],\n  [257, 98]\n]\n"
	if got != want {
		t.Errorf("train printed %q, want %q", got, want)
	}

	// The output must parse as a JSON array of pairs.
	var pairs [][]int
	if err := json.Unmarshal([]byte(got), &pairs); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if len(pairs) != 3 {
		t.Errorf("got %d pairs, want 3", len(pairs))
	}

	if got := runCLI(t, "train", corpus, "--num-merges", "0"); got != "[]\n" {
		t.Errorf("train with 0 merges printed %q, want %q", got, "[]\n")
	}
}

func TestEncodeCommand(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		merges string
		want   string
	}{
		{"stage example", "aaab", "[[97, 97], [256, 97], [257, 98]]", "[258]\n"},
		{"training text", "aaabdaaabac", "[[97, 97], [256, 97], [257, 98]]", "[258, 100, 258, 97, 99]\n"},
		{"competing merges", "ababab", "[[98, 97], [97, 98]]", "[97, 256, 256, 98]\n"},
		{"empty file", "", "[[97, 97]]", "[]\n"},
		{"empty merge list", "hi", "[]", "[104, 105]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := writeTempFile(t, "text.txt", tt.text)
			merges := writeTempFile(t, "merges.json", tt.merges)
			got := runCLI(t, "encode", text, "--merges", merges)
			if got != tt.want {
				t.Errorf("encode %q printed %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestEncodeCommandPretokenized(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		merges string
		want   string
	}{
		{
			"stage example",
			"Hello world's 123!!",
			"[[108, 108], [111, 32]]",
			"[72, 101, 256, 111, 32, 119, 111, 114, 108, 100, 39, 115, 32, 49, 50, 51, 33, 33]\n",
		},
		{"contraction is case sensitive", "DON'T", "[[68, 79], [39, 84]]", "[256, 78, 39, 84]\n"},
		// (185, 224) spans the boundary between the chunks "ह" and "िं".
		{"Hindi", "हिंदी", "[[185, 224]]", "[224, 164, 185, 224, 164, 191, 224, 164, 130, 224, 164, 166, 224, 165, 128]\n"},
		{"non-breaking space", "a b", "[[194, 160], [256, 98]]", "[97, 256, 98]\n"},
		{"empty file", "", "[[97, 98]]", "[]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := writeTempFile(t, "text.txt", tt.text)
			merges := writeTempFile(t, "merges.json", tt.merges)
			got := runCLI(t, "encode", text, "--merges", merges, "--pretokenize", "gpt2")
			if got != tt.want {
				t.Errorf("encode %q printed %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestTrainOutputWorksAsMergesFile(t *testing.T) {
	corpus := "to the top of the tree"
	corpusPath := writeTempFile(t, "corpus.txt", corpus)
	trained := runCLI(t, "train", corpusPath, "--num-merges", "5")
	mergesPath := writeTempFile(t, "merges.json", trained)

	got := runCLI(t, "encode", corpusPath, "--merges", mergesPath)
	if want := "[260, 259, 111, 112, 32, 111, 102, 259, 114, 101, 101]\n"; got != want {
		t.Errorf("encode printed %q, want %q", got, want)
	}
}

func TestDecodeCommand(t *testing.T) {
	aaab := "[[97, 97], [256, 97], [257, 98]]"
	tests := []struct {
		name   string
		ids    string
		merges string
		want   string // the decoded text, before JSON encoding
	}{
		{"stage example", "[258, 100]", aaab, "aaabd"},
		{"empty", "[]", aaab, ""},
		{"full wave emoji", "[240, 159, 145, 139]", "[]", "👋"},
		{"half wave emoji", "[240, 159, 145]", "[]", "�"},
		{"never-valid bytes", "[255, 255]", "[]", "��"},
		{"emoji split across merged tokens", "[257, 139]", "[[240, 159], [256, 145]]", "👋"},
		{"progress sentence", "[260, 259, 111, 112, 32, 111, 102, 259, 114, 101, 101]",
			"[[32, 116], [256, 104], [257, 101], [258, 256], [116, 111]]", "to the top of the tree"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := writeTempFile(t, "ids.json", tt.ids)
			merges := writeTempFile(t, "merges.json", tt.merges)
			out := runCLI(t, "decode", ids, "--merges", merges)
			var got string
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("output %q is not a JSON string: %v", out, err)
			}
			if got != tt.want {
				t.Errorf("decode %s printed %q, want %q", tt.ids, got, tt.want)
			}
		})
	}
}

func TestDecodeCommandEscapesJSON(t *testing.T) {
	// The bytes of: " \ newline tab < &
	ids := writeTempFile(t, "ids.json", "[34, 92, 10, 9, 60, 38]")
	merges := writeTempFile(t, "merges.json", "[]")
	got := runCLI(t, "decode", ids, "--merges", merges)
	if want := `"\"\\\n\t<&"` + "\n"; got != want {
		t.Errorf("decode printed %q, want %q", got, want)
	}
}

func TestParsePair(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want Pair
	}{
		{"1,2", Pair{1, 2}},
		{"256,97", Pair{256, 97}},
		{"32,116", Pair{32, 116}},
	} {
		if got, err := parsePair(tt.in); err != nil || got != tt.want {
			t.Errorf("parsePair(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", "1", "1,", ",2", "a,b", "1,2,3", "1;2"} {
		if got, err := parsePair(bad); err == nil {
			t.Errorf("parsePair(%q) = %v, want an error", bad, got)
		}
	}
}

func TestRunErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no command", nil, "usage"},
		{"unknown command", []string{"frobnicate", "x"}, "unknown command"},
		{"missing file argument", []string{"bytes"}, "expected one file argument"},
		{"too many file arguments", []string{"bytes", "a", "b"}, "expected one file argument"},
		{"unknown flag", []string{"bytes", "a", "--bogus", "1"}, "unknown flag --bogus"},
		{"file does not exist", []string{"bytes", filepath.Join(t.TempDir(), "missing.txt")}, "no such file"},
		{"top-pair with one ID", []string{"top-pair", writeTempFile(t, "ids.json", "[1]")}, "fewer than two IDs"},
		{"top-pair with bad JSON", []string{"top-pair", writeTempFile(t, "ids.json", "[1, 2")}, "parsing"},
		{"top-pair with non-integer IDs", []string{"top-pair", writeTempFile(t, "ids.json", `["a", "b"]`)}, "parsing"},
		{"merge without --pair", []string{"merge", writeTempFile(t, "ids.json", "[1, 2]")}, "missing required flag --pair"},
		{"merge with a bad --pair", []string{"merge", writeTempFile(t, "ids.json", "[1, 2]"), "--pair", "1"}, "two integers"},
		{"train without --num-merges", []string{"train", writeTempFile(t, "corpus.txt", "abc")}, "missing required flag --num-merges"},
		{"train with a negative count", []string{"train", writeTempFile(t, "corpus.txt", "abc"), "--num-merges", "-1"}, "non-negative integer"},
		{"train with a non-number", []string{"train", writeTempFile(t, "corpus.txt", "abc"), "--num-merges", "x"}, "non-negative integer"},
		{"encode without --merges or --vocab", []string{"encode", writeTempFile(t, "text.txt", "abc")}, "encode needs --merges <file> or --vocab <directory>"},
		{
			"encode with both --merges and --vocab",
			[]string{"encode", writeTempFile(t, "text.txt", "abc"), "--merges", writeTempFile(t, "merges.json", "[]"), "--vocab", t.TempDir()},
			"not both",
		},
		{"encode with a missing vocab directory", []string{"encode", writeTempFile(t, "text.txt", "abc"), "--vocab", filepath.Join(t.TempDir(), "nope")}, "no such file"},
		{
			"encode with a malformed merge",
			[]string{"encode", writeTempFile(t, "text.txt", "abc"), "--merges", writeTempFile(t, "merges.json", "[[97, 98, 99]]")},
			"merge 0 has 3 IDs, want 2",
		},
		{
			"encode with an unknown pretokenizer",
			[]string{"encode", writeTempFile(t, "text.txt", "abc"), "--merges", writeTempFile(t, "merges.json", "[]"), "--pretokenize", "gpt4"},
			`--pretokenize "gpt4" is not supported`,
		},
		{"decode without --merges", []string{"decode", writeTempFile(t, "ids.json", "[97]")}, "missing required flag --merges"},
		{
			"decode an unknown ID",
			[]string{"decode", writeTempFile(t, "ids.json", "[300]"), "--merges", writeTempFile(t, "merges.json", "[[97, 97]]")},
			"unknown token ID 300",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(tt.args, &out)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("run(%q) error = %v, want one containing %q", tt.args, err, tt.wantErr)
			}
			if out.Len() != 0 {
				t.Errorf("run(%q) printed %q to stdout on error", tt.args, out.String())
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantFile  string
		wantFlags map[string]string
	}{
		{"file only", []string{"in.txt"}, "in.txt", map[string]string{}},
		{"flag after file", []string{"in.txt", "--pair", "1,2"}, "in.txt", map[string]string{"pair": "1,2"}},
		{"flag before file", []string{"--pair", "1,2", "in.txt"}, "in.txt", map[string]string{"pair": "1,2"}},
		{"equals form", []string{"in.txt", "--pair=1,2"}, "in.txt", map[string]string{"pair": "1,2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args, "pair")
			if err != nil {
				t.Fatal(err)
			}
			if got.file != tt.wantFile {
				t.Errorf("file = %q, want %q", got.file, tt.wantFile)
			}
			if len(got.flags) != len(tt.wantFlags) {
				t.Errorf("flags = %v, want %v", got.flags, tt.wantFlags)
			}
			for k, v := range tt.wantFlags {
				if got.flags[k] != v {
					t.Errorf("flag %q = %q, want %q", k, got.flags[k], v)
				}
			}
		})
	}

	if _, err := parseArgs([]string{"in.txt", "--pair"}, "pair"); err == nil {
		t.Error("expected an error for a flag with no value")
	}
}
