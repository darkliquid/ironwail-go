// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/darkliquid/ironwail-go/internal/server"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go run ./tools/parity_recorder [flags] <stream1.json> [stream2.json]\n\n")
		fmt.Fprintf(os.Stderr, "Inspects or diffs recorded server message streams for bit parity.\n")
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	loadStream := func(path string) ([]server.RecordedFrameMessage, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}

		var frames []server.RecordedFrameMessage
		if err := json.Unmarshal(data, &frames); err != nil {
			return nil, err
		}
		return frames, nil
	}

	stream1, err := loadStream(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", flag.Arg(0), err)
		os.Exit(1)
	}

	if flag.NArg() == 1 {
		fmt.Printf("Stream %s: %d frames recorded\n", flag.Arg(0), len(stream1))
		for i, f := range stream1 {
			if i < 10 || i >= len(stream1)-5 {
				fmt.Printf("  Frame %4d (client %d): %d bytes [%s]\n", f.Frame, f.Client, len(f.Bytes), f.ByteHex)
			} else if i == 10 {
				fmt.Println("  ...")
			}
		}
		return
	}

	stream2, err := loadStream(flag.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", flag.Arg(1), err)
		os.Exit(1)
	}

	diffIdx, reason := server.DiffStreams(stream1, stream2)
	if diffIdx == -1 {
		fmt.Printf("SUCCESS: Streams %s and %s are identical (%d frames).\n", flag.Arg(0), flag.Arg(1), len(stream1))
	} else {
		fmt.Fprintf(os.Stderr, "FAILURE: Streams diverge at index %d: %s\n", diffIdx, reason)
		os.Exit(2)
	}
}
