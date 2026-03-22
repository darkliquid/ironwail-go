package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	goFuncPattern = regexp.MustCompile(`(?m)^func\s+(?:\([^)]+\)\s+)?([a-zA-Z0-9_]+)\s*\(`)
	cFuncPattern  = regexp.MustCompile(`(?m)^[a-zA-Z_][a-zA-Z0-9_*\s]+?\s+([a-zA-Z0-9_]+)\s*\([^;]*$`)
)

var cFiles = []string{
	"sv_phys.c", "sv_move.c", "sv_user.c", "sv_main.c",
	"cl_main.c", "cl_parse.c", "cl_input.c", "cl_tent.c",
	"host.c", "host_cmd.c", "net_main.c", "net_dgrm.c",
	"snd_dma.c", "snd_mix.c", "r_world.c", "r_alias.c", "r_part.c",
}

var cPrefixes = []string{"SV_", "CL_", "Net_", "R_", "S_", "Host_", "Sys_", "Q_"}

var cKeywords = map[string]struct{}{
	"if":     {},
	"while":  {},
	"for":    {},
	"switch": {},
	"return": {},
}

func main() {
	os.Exit(run())
}

func run() int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getwd: %v\n", err)
		return 1
	}

	cDirFlag := flag.String("c-dir", envOr("C_DIR", "/home/darkliquid/Projects/ironwail/Quake"), "path to the canonical C source tree")
	goDirFlag := flag.String("go-dir", envOr("GO_DIR", wd), "path to the Go repository root")
	outFlag := flag.String("out", "", "path to write the generated markdown report (default: <go-dir>/docs/NEW_PARITY_TODO.md)")
	flag.Parse()

	goDir := *goDirFlag
	if goDir == "" {
		goDir = wd
	}
	outPath := *outFlag
	if outPath == "" {
		outPath = filepath.Join(goDir, "docs", "NEW_PARITY_TODO.md")
	}

	goFunctions, err := collectGoFunctions(goDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "collect Go functions: %v\n", err)
		return 1
	}
	report, err := buildReport(*cDirFlag, goFunctions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build report: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir output dir: %v\n", err)
		return 1
	}
	if err := os.WriteFile(outPath, []byte(report), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		return 1
	}

	fmt.Printf("Wrote parity audit to %s\n", outPath)
	return 0
}

func buildReport(cDir string, goFunctions map[string]struct{}) (string, error) {
	var b strings.Builder
	b.WriteString("# Comprehensive C-to-Go Parity Audit\n\n")
	b.WriteString("Auto-generated analysis comparing the original Ironwail C codebase to the Go port.\n\n")

	for _, cFile := range cFiles {
		fmt.Fprintf(&b, "### Analysis of `%s`\n\n", cFile)
		cPath := filepath.Join(cDir, cFile)
		data, err := os.ReadFile(cPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(&b, "File not found: %s\n\n", cFile)
				continue
			}
			return "", fmt.Errorf("read %s: %w", cPath, err)
		}

		found := 0
		missing := make([]string, 0)
		for _, cFunc := range collectCFunctionsFromSource(string(data)) {
			searchName := normalizeSearchName(cFunc)
			if searchName == "" {
				continue
			}
			if hasMappedGoFunction(searchName, goFunctions) {
				found++
				continue
			}
			missing = append(missing, cFunc)
		}
		slices.Sort(missing)
		missing = slices.Compact(missing)
		for _, name := range missing {
			fmt.Fprintf(&b, "- [ ] `%s` is missing or heavily refactored.\n", name)
		}
		fmt.Fprintf(&b, "\nFound %d mapped functions. Missing: %d.\n\n", found, len(missing))
	}

	return b.String(), nil
}

func collectGoFunctions(goDir string) (map[string]struct{}, error) {
	functions := make(map[string]struct{})
	err := filepath.WalkDir(goDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".beads":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, name := range collectGoFunctionsFromSource(string(data)) {
			functions[name] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return functions, nil
}

func collectGoFunctionsFromSource(src string) []string {
	matches := goFuncPattern.FindAllStringSubmatch(src, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		out = append(out, strings.ToLower(match[1]))
	}
	return out
}

func collectCFunctionsFromSource(src string) []string {
	out := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(src))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.Contains(line, ";") || !strings.Contains(line, "(") {
			continue
		}
		match := cFuncPattern.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}
		name := match[1]
		if _, skip := cKeywords[name]; skip {
			continue
		}
		out = append(out, name)
	}
	return out
}

func normalizeSearchName(name string) string {
	trimmed := name
	for _, prefix := range cPrefixes {
		if strings.HasPrefix(strings.ToLower(trimmed), strings.ToLower(prefix)) {
			trimmed = trimmed[len(prefix):]
			break
		}
	}
	return strings.ToLower(strings.ReplaceAll(trimmed, "_", ""))
}

func hasMappedGoFunction(searchName string, goFunctions map[string]struct{}) bool {
	for goFunc := range goFunctions {
		if strings.Contains(goFunc, searchName) {
			return true
		}
	}
	return false
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
