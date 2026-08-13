//go:build !js || !wasm

// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"io/fs"
	"os"
)

type osUserFS struct{}

func defaultUserFS(userDir string) UserFS {
	return osUserFS{}
}

func (osUserFS) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (osUserFS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

func (osUserFS) Stat(filename string) (fs.FileInfo, error) {
	return os.Stat(filename)
}

func (osUserFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osUserFS) Remove(filename string) error {
	return os.Remove(filename)
}
