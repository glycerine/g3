package api

// Source: src/api/api_ast_map.cpp:139-157.

import (
	"bytes"
)

// String is Z3_ast_map_to_string: keys and values through
// mk_ismt2_pp(n, m, 3).
func (m *ASTMap) String() (string, error) {
	if err := m.check(); err != nil {
		return "", err
	}
	keys, err := m.object.keys()
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	out.WriteString("(ast-map")
	for _, key := range keys {
		value, err := m.object.find(key)
		if err != nil {
			return "", err
		}
		out.WriteString("\n  (")
		text, err := m.owner.ismt2PP(key, 3)
		if err != nil {
			return "", err
		}
		out.WriteString(text)
		out.WriteString("\n   ")
		if text, err = m.owner.ismt2PP(value, 3); err != nil {
			return "", err
		}
		out.WriteString(text)
		out.WriteByte(')')
	}
	out.WriteByte(')')
	return out.String(), nil
}
