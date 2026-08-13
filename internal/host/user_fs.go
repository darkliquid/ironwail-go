// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"io/fs"
	"os"
)

// UserFS abstracts file operations on user-specific directory paths (e.g. config
// files, saved games). On native OS platforms this maps directly to the os
// package. On WebAssembly / browser platforms it maps to localStorage (with
// in-memory fallback).
type UserFS interface {
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
	Stat(filename string) (fs.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	Remove(filename string) error
}

func (h *Host) getUserFS() UserFS {
	if h != nil && h.userFS != nil {
		return h.userFS
	}
	userDir := ""
	if h != nil {
		userDir = h.userDir
	}
	return defaultUserFS(userDir)
}

func (h *Host) readUserFile(filename string) ([]byte, error) {
	return h.getUserFS().ReadFile(filename)
}

func (h *Host) writeUserFile(filename string, data []byte, perm os.FileMode) error {
	return h.getUserFS().WriteFile(filename, data, perm)
}

func (h *Host) userFileExists(filename string) bool {
	_, err := h.getUserFS().Stat(filename)
	return err == nil
}

func (h *Host) mkdirUserDir(path string) error {
	return h.getUserFS().MkdirAll(path, 0755)
}
