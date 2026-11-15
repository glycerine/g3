package cmd_context

// Source: src/muz/fp/datalog_parser.cpp wpa_parser_impl::parse_rel_line.

import "github.com/glycerine/g3/pkg/muz/base"

// parseWPARelationLine is parse_rel_line. present=false is its false return
// for an empty line. On errWPASpaceExpected, rest is the text from the
// offending character on, which the source message quotes.
func parseWPARelationLine(line string) (values []uint64, present bool, rest string, err error) {
	line = wpaCutOffComment(line)
	if len(line) == 0 {
		return nil, false, "", nil
	}
	offset := 0
	for {
		for offset < len(line) && line[offset] == ' ' {
			offset++
		}
		if offset == len(line) {
			return values, true, "", nil
		}
		value, next, ok := base.ReadUint64(line, offset)
		if !ok {
			return nil, false, "", errWPANumberExpected
		}
		if next < len(line) && line[next] != ' ' {
			return nil, false, line[next:], errWPASpaceExpected
		}
		values = append(values, value)
		offset = next
	}
}
