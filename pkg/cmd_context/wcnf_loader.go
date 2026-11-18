package cmd_context

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/numeral"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type WCNFLoadSummary struct{ Variables, HardClauses, SoftClauses int }

func (c *Context) LoadWCNF(document opt.WCNFDocument) (WCNFLoadSummary, error) {
	if c == nil {
		return WCNFLoadSummary{}, fmt.Errorf("wcnf: nil command context")
	}
	_, err := c.Manager()
	if err != nil {
		return WCNFLoadSummary{}, err
	}
	// wcnf::wcnf: opt.set_clausal(true).
	c.optimizationClausal = true
	decls := make(map[uint32]ast.ASTHandle)
	summary := WCNFLoadSummary{}
	for i := range document.Clauses {
		clause, e := c.makeWCNFClause(document.Clauses[i].Literals, decls)
		if e != nil {
			return WCNFLoadSummary{}, fmt.Errorf("wcnf: clause %d: %w", i+1, e)
		}
		if document.Clauses[i].Weight >= document.TopWeight {
			e = c.Assert(clause, "")
			summary.HardClauses++
		} else {
			weight := numeral.NewRational()
			q := c.manager.NumeralService().Manager
			if e = q.SetString(&weight, strconv.FormatUint(document.Clauses[i].Weight, 10)); e == nil {
				e = c.AddSoftConstraint(clause, &weight, "")
			}
			e = errors.Join(e, q.Del(&weight))
			summary.SoftClauses++
		}
		if e != nil {
			return WCNFLoadSummary{}, fmt.Errorf("wcnf: clause %d: %w", i+1, e)
		}
	}
	// Native parsing creates only variables reached by clauses.
	summary.Variables = len(decls)
	return summary, nil
}

func (c *Context) makeWCNFClause(literals []opt.WCNFLiteral, decls map[uint32]ast.ASTHandle) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	terms := make([]ast.ASTHandle, len(literals))
	for i, literal := range literals {
		// read_clause: m.mk_const(symbol(var), m.mk_bool_sort()), a
		// numerical symbol (printed k!var) and no cmd_context declaration.
		sort, e := m.MkBoolSort()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		terms[i], err = m.MkConst(usymbol.Numerical(literal.Variable), sort)
		if err == nil {
			decls[literal.Variable] = terms[i]
		}
		if err == nil && literal.Negative {
			terms[i], err = m.MkNot(terms[i])
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return m.MkOrArray(terms)
}
