package base

import "github.com/glycerine/g3/pkg/ast"

// ReplaceRuleHead is the source copy constructor with a specified head. The
// tail, signs, and positive/uninterpreted boundaries are copied verbatim.
func (m *RuleManager) ReplaceRuleHead(source *Rule, head ast.ASTHandle, name string) (*Rule, error) {
	return m.cloneRuleState(source, head, name)
}

// ReplaceRuleHeadNamed preserves the source rule name.
func (m *RuleManager) ReplaceRuleHeadNamed(source *Rule, head ast.ASTHandle) (*Rule, error) {
	name, err := source.Name()
	if err != nil {
		return nil, err
	}
	return m.ReplaceRuleHead(source, head, name)
}
