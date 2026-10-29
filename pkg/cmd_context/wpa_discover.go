package cmd_context

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

func (p *wpaParser) files(extension string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(p.directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), "."+extension) {
			result = append(result, path)
		}
		return nil
	})
	sort.Strings(result)
	return result, err
}

func wpaBase(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
