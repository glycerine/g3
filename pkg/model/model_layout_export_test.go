package model

import "github.com/glycerine/g3/pkg/ast"

// ConstantDeclarationsInTableOrderForTest lists the m_interp keys in
// iteration order, for external tests.
func (md *ModelCore) ConstantDeclarationsInTableOrderForTest() []ast.ASTHandle {
	entries := md.constantEntries()
	result := make([]ast.ASTHandle, 0, len(entries))
	for _, entry := range entries {
		if !entry.value.IsNull() {
			result = append(result, entry.declaration)
		}
	}
	return result
}

// UniverseSortsInTableOrderForTest lists m_usort2universe's keys in
// iteration order, for external tests.
func (md *Model) UniverseSortsInTableOrderForTest() []ast.ASTHandle {
	return md.universeSorts()
}
