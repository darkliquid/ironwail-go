package fs

import (
	"encoding/binary"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	MaxQPath      = 64
	MaxOSPath     = 1024
	EnginePakName = "ironwail.pak"
)

type PackFile struct {
	Name    string
	Lookup  string
	FilePos int32
	FileLen int32
}

type Pack struct {
	Filename string
	Handle   *os.File
	Files    []PackFile
}

type SearchResult struct {
	Path     string
	Name     string
	SourceFS iofs.FS
	IsPack   bool
	Pack     *Pack
	FilePos  int32
	FileLen  int32
}

type searchPath struct {
	root string
	fs   iofs.FS
	pack *Pack
}

type FileSystem struct {
	searchPaths []searchPath
	lookupPaths []searchPath
	packs       []*Pack
	gameDir     string
	baseDir     string
	initialized bool
}

func NewFileSystem() *FileSystem {
	return &FileSystem{
		searchPaths: make([]searchPath, 0),
		lookupPaths: make([]searchPath, 0),
		packs:       make([]*Pack, 0),
	}
}

func (fs *FileSystem) Init(basedir, gamedir string) error {
	if fs.initialized {
		return nil
	}
	fs.baseDir = basedir
	fs.gameDir = gamedir

	// Base directory is the installation directory, we must always add 'id1'
	// as the fundamental Quake directory, then add any custom game directory.

	if err := fs.AddGameDirectory(filepath.Join(basedir, "id1")); err != nil {
		return fmt.Errorf("failed to add id1 directory: %w", err)
	}
	fs.addEnginePak()

	if gamedir != "" && gamedir != "id1" {
		if err := fs.AddGameDirectory(filepath.Join(basedir, gamedir)); err != nil {
			return fmt.Errorf("failed to add game directory: %w", err)
		}
	}

	fs.initialized = true
	return nil
}

func (fs *FileSystem) addEnginePak() {
	candidates := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Clean(filepath.Dir(exePath))
		if _, exists := seen[exeDir]; !exists {
			candidates = append(candidates, exeDir)
			seen[exeDir] = struct{}{}
		}
	}

	if fs.baseDir != "" {
		baseDir := filepath.Clean(fs.baseDir)
		if _, exists := seen[baseDir]; !exists {
			candidates = append(candidates, baseDir)
		}
	}

	for _, dir := range candidates {
		enginePakPath := filepath.Join(dir, EnginePakName)
		if _, err := os.Stat(enginePakPath); err != nil {
			continue
		}

		pack, err := fs.loadPack(enginePakPath)
		if err != nil {
			fmt.Printf("Warning: failed to load engine pak %s: %v\n", enginePakPath, err)
			continue
		}

		fs.packs = append(fs.packs, pack)
		fs.lookupPaths = append([]searchPath{{pack: pack}}, fs.lookupPaths...)
		fmt.Printf("Added engine pak: %s (%d files)\n", EnginePakName, len(pack.Files))
		return
	}
}

func (fs *FileSystem) AddGameDirectory(dir string) error {
	cleanDir := filepath.Clean(dir)
	loosePath := searchPath{
		root: cleanDir,
		fs:   os.DirFS(cleanDir),
	}
	fs.searchPaths = append(fs.searchPaths, loosePath)

	pakFiles, err := discoverPakFiles(cleanDir)
	if err != nil {
		return err
	}

	lookupGroup := make([]searchPath, 0, len(pakFiles)+1)
	for _, pakFile := range pakFiles {
		pack, err := fs.loadPack(pakFile)
		if err != nil {
			fmt.Printf("Warning: failed to load pack %s: %v\n", pakFile, err)
			continue
		}
		fs.packs = append(fs.packs, pack)
		lookupGroup = append([]searchPath{{pack: pack}}, lookupGroup...)
	}
	lookupGroup = append(lookupGroup, loosePath)
	fs.lookupPaths = append(lookupGroup, fs.lookupPaths...)

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
			Lookup:  canonicalPackLookup(string(entry.Name[:idx])),
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
	sanitizedName, err := sanitizePath(filename)
	if err != nil {
		return nil, err
	}

	lookupName := canonicalPackLookup(sanitizedName)
	for _, searchPath := range fs.lookupPaths {
		if searchPath.pack == nil {
			fullPath := filepath.Join(searchPath.root, filepath.FromSlash(sanitizedName))

			if !isWithinRoot(searchPath.root, fullPath) {
				return nil, fmt.Errorf("invalid path traversal attempt: %s", filename)
			}

			if stat, err := iofs.Stat(searchPath.fs, sanitizedName); err == nil && !stat.IsDir() {
				return &SearchResult{
					Path:     fullPath,
					Name:     sanitizedName,
					SourceFS: searchPath.fs,
					IsPack:   false,
				}, nil
			}
			continue
		}

		for _, pf := range searchPath.pack.Files {
			if pf.Lookup == lookupName {
				return &SearchResult{
					Path:    searchPath.pack.Filename,
					Name:    sanitizedName,
					IsPack:  true,
					Pack:    searchPath.pack,
					FilePos: pf.FilePos,
					FileLen: pf.FileLen,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("file not found: %s", sanitizedName)
}

func (fs *FileSystem) LoadFile(filename string) ([]byte, error) {
	result, err := fs.FindFile(filename)
	if err != nil {
		return nil, err
	}

	if result.IsPack {
		return fs.loadFromPack(result)
	}

	return iofs.ReadFile(result.SourceFS, result.Name)
}

func (fs *FileSystem) FindFirstAvailable(filenames []string) (*SearchResult, error) {
	if len(filenames) == 0 {
		return nil, fmt.Errorf("no filenames provided")
	}

	type candidate struct {
		name   string
		lookup string
	}
	candidates := make([]candidate, 0, len(filenames))
	for _, filename := range filenames {
		sanitizedName, err := sanitizePath(filename)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate{
			name:   sanitizedName,
			lookup: canonicalPackLookup(sanitizedName),
		})
	}

	for _, searchPath := range fs.lookupPaths {
		if searchPath.pack == nil {
			for _, candidate := range candidates {
				fullPath := filepath.Join(searchPath.root, filepath.FromSlash(candidate.name))
				if !isWithinRoot(searchPath.root, fullPath) {
					return nil, fmt.Errorf("invalid path traversal attempt: %s", candidate.name)
				}
				if stat, err := iofs.Stat(searchPath.fs, candidate.name); err == nil && !stat.IsDir() {
					return &SearchResult{
						Path:     fullPath,
						Name:     candidate.name,
						SourceFS: searchPath.fs,
						IsPack:   false,
					}, nil
				}
			}
			continue
		}

		for _, candidate := range candidates {
			for _, pf := range searchPath.pack.Files {
				if pf.Lookup == candidate.lookup {
					return &SearchResult{
						Path:    searchPath.pack.Filename,
						Name:    candidate.name,
						IsPack:  true,
						Pack:    searchPath.pack,
						FilePos: pf.FilePos,
						FileLen: pf.FileLen,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("none of the files were found: %s", strings.Join(filenames, ", "))
}

func (fs *FileSystem) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	result, err := fs.FindFirstAvailable(filenames)
	if err != nil {
		return "", nil, err
	}

	if result.IsPack {
		data, err := fs.loadFromPack(result)
		if err != nil {
			return "", nil, err
		}
		return result.Name, data, nil
	}

	data, err := iofs.ReadFile(result.SourceFS, result.Name)
	if err != nil {
		return "", nil, err
	}
	return result.Name, data, nil
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

// ModInfo describes a discovered mod directory under the base Quake directory.
type ModInfo struct {
	// Name is the directory name (e.g. "hipnotic", "rogue", "mymod").
	Name string
}

// ListMods returns all valid mod directories found in the basedir.
// A directory is considered a valid mod if it contains at least one pak file
// or a progs.dat file, following the same convention as C Ironwail's mod list.
// The well-known base directory "id1" is excluded because it is not a
// user-selectable mod; other directories are returned in sorted order.
func (fs *FileSystem) ListMods() []ModInfo {
	if fs.baseDir == "" {
		return nil
	}
	entries, err := os.ReadDir(fs.baseDir)
	if err != nil {
		return nil
	}

	var mods []ModInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(name, "id1") {
			continue
		}
		dirPath := filepath.Join(fs.baseDir, name)
		if isValidModDir(dirPath) {
			mods = append(mods, ModInfo{Name: name})
		}
	}
	return mods
}

// isValidModDir returns true if dir contains a pak file or a progs.dat.
func isValidModDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(name, "progs.dat") {
			return true
		}
		if pakFilePattern.MatchString(name) {
			return true
		}
	}
	return false
}

func (fs *FileSystem) ListFiles(pattern string) []string {
	var results []string

	for _, searchPath := range fs.searchPaths {
		matches, err := iofs.Glob(searchPath.fs, pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			results = append(results, filepath.ToSlash(match))
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

func sanitizePath(filename string) (string, error) {
	normalized := strings.ReplaceAll(filename, "\\", "/")
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(normalized)))

	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("invalid empty path")
	}

	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("absolute paths are not allowed: %s", filename)
	}

	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid path traversal attempt: %s", filename)
	}

	return cleaned, nil
}

func isWithinRoot(root, target string) bool {
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Clean(target)

	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil {
		return false
	}

	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

var pakFilePattern = regexp.MustCompile(`(?i)^pak([0-9]+)\.pak$`)

func discoverPakFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type pakInfo struct {
		path  string
		index int
	}
	var pakFiles []pakInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := pakFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		idx, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}
		pakFiles = append(pakFiles, pakInfo{
			path:  filepath.Join(dir, entry.Name()),
			index: idx,
		})
	}

	sort.Slice(pakFiles, func(i, j int) bool {
		if pakFiles[i].index != pakFiles[j].index {
			return pakFiles[i].index < pakFiles[j].index
		}
		return strings.ToLower(filepath.Base(pakFiles[i].path)) < strings.ToLower(filepath.Base(pakFiles[j].path))
	})

	paths := make([]string, len(pakFiles))
	for i := range pakFiles {
		paths[i] = pakFiles[i].path
	}
	return paths, nil
}

func canonicalPackLookup(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
}
