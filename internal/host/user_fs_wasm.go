//go:build js && wasm

// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall/js"
	"time"
	"unicode/utf8"
)

type wasmUserFS struct {
	mu      sync.Mutex
	userDir string
	mem     map[string][]byte
}

func defaultUserFS(userDir string) UserFS {
	return &wasmUserFS{
		userDir: filepath.Clean(userDir),
		mem:     make(map[string][]byte),
	}
}

func (w *wasmUserFS) toStorageKey(path string) string {
	cleaned := filepath.Clean(path)
	if w.userDir != "" && w.userDir != "." && w.userDir != "/" {
		if rel, err := filepath.Rel(w.userDir, cleaned); err == nil && !strings.HasPrefix(rel, "..") {
			cleaned = rel
		}
	}
	cleaned = filepath.ToSlash(cleaned)
	cleaned = strings.TrimPrefix(cleaned, "/")
	return "ironwail:" + cleaned
}

func (w *wasmUserFS) ReadFile(filename string) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := w.toStorageKey(filename)

	// Check localStorage first if available
	if storage := js.Global().Get("localStorage"); !storage.IsUndefined() && !storage.IsNull() {
		val := storage.Call("getItem", key)
		if !val.IsNull() && !val.IsUndefined() {
			str := val.String()
			if strings.HasPrefix(str, "base64:") {
				dec, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(str, "base64:"))
				if err == nil {
					return dec, nil
				}
			}
			return []byte(str), nil
		}
	}

	// Fallback to in-memory store
	if data, ok := w.mem[key]; ok {
		cp := make([]byte, len(data))
		copy(cp, data)
		return cp, nil
	}

	return nil, &os.PathError{Op: "open", Path: filename, Err: os.ErrNotExist}
}

func (w *wasmUserFS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := w.toStorageKey(filename)

	// Store in memory map
	cp := make([]byte, len(data))
	copy(cp, data)
	w.mem[key] = cp

	// Store in localStorage if available
	if storage := js.Global().Get("localStorage"); !storage.IsUndefined() && !storage.IsNull() {
		var encoded string
		if utf8.Valid(data) {
			encoded = string(data)
		} else {
			encoded = "base64:" + base64.StdEncoding.EncodeToString(data)
		}
		storage.Call("setItem", key, encoded)
	}

	return nil
}

func (w *wasmUserFS) Stat(filename string) (fs.FileInfo, error) {
	data, err := w.ReadFile(filename)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: filename, Err: os.ErrNotExist}
	}
	return &virtualFileInfo{
		name:    filepath.Base(filename),
		size:    int64(len(data)),
		mode:    0644,
		modTime: time.Now(),
	}, nil
}

func (w *wasmUserFS) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (w *wasmUserFS) Remove(filename string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := w.toStorageKey(filename)
	delete(w.mem, key)

	if storage := js.Global().Get("localStorage"); !storage.IsUndefined() && !storage.IsNull() {
		storage.Call("removeItem", key)
	}

	return nil
}

type virtualFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (v *virtualFileInfo) Name() string       { return v.name }
func (v *virtualFileInfo) Size() int64        { return v.size }
func (v *virtualFileInfo) Mode() fs.FileMode  { return v.mode }
func (v *virtualFileInfo) ModTime() time.Time { return v.modTime }
func (v *virtualFileInfo) IsDir() bool        { return false }
func (v *virtualFileInfo) Sys() any           { return nil }
