// Command qgo compiles Go source code to QCVM progs.dat bytecode.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/darkliquid/ironwail-go/cmd/qgo/compiler"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "source-order" {
		return runSourceOrder(args[1:], stdout, stderr)
	}
	return runCompile(args, stdout, stderr)
}

func runCompile(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("qgo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	output := fs.String("o", "progs.dat", "output file path")
	verbose := fs.Bool("v", false, "verbose output")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "qgo: %v\n", err)
		return 1
	}

	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}

	c := compiler.New()
	c.Verbose = *verbose

	data, err := c.Compile(dir)
	if err != nil {
		fmt.Fprintf(stderr, "qgo: %v\n", err)
		return 1
	}

	if err := os.WriteFile(*output, data, 0644); err != nil {
		fmt.Fprintf(stderr, "qgo: write %s: %v\n", *output, err)
		return 1
	}

	if *verbose {
		fmt.Fprintf(stdout, "wrote %s (%d bytes)\n", *output, len(data))
	}
	return 0
}

func runSourceOrder(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("qgo source-order", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	format := fs.String("format", "text", "output format")
	scope := fs.String("scope", "functions", "source-order scope")
	output := fs.String("o", "", "output file path")
	_ = fs.Bool("strict", false, "treat ambiguous declarations as errors")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "qgo: %v\n", err)
		return 1
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "qgo: unsupported -format %q for source-order; supported formats: text, json\n", *format)
		return 1
	}
	if *scope != "functions" && *scope != "files" {
		fmt.Fprintf(stderr, "qgo: unsupported -scope %q for source-order; supported scopes: functions, files\n", *scope)
		return 1
	}

	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}

	order, err := compiler.SourceOrder(dir)
	if err != nil {
		fmt.Fprintf(stderr, "qgo: %v\n", err)
		return 1
	}

	payload, err := formatSourceOrder(order, *format, *scope)
	if err != nil {
		fmt.Fprintf(stderr, "qgo: %v\n", err)
		return 1
	}
	if *output == "" {
		if _, err := stdout.Write(payload); err != nil {
			fmt.Fprintf(stderr, "qgo: write stdout: %v\n", err)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*output, payload, 0644); err != nil {
		fmt.Fprintf(stderr, "qgo: write %s: %v\n", *output, err)
		return 1
	}
	return 0
}

func formatSourceOrder(order []compiler.SourceOrderEntry, format, scope string) ([]byte, error) {
	switch format {
	case "text":
		return formatTextSourceOrder(order, scope), nil
	case "json":
		return formatJSONSourceOrder(order, scope)
	default:
		return nil, fmt.Errorf("unsupported source-order format %q", format)
	}
}

func formatTextSourceOrder(order []compiler.SourceOrderEntry, scope string) []byte {
	if len(order) == 0 {
		return nil
	}
	switch scope {
	case "functions":
		buf := make([]byte, 0, len(order)*24)
		for _, entry := range order {
			buf = fmt.Appendf(buf, "%d\t%s\t%s\n", entry.Index, entry.File, entry.Function)
		}
		return buf
	case "files":
		buf := make([]byte, 0, len(order)*16)
		files := make([]string, 0)
		seen := make(map[string]struct{}, len(order))
		for _, entry := range order {
			if _, ok := seen[entry.File]; ok {
				continue
			}
			seen[entry.File] = struct{}{}
			files = append(files, entry.File)
		}
		for idx, file := range files {
			buf = fmt.Appendf(buf, "%d\t%s\n", idx, file)
		}
		return buf
	default:
		return nil
	}
}

type sourceOrderFunctionJSONRow struct {
	Index    int    `json:"index"`
	File     string `json:"file"`
	Function string `json:"function"`
}

type sourceOrderFileJSONRow struct {
	Index int    `json:"index"`
	File  string `json:"file"`
}

func formatJSONSourceOrder(order []compiler.SourceOrderEntry, scope string) ([]byte, error) {
	switch scope {
	case "functions":
		rows := make([]sourceOrderFunctionJSONRow, 0, len(order))
		for _, entry := range order {
			rows = append(rows, sourceOrderFunctionJSONRow{
				Index:    entry.Index,
				File:     entry.File,
				Function: entry.Function,
			})
		}
		return marshalJSONWithTrailingNewline(rows)
	case "files":
		rows := make([]sourceOrderFileJSONRow, 0)
		seen := make(map[string]struct{}, len(order))
		for _, entry := range order {
			if _, ok := seen[entry.File]; ok {
				continue
			}
			seen[entry.File] = struct{}{}
			rows = append(rows, sourceOrderFileJSONRow{
				Index: len(rows),
				File:  entry.File,
			})
		}
		return marshalJSONWithTrailingNewline(rows)
	default:
		return nil, fmt.Errorf("unsupported source-order scope %q", scope)
	}
}

func marshalJSONWithTrailingNewline(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
