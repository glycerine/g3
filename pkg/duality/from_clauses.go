package duality

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// FromClauses constructs the source RPFP from Horn formulas using the active
// USE_QE_LITE pipeline. Services and input expressions are borrowed. On failure
// any partially constructed nodes/edges remain owned by g and can be closed.
// Bounds, when supplied, must contain one entry per input clause.
// Source: duality_rpfp.cpp, FromClauses.
func (g *RPFP) FromClauses(clauses []*Expr, bounds []uint32, simplifier *interp.Manager, quantifiers *qe.Lite) (result error) {
	if err := g.open(); err != nil {
		return err
	}
	if bounds != nil && len(bounds) != len(clauses) {
		return errors.New("duality: clause bound count mismatch")
	}
	if simplifier == nil || quantifiers == nil {
		return ErrWrapperOwner
	}
	m := g.context.manager
	manager, err := simplifier.ASTManager()
	if err != nil {
		return err
	}
	if manager != m {
		return ErrContextMismatch
	}
	for _, clause := range clauses {
		if err := clause.open(); err != nil {
			return err
		}
		if clause.value.object.context != g.context {
			return ErrContextMismatch
		}
	}
	// The fixedpoint interface owns the original clauses for proof extraction.
	// Associate edges before pruning, which can remove earlier clause positions.
	base := len(g.sourceClauses)
	for _, clause := range clauses {
		ref, err := m.CopyRef(clause.value.ref)
		if err != nil {
			return err
		}
		g.sourceClauses = append(g.sourceClauses, ref)
	}
	if g.clauseIndices == nil {
		g.clauseIndices = make(map[*Edge]int)
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	// fail_pred = ctx.fresh_func_decl("@Fail", ctx.bool_sort()): only the
	// declaration; the query node later applies it.
	failName, err := m.Symbols.Intern("@Fail")
	if err != nil {
		return err
	}
	failDecl, err := m.MkFreshFuncDecl(failName, usymbol.Symbol{}, nil, boolean)
	if err != nil {
		return err
	}
	failRef, err := m.NewRef(failDecl)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&failRef)) }()
	predicate := func(head *Expr) (ast.ASTHandle, bool, error) {
		h, err := head.Raw()
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		family, err := m.GetFamilyID(h)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if family == m.BasicFamilyID() && op == ast.ASTOpFalse {
			return failDecl, true, nil
		}
		d, err := m.GetDecl(h)
		return d, false, err
	}
	prepared := make([]*preparedHornClause, 0, len(clauses))
	defer func() {
		for _, p := range prepared {
			result = errors.Join(result, p.close())
		}
	}()
	predicates := make(map[ast.ASTHandle]*Node)
	for _, clause := range clauses {
		p, err := prepareHornClause(clause, simplifier)
		if err != nil {
			return err
		}
		prepared = append(prepared, p)
		decl, query, err := predicate(p.Head)
		if err != nil {
			return err
		}
		if predicates[decl] != nil {
			p.Parent = predicates[decl]
			continue
		}
		normalized, err := normalizeClauseHead(p.Head, nil)
		if err != nil {
			return err
		}
		// Indparams lives to the end of this block; the R(Indparams) argument
		// of CreateNode is a temporary of that statement.
		var indparams []*Expr
		closeIndparams := func() (cause error) {
			for _, e := range indparams {
				cause = errors.Join(cause, e.Close())
			}
			indparams = nil
			return cause
		}
		args := make([]ast.ASTHandle, len(normalized.Parameters))
		for i, param := range normalized.Parameters {
			value, err := g.SubstBound(p.Replacements, param)
			if value != nil {
				indparams = append(indparams, value)
			}
			if err == nil {
				args[i], err = value.Raw()
			}
			if err != nil {
				return errors.Join(err, normalized.close(), closeIndparams())
			}
		}
		app, err := m.MkApp(decl, args)
		if err != nil {
			return errors.Join(err, normalized.close(), closeIndparams())
		}
		term, err := NewExpr(g.context, app)
		if err != nil {
			return errors.Join(err, normalized.close(), closeIndparams())
		}
		node, createErr := g.CreateNode(term)
		createErr = errors.Join(createErr, term.Close())
		if err := errors.Join(createErr, normalized.close()); err != nil {
			return errors.Join(err, closeIndparams())
		}
		predicates[decl] = node
		p.Parent = node
		if query {
			if err := node.Bound.SetEmpty(); err != nil {
				return err
			}
		}
		if bounds != nil {
			node.RecursionBound = 0
		}
		if err := closeIndparams(); err != nil {
			return err
		}
	}
	someLabels := false
	for i, p := range prepared {
		if err := g.clauseEdge(p, predicates, bounds, i, base+i, simplifier, quantifiers, &someLabels); err != nil {
			return err
		}
	}
	if err := g.RemoveDeadNodes(); err != nil {
		return err
	}
	return g.Unhoist()
}

func (g *RPFP) clauseEdge(p *preparedHornClause, predicates map[ast.ASTHandle]*Node, bounds []uint32, index, originalIndex int, simplifier *interp.Manager, quantifiers *qe.Lite, someLabels *bool) (result error) {
	m := g.context.manager
	var expressions []*Expr
	hold := func(e *Expr, err error) (*Expr, error) {
		if e != nil {
			expressions = append(expressions, e)
		}
		return e, err
	}
	defer func() {
		for _, e := range expressions {
			result = errors.Join(result, e.Close())
		}
	}()
	normalized, err := normalizeClauseHead(p.Head, p.Body)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, normalized.close()) }()
	scanned, err := g.scanBodyTheoryQuantifiers(normalized.Body, predicates)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, scanned.Close()) }()
	body, labels, err := g.RemoveLabels(scanned.Term)
	if err != nil {
		return err
	}
	expressions = append(expressions, body)
	defer func() {
		for i := range labels {
			result = errors.Join(result, labels[i].Close())
		}
	}()
	original, err := scanned.Term.Raw()
	if err != nil {
		return err
	}
	stripped, err := body.Raw()
	if err != nil {
		return err
	}
	if stripped != original {
		*someLabels = true
	}
	ref, err := simplifier.Simplify(stripped)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	if !*someLabels {
		indices := []uint32{}
		seen := make(map[uint32]bool)
		for _, param := range normalized.Parameters {
			h, err := param.Raw()
			if err != nil {
				return err
			}
			kind, err := m.GetKind(h)
			if err != nil {
				return err
			}
			if kind == ast.ASTVar {
				idx, err := m.GetVarIndex(h)
				if err != nil {
					return err
				}
				if !seen[idx] {
					seen[idx] = true
					indices = append(indices, idx)
				}
			}
		}
		sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
		// expr::qe_lite constructs ::qe_lite qe(m(), params_ref()) per call,
		// so its rewriters take the proof mode current at this point.
		local, err := qe.NewLite(m, nil, true, quantifiers.Config())
		if err != nil {
			return err
		}
		err = local.EliminateIndices(indices, false, ref)
		if err = errors.Join(err, local.Close()); err != nil {
			return err
		}
	}
	h, err := m.RefGet(ref)
	if err != nil {
		return err
	}
	simplified, err := hold(NewExpr(g.context, h))
	if err != nil {
		return err
	}
	substituted, err := hold(g.SubstBound(p.Replacements, simplified))
	if err != nil {
		return err
	}
	labeled := scanned.Term
	if *someLabels {
		labeled, err = hold(g.SubstBound(p.Replacements, labeled))
		if err != nil {
			return err
		}
	}
	params := make([]*Expr, len(normalized.Parameters))
	for i, param := range normalized.Parameters {
		params[i], err = hold(g.SubstBound(p.Replacements, param))
		if err != nil {
			return err
		}
	}
	transformer, err := NewTransformer(g.context, scanned.Parameters, params, substituted)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, transformer.Close()) }()
	parent := p.Parent
	if parent == nil {
		return ErrWrapperOwner
	}
	edge, err := g.CreateEdge(parent, transformer, scanned.Children)
	if err != nil {
		return err
	}
	edge.quantifiedTheory = scanned.QuantifiedTheory
	labeledRaw, err := labeled.Raw()
	if err != nil {
		return err
	}
	edge.labeled, err = m.NewRef(labeledRaw)
	if err != nil {
		return err
	}
	g.clauseIndices[edge] = originalIndex
	if bounds != nil && bounds[index] > parent.RecursionBound {
		parent.RecursionBound = bounds[index]
	}
	return nil
}
