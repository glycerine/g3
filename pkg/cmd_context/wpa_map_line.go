package cmd_context

import (
	"fmt"
	"strconv"
	"strings"
)

func parseWPAMapLine(line string) (uint64, string, bool, error) {
	if comment := strings.IndexByte(line, '#'); comment >= 0 {
		line = line[:comment]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, "", false, nil
	}
	space := strings.IndexByte(line, ' ')
	if space < 0 {
		return 0, "", false, fmt.Errorf("map line requires a number and name")
	}
	number, err := strconv.ParseUint(line[:space], 10, 64)
	if err != nil {
		return 0, "", false, fmt.Errorf("invalid symbol number %q", line[:space])
	}
	name := strings.TrimSpace(line[space+1:])
	for _, marker := range []string{" SC_EXTERN ", " _ZONE_"} {
		if cut := strings.Index(name, marker); cut >= 0 {
			name = name[:cut]
		}
	}
	name = strings.TrimSpace(name)
	name = strings.TrimSpace(strings.TrimSuffix(name, "Constant"))
	if name == "" {
		return 0, "", false, fmt.Errorf("empty symbol name")
	}
	return number, name, true, nil
}
