package host

import (
"path/filepath"
"strings"
"testing"
)

func TestSaveFileSearchPaths(t *testing.T) {
tests := []struct {
name     string
gameDir  string
expected []string
}{
{
name:    "Mod directory",
gameDir: "hipnotic",
expected: []string{
"/home/user/.ironwail/saves/s0.sav",
"/quake/hipnotic/s0.sav",
"/quake/hipnotic/saves/s0.sav",
"/quake/s0.sav",
"/quake/id1/s0.sav",
"/quake/id1/saves/s0.sav",
},
},
{
name:    "Base directory (id1)",
gameDir: "id1",
expected: []string{
"/home/user/.ironwail/saves/s0.sav",
"/quake/id1/s0.sav",
"/quake/id1/saves/s0.sav",
"/quake/s0.sav",
},
},
}

for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
h := &Host{
baseDir: "/quake",
gameDir: tc.gameDir,
userDir: "/home/user/.ironwail",
}

// Mock saving logic requires some fields but saveFileSearchPaths is pureish
// (only reads config fields)

paths, err := h.saveFileSearchPaths("s0")
if err != nil {
t.Fatalf("saveFileSearchPaths failed: %v", err)
}

// The function might return paths slightly differently depending on OS separators
// But we clean them in the loop.

if len(paths) != len(tc.expected) {
t.Errorf("Expected %d paths, got %d", len(tc.expected), len(paths))
for i, p := range paths {
t.Logf("  %d: %s", i, p)
}
return
}

for i, path := range paths {
// Normalize path separators for cross-platform
// But strings here are hardcoded with /
// On Windows /quake might be weird but filepath.Clean handles it?
// Actually filepath.Clean uses OS separator.
// The expected strings use /.
// If running on Linux, / is fine.

// Let's replace expected / with filepath.Separator if needed.
expected := strings.ReplaceAll(tc.expected[i], "/", string(filepath.Separator))
path = filepath.Clean(path)
expected = filepath.Clean(expected)

if path != expected {
t.Errorf("Path %d: expected %s, got %s", i, expected, path)
}
}
})
}
}
