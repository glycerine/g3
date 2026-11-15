package cmd_context

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// get_file_names (fork POSIX branch, AUDIT.md cmd_context_02-F11): a
// directory's own *.map files in sorted order (hidden files included, a
// directory named *.map excluded), then each subdirectory not starting with
// '.', recursively.
func TestWPAFileDiscoveryRecursesAndSkipsHiddenDirectories(t *testing.T) {
	directory := t.TempDir()
	for path, contents := range map[string]string{
		"root.map":                 "",
		"b.map":                    "",
		".h.map":                   "",
		"xmap":                     "",
		"e.mapx":                   "",
		"a0/w.map":                 "",
		"nested/child.map":         "",
		"nested/deeper/y.map":      "",
		".hidden/secret.map":       "",
		"nested/case-mismatch.MAP": "",
		"dir.map/inner.map":        "",
	} {
		fullPath := filepath.Join(directory, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	parser := &wpaParser{directory: directory}
	files := parser.files("map")
	want := []string{
		directory + "/.h.map", directory + "/b.map", directory + "/root.map",
		directory + "/a0/w.map", directory + "/dir.map/inner.map",
		directory + "/nested/child.map", directory + "/nested/deeper/y.map",
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files=%v want=%v", files, want)
	}
	parser.directory = directory + "/"
	if files = parser.files("map"); files[0] != directory+"/.h.map" {
		t.Fatalf("a trailing separator was doubled: %v", files)
	}
}

func TestWPAFileBaseMatchesDatalogExtensionRule(t *testing.T) {
	for path, want := range map[string]string{
		"/tmp/predicate.rel":    "predicate",
		"/tmp/.rel":             ".rel",
		`C:\data\predicate.rel`: "predicate",
	} {
		if got := wpaBase(path); got != want {
			t.Errorf("wpaBase(%q) = %q, want %q", path, got, want)
		}
	}
}
