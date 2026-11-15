package cmd_context

// Source: src/muz/base/dl_util.cpp get_file_names, POSIX branch of the
// fork (FORK FIX #66, AUDIT.md cmd_context_02-F11; z3 4.7.1 has only the
// Windows branch and is NOT_IMPLEMENTED_YET elsewhere).

import (
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/muz/base"
)

// getFileNames appends to res the files of directory whose names end with
// "."+extension, then, when traverseSubdirs, those of every entry not
// starting with '.', recursively (a non-directory lists nothing). Names
// are visited in sorted byte order. A directory named *.extension is not
// listed as a file. An unreadable directory lists nothing.
func getFileNames(directory, extension string, traverseSubdirs bool, res []string) []string {
	if last := directory[len(directory)-1]; last != '\\' && last != '/' {
		directory += "/"
	}
	// os.ReadDir sorts by file name, the order of the fork's std::sort.
	entries, err := os.ReadDir(directory)
	if err != nil && len(entries) == 0 {
		return res
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	for _, name := range names {
		if len(name) > len(extension) && name[len(name)-len(extension)-1] == '.' &&
			strings.HasSuffix(name, extension) && !wpaIsDirectory(directory+name) {
			res = append(res, directory+name)
		}
	}
	if traverseSubdirs {
		for _, name := range names {
			if name[0] == '.' {
				continue
			}
			res = getFileNames(directory+name, extension, traverseSubdirs, res)
		}
	}
	return res
}

// wpaIsDirectory is dl_util.cpp is_directory: stat follows symbolic links.
func wpaIsDirectory(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}

func (p *wpaParser) files(extension string) []string {
	return getFileNames(p.directory, extension, true, nil)
}

func wpaBase(path string) string {
	return base.FileNameWithoutExtension(path)
}
