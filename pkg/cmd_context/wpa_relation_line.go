package cmd_context

import (
	"fmt"
	"strconv"
	"strings"
)

func parseWPARelationLine(line string) ([]uint64, bool, error) {
	if comment := strings.IndexByte(line, '#'); comment >= 0 {
		line = line[:comment]
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, false, nil
	}
	values := make([]uint64, len(fields))
	for i, field := range fields {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil, false, fmt.Errorf("invalid relation element %q", field)
		}
		values[i] = value
	}
	return values, true, nil
}
