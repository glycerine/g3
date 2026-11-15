package cmd_context

// Source: src/muz/fp/datalog_parser.cpp wpa_parser_impl::cut_off_comment and
// parse_map_line.

import (
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/muz/base"
)

// The two map-line failures; parse_map_file formats them with the line and
// file, as the source exceptions do.
var (
	errWPANumberExpected = errors.New("number expected")
	errWPASpaceExpected  = errors.New("' ' expected")
)

// wpaCutOffComment is cut_off_comment: the line ends at '#', '\n' or '\r'.
func wpaCutOffComment(line string) string {
	if i := strings.IndexAny(line, "#\n\r"); i >= 0 {
		return line[:i]
	}
	return line
}

func parseWPAMapLine(line string) (uint64, string, bool, error) {
	return parseWPAMapLineWithNames(line, true)
}

// parseWPAMapLineWithNames is parse_map_line; present=false is its false
// return for an empty line.
func parseWPAMapLineWithNames(line string, useNames bool) (uint64, string, bool, error) {
	line = wpaCutOffComment(line)
	if len(line) == 0 {
		return 0, "", false, nil
	}
	number, next, ok := base.ReadUint64(line, 0)
	if !ok {
		return 0, "", false, errWPANumberExpected
	}
	if next >= len(line) || line[next] != ' ' {
		return 0, "", false, errWPASpaceExpected
	}
	if !useNames {
		return number, "<names ignored>", true, nil
	}
	name := line[next+1:]
	for _, marker := range []string{" SC_EXTERN ", " _ZONE_"} {
		if cut := strings.Index(name, marker); cut >= 0 {
			name = name[:cut]
		}
	}
	if len(name) > len("Constant ") && strings.HasSuffix(name, "Constant ") {
		name = strings.TrimSuffix(name, "Constant ")
	}
	// z3 4.7.1 tests rest_of_line[rest_of_line.size()-1] even when the name
	// is empty ("23 " or "23  SC_EXTERN x"), undefined behavior. The fork
	// tests for empty first (FORK FIX #69): the name is the empty symbol.
	name = strings.TrimSuffix(name, " ")
	return number, name, true, nil
}
