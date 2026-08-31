// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package loc

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/console"
)

// Localization manages key-value string translation tables.
type Localization struct {
	mu      sync.RWMutex
	entries map[string]string
}

// New creates an empty Localization instance.
func New() *Localization {
	return &Localization{
		entries: make(map[string]string),
	}
}

var (
	globalMu  sync.RWMutex
	globalLoc = New()
)

// Default returns the process-wide localization instance.
func Default() *Localization {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalLoc
}

// SetDefault replaces the process-wide localization instance.
func SetDefault(l *Localization) {
	globalMu.Lock()
	defer globalMu.Unlock()
	if l != nil {
		globalLoc = l
	}
}

// Load reads and parses key-value pairs from a localization reader.
func (l *Localization) Load(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// Strip UTF-8 BOM if present
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	newEntries := make(map[string]string, len(lines))

	for _, line := range lines {
		line = strings.TrimLeft(line, " \t\r")
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}

		eqIdx := strings.IndexByte(line, '=')
		if eqIdx < 0 {
			continue
		}

		key := strings.TrimRight(line[:eqIdx], " \t\r")
		valRaw := strings.TrimLeft(line[eqIdx+1:], " \t\r")

		if key == "" {
			continue
		}

		val := parseLocValue(valRaw)
		newEntries[key] = val
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = newEntries
	return nil
}

func parseLocValue(valRaw string) string {
	if strings.HasPrefix(valRaw, "\"") {
		// Quoted string: extract until the matching unescaped quote
		var b strings.Builder
		valRaw = valRaw[1:]
		for i := 0; i < len(valRaw); i++ {
			if valRaw[i] == '\\' && i+1 < len(valRaw) {
				i++
				switch valRaw[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case 'r':
					b.WriteByte('\r')
				case 'v':
					b.WriteByte('\v')
				case 'b':
					b.WriteByte('\b')
				case 'f':
					b.WriteByte('\f')
				case '"':
					b.WriteByte('"')
				case '\'':
					b.WriteByte('\'')
				case '\\':
					b.WriteByte('\\')
				default:
					b.WriteByte(valRaw[i])
				}
				continue
			}
			if valRaw[i] == '"' {
				break
			}
			b.WriteByte(valRaw[i])
		}
		return b.String()
	}

	// Unquoted string: trim trailing whitespace and handle escape sequences
	valRaw = strings.TrimRight(valRaw, " \t\r\n")
	var b strings.Builder
	for i := 0; i < len(valRaw); i++ {
		if valRaw[i] == '\\' && i+1 < len(valRaw) {
			i++
			switch valRaw[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(valRaw[i])
			}
			continue
		}
		b.WriteByte(valRaw[i])
	}
	return b.String()
}

// LoadFile reads and parses a localization file from the filesystem.
func (l *Localization) LoadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return l.Load(f)
}

// LoadFromZip reads and parses a localization file from a zip archive.
func (l *Localization) LoadFromZip(zr *zip.Reader, innerPath string) error {
	normalizedTarget := strings.ToLower(strings.ReplaceAll(innerPath, "\\", "/"))
	for _, f := range zr.File {
		normalizedFile := strings.ToLower(strings.ReplaceAll(f.Name, "\\", "/"))
		if normalizedFile == normalizedTarget {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer func() { _ = rc.Close() }()
			return l.Load(rc)
		}
	}
	return fmt.Errorf("file %q not found in zip archive", innerPath)
}

// LoadFromArchiveFile opens a zip/kpf archive at archivePath and loads the file at innerPath.
func (l *Localization) LoadFromArchiveFile(archivePath, innerPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()
	return l.LoadFromZip(&zr.Reader, innerPath)
}

// GetString returns the translated string for key if it begins with '$',
// or the original key if no translation exists or it doesn't start with '$'.
func (l *Localization) GetString(key string) string {
	if !strings.HasPrefix(key, "$") {
		return key
	}
	lookupKey := key[1:]
	l.mu.RLock()
	defer l.mu.RUnlock()
	if val, ok := l.entries[lookupKey]; ok {
		return val
	}
	return key
}

// GetRawString returns the translated string for key if found, or empty string.
func (l *Localization) GetRawString(key string) string {
	if !strings.HasPrefix(key, "$") {
		return ""
	}
	lookupKey := key[1:]
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.entries[lookupKey]
}

// HasPlaceholders reports whether str contains '{}' or '{N}' placeholders.
func HasPlaceholders(str string) bool {
	for i := 0; i < len(str); i++ {
		if str[i] == '{' {
			j := i + 1
			for j < len(str) && str[j] >= '0' && str[j] <= '9' {
				j++
			}
			if j < len(str) && str[j] == '}' {
				return true
			}
		}
	}
	return false
}

// Len returns the number of localization entries loaded.
func (l *Localization) Len() int {
	if l == nil {
		return 0
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Format replaces '{}' and '{N}' placeholders in format with values from getArg.
func Format(format string, getArg func(int) string) string {
	var b strings.Builder
	b.Grow(len(format))
	autoIdx := 0

	for i := 0; i < len(format); i++ {
		if format[i] == '{' {
			j := i + 1
			isDigits := true
			num := 0

			if j < len(format) && format[j] == '}' {
				b.WriteString(getArg(autoIdx))
				autoIdx++
				i = j
				continue
			}

			for j < len(format) && format[j] != '}' {
				if format[j] < '0' || format[j] > '9' {
					isDigits = false
					break
				}
				num = num*10 + int(format[j]-'0')
				j++
			}

			if isDigits && j < len(format) && format[j] == '}' {
				b.WriteString(getArg(num))
				i = j
				continue
			}
		}
		b.WriteByte(format[i])
	}

	return b.String()
}

// Init loads localization tables from basedir for the specified language.
func Init(baseDir string, language string) {
	if language == "" || strings.EqualFold(language, "auto") {
		language = "english"
	}
	language = strings.ToLower(language)

	l := New()
	targetFile := fmt.Sprintf("loc_%s.txt", language)
	targetInnerPath := "localization/" + targetFile

	loadFromBaseDir := func(innerPath, langFile string) (bool, string) {
		if baseDir == "" {
			return false, ""
		}
		// 1. Try loose file under baseDir/localization/
		loosePath := filepath.Join(baseDir, "localization", langFile)
		if err := l.LoadFile(loosePath); err == nil {
			return true, loosePath
		}
		// 2. Try QuakeEX.kpf in baseDir
		kpfPath := filepath.Join(baseDir, "QuakeEX.kpf")
		if err := l.LoadFromArchiveFile(kpfPath, innerPath); err == nil {
			return true, fmt.Sprintf("%s (%s)", kpfPath, innerPath)
		}
		// 3. Try rerelease/QuakeEX.kpf in baseDir
		kpfRerelease := filepath.Join(baseDir, "rerelease", "QuakeEX.kpf")
		if err := l.LoadFromArchiveFile(kpfRerelease, innerPath); err == nil {
			return true, fmt.Sprintf("%s (%s)", kpfRerelease, innerPath)
		}
		return false, ""
	}

	found, source := loadFromBaseDir(targetInnerPath, targetFile)
	if !found && language != "english" {
		found, source = loadFromBaseDir("localization/loc_english.txt", "loc_english.txt")
	}

	if found {
		slog.Info("localization initialized", "language", language, "entries", len(l.entries), "source", source)
		console.Printf("[skipnotify]Loaded %d strings from '%s'\n", len(l.entries), source)
	} else {
		slog.Debug("no localization tables found", "language", language, "basedir", baseDir)
	}

	SetDefault(l)
}

// GetString translates key using the process-wide default Localization table.
func GetString(key string) string {
	return Default().GetString(key)
}

// GetRawString translates key using the process-wide default Localization table.
func GetRawString(key string) string {
	return Default().GetRawString(key)
}
