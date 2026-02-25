//go:build gogpu
// +build gogpu

package main

import (
	"log/slog"

	"github.com/gogpu/gogpu"
)

func init() {
	// Set up gogpu logger
	gogpu.SetLogger(slog.Default())
}
