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
