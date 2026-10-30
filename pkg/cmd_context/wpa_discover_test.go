package cmd_context

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWPAFileDiscoveryRecursesAndSkipsHiddenDirectories(t *testing.T) {
	directory := t.TempDir()
	for path, contents := range map[string]string{
		"root.map":                 "",
		"nested/child.map":         "",
		".hidden/secret.map":       "",
		"nested/case-mismatch.MAP": "",
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
	files, err := parser.files("map")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(directory, "nested", "child.map"), filepath.Join(directory, "root.map")}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files=%v want=%v", files, want)
	}
}
