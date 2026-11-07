package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glycerine/g3/pkg/portcheck"
)

func main() {
	full := flag.Bool("f", false, "also list associated Go source files after each C++ file")
	manifest := flag.String("manifest", "", "port manifest path (searched for in the current directory and its parents when omitted)")
	flag.Parse()

	path := *manifest
	if path == "" {
		var err error
		path, err = findManifest()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := listPartial(os.Stdout, path, *full); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func findManifest() (string, error) {
	dir, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "port_manifest.json")
		if info, statErr := os.Stat(candidate); statErr == nil {
			if info.IsDir() {
				return "", fmt.Errorf("manifest path is a directory: %s", candidate)
			}
			return candidate, nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("port_manifest.json not found in current directory or its parents; pass -manifest")
}

func listPartial(output io.Writer, manifestPath string, full bool) error {
	manifest, err := portcheck.Load(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	goByCPP := make(map[string]map[string]struct{})
	addGoFiles := func(cppPath string, targets ...string) {
		files := goByCPP[cppPath]
		if files == nil {
			files = make(map[string]struct{})
			goByCPP[cppPath] = files
		}
		for _, target := range targets {
			if strings.HasSuffix(target, ".go") {
				files[target] = struct{}{}
			}
		}
	}

	for _, unit := range manifest.Units {
		if unit.Status != "partial" || !isCPPFile(unit.Path) {
			continue
		}
		addGoFiles(unit.Path, unit.GoTargets...)
		for _, method := range unit.Methods {
			if method.GoTarget != "" {
				addGoFiles(unit.Path, method.GoTarget)
			}
			addGoFiles(unit.Path, method.GoTargets...)
		}
	}

	// API records can point to a partial C++ source or header independently of
	// the unit's aggregate target list. Include those Go implementation files too.
	for _, api := range manifest.APIs {
		for _, source := range api.Sources {
			if _, partial := goByCPP[source]; !partial {
				continue
			}
			targets := append([]string(nil), api.GoTargets...)
			if api.GoTarget != "" {
				targets = append(targets, api.GoTarget)
			}
			addGoFiles(source, targets...)
		}
	}

	cppFiles := make([]string, 0, len(goByCPP))
	for path := range goByCPP {
		cppFiles = append(cppFiles, path)
	}
	sort.Strings(cppFiles)

	for _, cppPath := range cppFiles {
		if _, err := fmt.Fprintln(output, cppPath); err != nil {
			return err
		}
		if !full {
			continue
		}
		goFiles := make([]string, 0, len(goByCPP[cppPath]))
		for path := range goByCPP[cppPath] {
			goFiles = append(goFiles, path)
		}
		sort.Strings(goFiles)
		for _, goPath := range goFiles {
			if _, err := fmt.Fprintf(output, "  %s\n", goPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func isCPPFile(path string) bool {
	return strings.HasSuffix(path, ".cpp") || strings.HasSuffix(path, ".h") || strings.HasSuffix(path, ".hpp")
}
