package fp

import "strings"

// withoutTransformLines drops the rule_transformer::operator() IF_VERBOSE(1)
// "(transform <typeid>...)" lines (their timings vary) from captured
// verbose output.
func withoutTransformLines(text string) string {
	var kept strings.Builder
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.HasPrefix(line, "(transform ") {
			continue
		}
		kept.WriteString(line)
	}
	return kept.String()
}

// transformFilteredString reads a captured verbose stream without the
// "(transform ...)" lines.
func transformFilteredString(stream interface{ String() (string, error) }) (string, error) {
	text, err := stream.String()
	return withoutTransformLines(text), err
}
