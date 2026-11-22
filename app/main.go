// Command your_program.sh is a byte-pair encoding (BPE) tokenizer.
//
// Usage: your_program.sh <command> <file> [--flag value ...]
//
// Every command prints its result to stdout as JSON. Errors go to stderr.
package main

import (
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

func writeLine(w io.Writer, s string) error {
	_, err := fmt.Fprintln(w, s)
	return err
}
