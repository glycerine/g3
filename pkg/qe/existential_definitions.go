package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// EliminateExistentialDefinitions replaces existentially bound variables defined
// by an equality in their conjunction. Equation solving, the occurs check and
// capture-avoiding substitution are performed by EqualityDER. The caller owns
// formula, which changes only when the traversal finishes an elimination.
func EliminateExistentialDefinitions(m *ast.ASTManager, config Config, formula ast.ASTRef) error {
	return rewriteVerificationDefinitions(m, config, formula, false)
}

// SimplifyVerificationCondition also instantiates a universal when its
// implication guards force its only bound variable to equal a ground term.
func SimplifyVerificationCondition(m *ast.ASTManager, config Config, formula ast.ASTRef) error {
	return rewriteVerificationDefinitions(m, config, formula, true)
}

func rewriteVerificationDefinitions(m *ast.ASTManager, config Config, formula ast.ASTRef, universals bool) (err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return ast.ErrASTHandle
	}
	original, err := m.RefGet(formula)
	if err != nil {
		return err
	}
	if original.IsNull() {
		return ast.ErrASTHandle
	}
	boolean, err := m.HasBoolSort(original)
	if err != nil {
		return err
	}
	if !boolean {
		return ast.ErrASTDomain
	}
	quantified, err := m.HasQuantifiers(original)
	if err != nil || !quantified {
		return err
	}
	if m.Budget().InjectionArmed() || m.Limit.Limit != 0 || m.Limit.CancelFlag() || m.TraceStream != nil || ast.ASTSourceDebug || diagnostic.TraceCompiled() {
		return nil
	}
	proofs, err := m.ProofsEnabled()
	if err != nil || proofs {
		return err
	}
	active, err := config.Cooperation.Enabled()
	if err != nil || active {
		return err
	}
	verbose, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || verbose {
		return err
	}
	if m.NumeralService().RuntimeOwner().AllocationLimited() {
		return nil
	}
	cfg := definitionRewriter{m: m, services: config, remaining: 45000, universals: universals}
	rw, err := rewriter.NewTraversal(m, &cfg)
	if rw != nil {
		defer func() { err = errors.Join(err, rw.Close()) }()
	}
	if err != nil {
		return err
	}
	defer func() {
		if cfg.der != nil {
			err = errors.Join(err, cfg.der.Close())
		}
	}()
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&out)) }()
	if err = rw.Rewrite(original, out); err != nil {
		// A timer can fire inside DER or the enclosing traversal. Let the
		// ordinary check observe cancellation with the original goal.
		if m.Limit.CancelFlag() && err.Error() == m.Limit.CancelMessage() {
			var canceled tactic.TacticException
			if errors.As(err, &canceled) || rewriter.IsRewriterException(err) {
				return nil
			}
		}
		return err
	}
	if cfg.stopped || !cfg.changed || m.Limit.CancelFlag() {
		return nil
	}
	next, err := m.RefGet(out)
	if err != nil || next == original {
		return err
	}
	return m.SetRef(formula, next)
}

type definitionRewriter struct {
	rewriter.DefaultConfiguration
	m                *ast.ASTManager
	services         Config
	der              *EqualityDER
	remaining        uint32
	stopped, changed bool
	universals       bool
}

func (c *definitionRewriter) spend(n uint32) bool {
	if c.stopped || n > c.remaining || c.m.Limit.CancelFlag() {
		c.stopped = true
		return false
	}
	c.remaining -= n
	return true
}

func (*definitionRewriter) RewritePatterns() bool { return false }
func (c *definitionRewriter) PreVisit(h ast.ASTHandle) (bool, error) {
	if !c.spend(1) {
		return false, nil
	}
	m := c.m
	quantified, err := m.HasQuantifiers(h)
	if err != nil || !quantified {
		return false, err
	}
	hd, err := m.NodeHeader(h)
	if err != nil {
		return false, err
	}
	switch hd.Kind {
	case ast.ASTApp:
		return hd.Family == m.BasicFamilyID() && c.spend(hd.NumArgs), nil
	case ast.ASTQuantifier:
		return c.unannotated(h)
	default:
		return false, nil
	}
}

// Patterns belong to the search strategy selected by the caller. Leave the
// entire annotated quantifier intact, including terms nested in its body.
func (c *definitionRewriter) unannotated(q ast.ASTHandle) (bool, error) {
	m := c.m
	boolean, err := m.HasBoolSort(q)
	if err != nil || !boolean {
		return false, err
	}
	count, err := m.GetNumPatterns(q)
	if err != nil || count != 0 {
		return false, err
	}
	count, err = m.GetNumNoPatterns(q)
	return count == 0, err
}

func (c *definitionRewriter) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (_ bool, err error) {
	m := c.m
	if c.stopped || patterns.Count() != 0 || noPatterns.Count() != 0 {
		return false, nil
	}
	all, err := m.IsForall(q)
	if err != nil {
		return false, err
	}
	if all {
		if c.universals {
			return c.reduceGuardedUniversal(q, body, out)
		}
		return false, nil
	}
	n, err := m.GetNumDeclarations(q)
	if err != nil || !c.spend(n) {
		return false, err
	}
	weight, err := m.GetQuantifierWeight(q)
	if err != nil {
		return false, err
	}
	binders := []ast.ASTHandle{q}
	for {
		kind, err := m.GetKind(body)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTQuantifier {
			break
		}
		nested, err := c.unannotated(body)
		if err != nil {
			return false, err
		}
		if !nested {
			break
		}
		forall, err := m.IsForall(body)
		if err != nil {
			return false, err
		}
		if forall {
			break
		}
		w, err := m.GetQuantifierWeight(body)
		if err != nil {
			return false, err
		}
		qid, skid, err := m.GetQuantifierIDs(body)
		if err != nil {
			return false, err
		}
		if w != weight || !qid.IsNull() || !skid.IsNull() {
			break
		}
		more, err := m.GetNumDeclarations(body)
		if err != nil || !c.spend(more) {
			return false, err
		}
		n += more
		binders = append(binders, body)
		body, err = m.GetQuantifierBody(body)
		if err != nil {
			return false, err
		}
	}
	conjuncts, found, err := c.definingConjuncts(body, n)
	if err != nil || !found {
		return false, err
	}
	local, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&local)) }()
	flat := body
	if len(conjuncts) > 1 {
		flat, err = m.MkAndArray(conjuncts)
		if err != nil {
			return false, err
		}
	}
	if err = m.SetRef(local, flat); err != nil {
		return false, err
	}
	var target ast.ASTHandle
	if len(binders) == 1 {
		target, err = m.UpdateQuantifierBody(q, flat)
	} else {
		target, err = c.merge(binders, flat, weight)
	}
	if err != nil {
		return false, err
	}
	if err = m.SetRef(local, target); err != nil {
		return false, err
	}
	if c.der == nil {
		c.der, err = NewEqualityDER(m, nil, c.services)
		if err != nil {
			return false, err
		}
	}
	if err = c.der.ApplyQuantifier(target, out, proof); err != nil {
		return false, err
	}
	next, err := m.RefGet(out)
	if err != nil || next == target {
		return false, err
	}
	c.changed = true
	return true, nil
}

// Keep source order when flattening. Reordering the remaining conjuncts can
// change generated triggers and consequently the cost of quantifier search.
func (c *definitionRewriter) definingConjuncts(body ast.ASTHandle, n uint32) ([]ast.ASTHandle, bool, error) {
	m := c.m
	var leaves []ast.ASTHandle
	found := false
	stack := []ast.ASTHandle{body}
	for len(stack) > 0 {
		j := len(stack) - 1
		h := stack[j]
		stack = stack[:j]
		hd, err := m.NodeHeader(h)
		if err != nil {
			return nil, false, err
		}
		if hd.Kind == ast.ASTApp && hd.Family == m.BasicFamilyID() {
			switch hd.DeclKind {
			case ast.ASTOpAnd:
				if !c.spend(hd.NumArgs) {
					return nil, false, nil
				}
				for i := hd.NumArgs; i > 0; i-- {
					arg, err := m.GetArg(h, i-1)
					if err != nil {
						return nil, false, err
					}
					stack = append(stack, arg)
				}
				continue
			case ast.ASTOpEq:
				if !found && hd.NumArgs == 2 {
					for i := uint32(0); i < 2; i++ {
						arg, err := m.GetArg(h, i)
						if err != nil {
							return nil, false, err
						}
						kind, err := m.GetKind(arg)
						if err != nil {
							return nil, false, err
						}
						if kind == ast.ASTVar {
							index, err := m.GetVarIndex(arg)
							if err != nil {
								return nil, false, err
							}
							if index < n {
								found = true
								break
							}
						}
					}
				}
			}
		}
		leaves = append(leaves, h)
	}
	return leaves, found, nil
}

func (c *definitionRewriter) merge(binders []ast.ASTHandle, body ast.ASTHandle, weight int32) (ast.ASTHandle, error) {
	m := c.m
	qid, skid, err := m.GetQuantifierIDs(binders[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	spec := ast.ASTQuantifierSpec{Body: body, Weight: weight, QID: qid, SkolemID: skid}
	// Consecutive existential binders have the same de Bruijn indices when their
	// declarations are concatenated from the outermost binder to the innermost.
	for _, b := range binders {
		n, err := m.GetNumDeclarations(b)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		for i := uint32(0); i < n; i++ {
			sort, err := m.GetDeclarationSort(b, i)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			name, err := m.GetDeclarationName(b, i)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			spec.Sorts = append(spec.Sorts, sort)
			spec.Names = append(spec.Names, name)
		}
	}
	return m.MkQuantifier(spec)
}
