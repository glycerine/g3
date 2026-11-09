package smtcore

import "github.com/glycerine/g3/pkg/ast"

// testElements snapshots m_elems as term -> generation for assertions.
func (s *smtModelFinderInstantiationSet) testElements() map[ast.ASTHandle]uint32 {
	result := make(map[ast.ASTHandle]uint32)
	if s == nil || s.elements == nil {
		return result
	}
	for _, entry := range s.elements.Entries() {
		result[entry.Key] = uint32(entry.Num)
	}
	return result
}
