package fs

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxQPath  = 64
	MaxOSPath = 1024
)

type PackFile struct {
	Name    string
	FilePos int32
	FileLen int32
}

type Pack struct {
	Filename string
	Handle   *os.File
	Files    []PackFile
}

type SearchResult struct {
	Path    string
	IsPack  bool
	Pack    *Pack
	FilePos int32
	FileLen int32
}

type FileSystem struct {
	searchPaths []string
	packs       []*Pack
	gameDir     string
	baseDir     string
}

func NewFileSystem() *FileSystem {
	return &FileSystem{
		searchPaths: make([]string, 0),
		packs:       make([]*Pack, 0),
	}
}

func (fs *FileSystem) Init(basedir, gamedir string) error {
	fs.baseDir = basedir
	fs.gameDir = gamedir

	// Base directory is the installation directory, we must always add 'id1'
	// as the fundamental Quake directory, then add any custom game directory.

	if err := fs.AddGameDirectory(filepath.Join(basedir, "id1")); err != nil {
		return fmt.Errorf("failed to add id1 directory: %w", err)
	}

	if gamedir != "" && gamedir != "id1" {
		if err := fs.AddGameDirectory(filepath.Join(basedir, gamedir)); err != nil {
			return fmt.Errorf("failed to add game directory: %w", err)
		}
	}

	return nil
}

func (fs *FileSystem) AddGameDirectory(dir string) error {
	fs.searchPaths = append(fs.searchPaths, dir)

	pakFiles, err := filepath.Glob(filepath.Join(dir, "*.pak"))
	if err != nil {
		return err
	}

	for _, pakFile := range pakFiles {
		pack, err := fs.loadPack(pakFile)
		if err != nil {
			fmt.Printf("Warning: failed to load pack %s: %v\n", pakFile, err)
			continue
		}
		fs.packs = append(fs.packs, pack)
	}

	return nil
}

func (fs *FileSystem) loadPack(filename string) (*Pack, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	var header struct {
		ID     [4]byte
		DirOfs int32
		DirLen int32
	}

	if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
		file.Close()
		return nil, err
	}

	if string(header.ID[:]) != "PACK" {
		file.Close()
		return nil, fmt.Errorf("not a pack file")
	}

	numFiles := int(header.DirLen / 64)

	if _, err := file.Seek(int64(header.DirOfs), io.SeekStart); err != nil {
		file.Close()
		return nil, err
	}

	files := make([]PackFile, numFiles)
	for i := 0; i < numFiles; i++ {
		var entry struct {
			Name    [56]byte
			FilePos int32
			FileLen int32
		}
		if err := binary.Read(file, binary.LittleEndian, &entry); err != nil {
			file.Close()
			return nil, err
		}
		idx := 0
		for idx < len(entry.Name) && entry.Name[idx] != 0 {
			idx++
		}
		files[i] = PackFile{
			Name:    string(entry.Name[:idx]),
			FilePos: entry.FilePos,
			FileLen: entry.FileLen,
		}
		}

	return &Pack{
		Filename: filename,
		Handle:   file,
		Files:    files,
	}, nil
}

func (fs *FileSystem) FindFile(filename string) (*SearchResult, error) {
	filename = filepath.ToSlash(filename)

	// First check loose files in reverse order of added paths (game dir overrides base dir)
	for i := len(fs.searchPaths) - 1; i >= 0; i-- {
		fullPath := filepath.Join(fs.searchPaths[i], filename)
		if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
			return &SearchResult{
				Path:   fullPath,
				IsPack: false,
			}, nil
		}
	}

	// Then check packs in reverse order (pak1 overrides pak0)
	for i := len(fs.packs) - 1; i >= 0; i-- {
		pack := fs.packs[i]
		for _, pf := range pack.Files {
			if pf.Name == filename {
				return &SearchResult{
					Path:    pack.Filename,
					IsPack:  true,
					Pack:    pack,
					FilePos: pf.FilePos,
					FileLen: pf.FileLen,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("file not found: %s", filename)
}

func (fs *FileSystem) LoadFile(filename string) ([]byte, error) {
	result, err := fs.FindFile(filename)
	if err != nil {
		return nil, err
	}

	if result.IsPack {
		return fs.loadFromPack(result)
	}

	return os.ReadFile(result.Path)
}

func (fs *FileSystem) loadFromPack(result *SearchResult) ([]byte, error) {
	if _, err := result.Pack.Handle.Seek(int64(result.FilePos), io.SeekStart); err != nil {
		return nil, err
	}

	data := make([]byte, result.FileLen)
	if _, err := io.ReadFull(result.Pack.Handle, data); err != nil {
		return nil, err
	}

	return data, nil
}

func (fs *FileSystem) FileExists(filename string) bool {
	_, err := fs.FindFile(filename)
	return err == nil
}

func (fs *FileSystem) GetGameDir() string {
	return fs.gameDir
}

func (fs *FileSystem) GetBaseDir() string {
	return fs.baseDir
}

func (fs *FileSystem) ListFiles(pattern string) []string {
	var results []string

	for _, path := range fs.searchPaths {
		matches, err := filepath.Glob(filepath.Join(path, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			rel, err := filepath.Rel(path, match)
			if err == nil {
				results = append(results, filepath.ToSlash(rel))
			}
		}
	}

	for _, pack := range fs.packs {
		for _, pf := range pack.Files {
			matched, err := filepath.Match(pattern, pf.Name)
			if err == nil && matched {
				results = append(results, pf.Name)
			}
		}
	}

	return results
}

func (fs *FileSystem) Close() {
	for _, pack := range fs.packs {
		if pack.Handle != nil {
			pack.Handle.Close()
		}
	}
}

func SkipPath(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func StripExtension(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx > strings.LastIndex(path, "/") {
		return path[:idx]
	}
	return path
}

func GetExtension(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx > strings.LastIndex(path, "/") && idx < len(path)-1 {
		return path[idx+1:]
	}
	return ""
}

func AddExtension(path, ext string) string {
	if GetExtension(path) == "" {
		return path + ext
	}
	return path
}

func DefaultExtension(path, ext string) string {
	if GetExtension(path) == "" {
		return path + ext
	}
	return path
}

func FileBase(path string) string {
	path = SkipPath(path)
	return StripExtension(path)
}

func CreatePath(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}
