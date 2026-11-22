// Command your_program.sh is a byte-pair encoding (BPE) tokenizer.
//
// Usage: your_program.sh <command> <file> [--flag value ...]
//
// Every command prints its result to stdout as JSON. Errors go to stderr.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run executes the command named by args[0] and writes its output to w.
func run(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: your_program.sh <command> <file> [--flag value ...]")
	}
	command, rest := args[0], args[1:]
	switch command {
	case "bytes":
		return runBytes(rest, w)
	case "top-pair":
		return runTopPair(rest, w)
	case "merge":
		return runMerge(rest, w)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

// runBytes prints a file's UTF-8 bytes. Each byte's value is its token ID.
func runBytes(args []string, w io.Writer) error {
	cmd, err := parseArgs(args)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(cmd.file)
	if err != nil {
		return err
	}
	return writeLine(w, formatIDs(bytesToIDs(data)))
}

// runTopPair prints the most frequent pair of neighboring IDs in a JSON list.
func runTopPair(args []string, w io.Writer) error {
	cmd, err := parseArgs(args)
	if err != nil {
		return err
	}
	ids, err := readIDs(cmd.file)
	if err != nil {
		return err
	}
	pair, ok := mostFrequentPair(ids)
	if !ok {
		return fmt.Errorf("%s has fewer than two IDs, so it has no pairs", cmd.file)
	}
	return writeLine(w, formatPair(pair))
}

// runMerge replaces each occurrence of --pair in a JSON list of IDs with the
// first merge ID, 256.
func runMerge(args []string, w io.Writer) error {
	cmd, err := parseArgs(args, "pair")
	if err != nil {
		return err
	}
	pairFlag, err := cmd.required("pair")
	if err != nil {
		return err
	}
	pair, err := parsePair(pairFlag)
	if err != nil {
		return err
	}
	ids, err := readIDs(cmd.file)
	if err != nil {
		return err
	}
	return writeLine(w, formatIDs(mergePair(ids, pair, firstMergeID)))
}

// parsePair parses two integers separated by a comma, like "1,2".
func parsePair(s string) (Pair, error) {
	first, second, found := strings.Cut(s, ",")
	if found {
		a, errA := strconv.Atoi(strings.TrimSpace(first))
		b, errB := strconv.Atoi(strings.TrimSpace(second))
		if errA == nil && errB == nil {
			return Pair{a, b}, nil
		}
	}
	return Pair{}, fmt.Errorf("--pair %q must be two integers separated by a comma, like 1,2", s)
}

// readIDs parses a file holding a JSON array of token IDs.
func readIDs(path string) ([]int, error) {
	var ids []int
	if err := readJSON(path, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// readJSON parses the JSON file at path into v.
func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}

// commandArgs holds a command's file argument and its flag values.
type commandArgs struct {
	file  string
	flags map[string]string
}

// parseArgs reads exactly one positional file argument plus any of the
// allowed flags, written as "--name value" or "--name=value", in any order.
func parseArgs(args []string, allowed ...string) (commandArgs, error) {
	cmd := commandArgs{flags: make(map[string]string)}
	var positional []string
	for i := 0; i < len(args); i++ {
		name, isFlag := strings.CutPrefix(args[i], "--")
		if !isFlag || name == "" {
			positional = append(positional, args[i])
			continue
		}
		name, value, hasValue := strings.Cut(name, "=")
		if !slices.Contains(allowed, name) {
			return cmd, fmt.Errorf("unknown flag --%s", name)
		}
		if !hasValue {
			if i+1 == len(args) {
				return cmd, fmt.Errorf("flag --%s needs a value", name)
			}
			i++
			value = args[i]
		}
		cmd.flags[name] = value
	}
	if len(positional) != 1 {
		return cmd, fmt.Errorf("expected one file argument, got %d", len(positional))
	}
	cmd.file = positional[0]
	return cmd, nil
}

// required returns the value of a flag the command can't run without.
func (c commandArgs) required(name string) (string, error) {
	value, ok := c.flags[name]
	if !ok {
		return "", fmt.Errorf("missing required flag --%s", name)
	}
	return value, nil
}

// formatIDs renders ids as a one-line JSON array, like [104, 105, 32].
func formatIDs(ids []int) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, id := range ids {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.Itoa(id))
	}
	sb.WriteByte(']')
	return sb.String()
}

// formatPair renders p as a JSON array, like [1, 2].
func formatPair(p Pair) string {
	return formatIDs([]int{p.First, p.Second})
}

func writeLine(w io.Writer, s string) error {
	_, err := fmt.Fprintln(w, s)
	return err
}
