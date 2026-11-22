package text

// Source: src/util/symbol.cpp; src/util/params.cpp.

import "strings"

func CStringLength(text string) int {
	if i := strings.IndexByte(text, 0); i >= 0 {
		return i
	}
	return len(text)
}

// EqualCString compares a NUL-free spelling with the C string in input.
// The first byte after a matching spelling must terminate the string.
func EqualCString(spelling, input string) bool {
	return spelling == input || len(input) > len(spelling) && input[len(spelling)] == 0 && input[:len(spelling)] == spelling
}
