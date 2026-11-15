package normal_forms

// Source: src/ast/normal_forms/name_exprs.cpp:24-151;
// src/ast/normal_forms/name_exprs.h:24-62.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ExprPredicate selects expressions that should be replaced by defined names.
type ExprPredicate interface {
	NameExpression(ast.ASTHandle) (bool, error)
}

type ExprPredicateFunc func(ast.ASTHandle) (bool, error)

func (f ExprPredicateFunc) NameExpression(h ast.ASTHandle) (bool, error) { return f(h) }

// nameExprsConfiguration is name_exprs_core::cfg: a default rewriter
// configuration whose get_subst names every expression the predicate selects.
type nameExprsConfiguration struct {
	rewriter.DefaultConfiguration
	manager   *ast.ASTManager
	names     *DefinedNames
	predicate ExprPredicate
	// m_r and m_pr hold the last name and its proof for the traversal.
	r, pr ast.ASTRef
	// m_def_exprs and m_def_proofs, set by each Rewrite call.
	definitions, definitionProofs *[]ast.ASTRef
}

// genNameForExpr is cfg::gen_name_for_expr.
func (c *nameExprsConfiguration) genNameForExpr(h ast.ASTHandle) (value, proof ast.ASTHandle, result error) {
	m := c.manager
	newDef, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	newDefProof, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, errors.Join(e, m.DelRef(&newDef))
	}
	defer func() {
		if newDef != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&newDef))
		}
		if newDefProof != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&newDefProof))
		}
	}()
	created, e := c.names.MkName(h, newDef, newDefProof, c.r, c.pr)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	proofs, e := m.ProofsEnabled()
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	if created {
		*c.definitions = append(*c.definitions, newDef)
		newDef = ast.ASTRef{}
		if proofs {
			*c.definitionProofs = append(*c.definitionProofs, newDefProof)
			newDefProof = ast.ASTRef{}
		}
	}
	if value, e = m.RefGet(c.r); e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	proof, e = m.RefGet(c.pr)
	return value, proof, e
}

// GetSubst is cfg::get_subst.
func (c *nameExprsConfiguration) GetSubst(s ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	selected, e := c.predicate.NameExpression(s)
	if e != nil || !selected {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	value, proof, e := c.genNameForExpr(s)
	return value, proof, e == nil, e
}

// ExprNamer is name_exprs_core: a rewriter_tpl over nameExprsConfiguration
// that replaces selected subexpressions with names and appends each new
// definition and optional def-intro proof to caller-owned reference slices.
type ExprNamer struct {
	config    nameExprsConfiguration
	traversal *rewriter.Traversal
	proofs    bool
	root      ast.ASTHandle
	closed    bool
}

func newExprNamer(m *ast.ASTManager, names *DefinedNames) (*ExprNamer, error) {
	if m == nil || names == nil || names.manager != m || names.valid() != nil {
		return nil, ast.ErrASTHandle
	}
	n := &ExprNamer{}
	n.config.manager = m
	n.config.names = names
	var e error
	if n.proofs, e = m.ProofsEnabled(); e != nil {
		return nil, e
	}
	if n.config.r, e = m.NewRef(ast.ASTHandle{}); e != nil {
		return nil, e
	}
	if n.config.pr, e = m.NewRef(ast.ASTHandle{}); e != nil {
		return nil, errors.Join(e, n.Close())
	}
	if n.proofs {
		n.traversal, e = rewriter.NewProofTraversal(m, &n.config)
	} else {
		n.traversal, e = rewriter.NewTraversal(m, &n.config)
	}
	if e != nil {
		return nil, errors.Join(e, n.Close())
	}
	return n, nil
}

// NewExprNamer is mk_expr_namer.
func NewExprNamer(m *ast.ASTManager, names *DefinedNames, predicate ExprPredicate) (*ExprNamer, error) {
	if predicate == nil {
		return nil, ast.ErrASTHandle
	}
	n, e := newExprNamer(m, names)
	if e != nil {
		return nil, e
	}
	n.config.predicate = predicate
	return n, nil
}

// NewQuantifierLabelNamer is mk_quantifier_label_namer.
func NewQuantifierLabelNamer(m *ast.ASTManager, names *DefinedNames) (*ExprNamer, error) {
	return NewExprNamer(m, names, ExprPredicateFunc(func(h ast.ASTHandle) (bool, error) {
		kind, e := m.GetKind(h)
		if e != nil || kind == ast.ASTQuantifier {
			return kind == ast.ASTQuantifier, e
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		var positive bool
		var labels []usymbol.Symbol
		return m.IsLabel(h, &positive, &labels)
	}))
}

// NewNestedFormulaNamer is mk_nested_formula_namer: every basic-family
// application with arguments other than the root, and every quantifier.
func NewNestedFormulaNamer(m *ast.ASTManager, names *DefinedNames) (*ExprNamer, error) {
	n, e := newExprNamer(m, names)
	if e != nil {
		return nil, e
	}
	n.config.predicate = ExprPredicateFunc(func(h ast.ASTHandle) (bool, error) {
		kind, e := m.GetKind(h)
		if e != nil {
			return false, e
		}
		if kind == ast.ASTApp {
			family, e := m.GetFamilyID(h)
			if e != nil {
				return false, e
			}
			count, e := m.GetNumArgs(h)
			return family == 0 && count > 0 && h != n.root, e
		}
		return kind == ast.ASTQuantifier, nil
	})
	return n, nil
}

func (n *ExprNamer) open() error {
	if n == nil || n.closed || n.traversal == nil || n.config.names == nil || n.config.names.valid() != nil {
		return ast.ErrASTHandle
	}
	return nil
}

// Rewrite is name_exprs_core::operator(): it appends outputs; the caller owns
// every appended ASTRef and must release it through the manager.
func (n *ExprNamer) Rewrite(root ast.ASTHandle, definitions, definitionProofs *[]ast.ASTRef, out, proofOut ast.ASTRef) error {
	if e := n.open(); e != nil {
		return e
	}
	if definitions == nil || definitionProofs == nil {
		return ast.ErrASTHandle
	}
	n.root = root
	n.config.definitions = definitions
	n.config.definitionProofs = definitionProofs
	defer func() {
		n.config.definitions = nil
		n.config.definitionProofs = nil
	}()
	if n.proofs {
		return n.traversal.RewriteWithProof(root, out, proofOut)
	}
	return n.traversal.Rewrite(root, out)
}

func (n *ExprNamer) Reset() error {
	if e := n.open(); e != nil {
		return e
	}
	return n.traversal.Reset()
}

func (n *ExprNamer) Close() error {
	if n == nil {
		return ast.ErrASTHandle
	}
	if n.closed {
		return nil
	}
	m := n.config.manager
	var err error
	if n.traversal != nil {
		if err = n.traversal.Close(); err != nil {
			return err
		}
		n.traversal = nil
	}
	if n.config.pr != (ast.ASTRef{}) {
		err = errors.Join(err, m.DelRef(&n.config.pr))
	}
	if n.config.r != (ast.ASTRef{}) {
		err = errors.Join(err, m.DelRef(&n.config.r))
	}
	if err != nil {
		return err
	}
	n.closed = true
	return nil
}
