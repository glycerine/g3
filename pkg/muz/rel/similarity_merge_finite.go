package rel

// Source: dl_mk_similarity_compressor.cpp merge_class, finite table path.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// similarityMergeFinite publishes auxiliary facts to the existing finite
// relation manager and returns the caller-owned representative rule. Domains
// not supported by that manager return an error before predicate creation.
func similarityMergeFinite(context *base.DatalogContext, relations *RelationManager, class similarityClass) (ast.ASTHandle, *base.Rule, error) {
	if context == nil || relations == nil || !class.merge || len(class.rules) < 2 || len(class.constants) == 0 {
		return ast.ASTHandle{}, nil, ast.ErrASTDomain
	}
	m, e := context.Manager()
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	if e = relations.open(); e != nil {
		return ast.ASTHandle{}, nil, e
	}
	if relations.manager != m {
		return ast.ASTHandle{}, nil, ast.ErrASTDomain
	}
	rm, e := context.RuleManager()
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	ruleManager, e := class.rules[0].ASTManager()
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	if ruleManager != m {
		return ast.ASTHandle{}, nil, ast.ErrASTDomain
	}
	domains, rows, e := similarityOrphanRows(class.rules, class.constants)
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	// Validate and encode before registering a predicate so an unsupported
	// relation domain does not leave an unusable generated declaration behind.
	facts := make([]Fact, len(rows))
	for i, row := range rows {
		facts[i], e = relations.EncodeFact(domains, row)
		if e != nil {
			return ast.ASTHandle{}, nil, e
		}
	}
	head, e := class.rules[0].HeadDeclaration()
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	prefix, e := m.GetASTName(head)
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	suffix, e := m.Symbols.Intern(fmt.Sprintf("sc_%d", len(class.constants)))
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	aux, e := context.FreshHeadPredicate(prefix, suffix, domains)
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	table, e := relations.GetRelation(aux)
	if e != nil {
		return aux, nil, e
	}
	for _, fact := range facts {
		if _, e = table.Add(fact); e != nil {
			return aux, nil, e
		}
	}
	if e = relations.MarkSaturated(aux); e != nil {
		return aux, nil, e
	}
	rule, e := similarityRewriteRule(rm, class.rules[0], class.constants, aux)
	return aux, rule, e
}
