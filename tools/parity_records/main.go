// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/darkliquid/ironwail-go/internal/game"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go run ./tools/parity_records <state_dump.json>\n\n")
		fmt.Fprintf(os.Stderr, "Computes deterministic render-record hashes per frame from a dumpstate JSON stream.\n")
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	filePath := flag.Arg(0)
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", filePath, err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	reader := bufio.NewReader(f)
	frameNum := 0

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && len(line) == 0 {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "Error reading line: %v\n", err)
			break
		}

		var dump game.DumpFrameState
		if err := json.Unmarshal(line, &dump); err != nil {
			// Try parsing as array item or skip invalid line
			continue
		}

		rec := game.ComputeRenderRecordFromDump(dump)
		hash := rec.Hash()
		fmt.Printf("Frame %4d: %s (leaf=%d, ents=%d)\n", dump.Frame, hash, rec.ViewLeaf, len(rec.Entities))
		frameNum++
	}

	fmt.Fprintf(os.Stderr, "Computed render-record hashes for %d frames.\n", frameNum)
}
