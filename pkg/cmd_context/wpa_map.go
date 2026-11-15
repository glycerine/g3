package cmd_context

// Source: src/muz/fp/datalog_parser.cpp wpa_parser_impl::parse_map_file and
// line_reader.

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func joinClose(err error, closeErr error) error { return errors.Join(err, closeErr) }

// readWPALines is line_reader: the file split at every '\n', including the
// (possibly empty) piece after the last one. Each line is a C string, so it
// ends at its first NUL byte.
func readWPALines(path string) ([]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(contents), "\n")
	for i, line := range lines {
		if end := strings.IndexByte(line, 0); end >= 0 {
			lines[i] = line[:end]
		}
	}
	return lines, nil
}

func (p *wpaParser) readMaps() error {
	for _, path := range p.files("map") {
		if err := p.parseMapFile(path); err != nil {
			return err
		}
	}
	return nil
}

// parseMapFile is parse_map_file.
func (p *wpaParser) parseMapFile(fname string) error {
	if err := wpaVerbose("Parsing map file " + fname + "\n"); err != nil {
		return err
	}
	p.currentFile, p.currentLine = fname, 0
	sortElements := p.ensureSortContent(wpaBase(fname))
	lines, err := readWPALines(fname)
	if err != nil {
		return err
	}
	for _, line := range lines {
		p.currentLine++
		number, name, present, parseErr := parseWPAMapLineWithNames(line, p.useMapNames)
		switch parseErr {
		case nil:
		case errWPANumberExpected:
			return fmt.Errorf("number expected at line %d in file %s", p.currentLine, p.currentFile)
		case errWPASpaceExpected:
			return fmt.Errorf("' ' expected after the number at line %d in file %s", p.currentLine, p.currentFile)
		default:
			return parseErr
		}
		if !present {
			continue
		}
		sortElements.numbers[number] = true
		if p.useMapNames {
			// num2sym::insert_if_not_there2 keeps the first name; a
			// different later spelling only warns.
			old, found := p.mapNames[number]
			if !found {
				p.mapNames[number], old = name, name
			}
			if old != name {
				if err = diagnostic.BorrowProcessWarningService().Warningf("mismatch of number names on line %d in file %s. old: \"%s\" new: \"%s\"", p.currentLine, fname, old, name); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
