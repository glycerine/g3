package qe

// Source: src/muz/spacer/spacer_mev_array.cpp model_evaluator_array_util.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

// legacyArrayRefs owns AST references like a source expr_ref_vector.
type legacyArrayRefs struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (r *legacyArrayRefs) pin(h ast.ASTHandle) error {
	if h.IsNull() {
		return nil
	}
	ref, err := r.manager.NewRef(h)
	if err != nil {
		return err
	}
	r.refs = append(r.refs, ref)
	return nil
}

// adopt takes ownership of an already counted reference.
func (r *legacyArrayRefs) adopt(ref ast.ASTRef) {
	r.refs = append(r.refs, ref)
}

// close releases the references last to first. A pending manager refuses the
// release before any reference is dropped.
func (r *legacyArrayRefs) close() error {
	if len(r.refs) == 0 {
		return nil
	}
	if _, err := r.manager.ProofMode(); err != nil {
		return err
	}
	for len(r.refs) != 0 {
		index := len(r.refs) - 1
		if err := r.manager.DelRef(&r.refs[index]); err != nil {
			return err
		}
		r.refs = r.refs[:index]
	}
	return nil
}

// legacyArrayRefsCleanupFailure keeps references whose release was refused
// reachable for a later cleanup.
type legacyArrayRefsCleanupFailure struct {
	owner          *legacyArrayRefs
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyArrayRefsCleanupFailure)(nil)

func (f *legacyArrayRefsCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyArrayRefsCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyArrayRefsCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyArrayRefsCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArrayRefsCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArrayRefsCleanupFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.owner != nil {
		f.cleanup = f.owner.close()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.owner = nil
	}
	f.cleanup = nil
	return nil
}

// closeLegacyArrayRefs ends a source lexical scope holding refs.
func closeLegacyArrayRefs(refs *legacyArrayRefs, cause error) error {
	cleanup := refs.close()
	if cleanup == nil {
		return cause
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	reported := cleanup
	if cause != nil {
		reported = errors.Join(cause, cleanup)
	}
	var nested memory.ConstructionFailure
	_ = errors.As(cause, &nested)
	return &legacyArrayRefsCleanupFailure{
		owner: refs, nested: nested, runtime: refs.manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup, reported: reported,
	}
}

func (p *legacyArrayProjection) traceMev(function string, line int, body func(io.Writer) error) error {
	tags := p.syntax.config.Trace
	if tags == nil {
		tags = p.syntax.manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("model_evaluator", function, "src/muz/spacer/spacer_mev_array.cpp", line,
		func() bool { return true }, func(out io.Writer) error { return body(traceBodyWriter{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// traceMevTerms prints prefix, then each term followed by a newline.
func (p *legacyArrayProjection) traceMevTerms(function string, line int, prefix string, terms ...ast.ASTHandle) error {
	return p.traceMev(function, line, func(out io.Writer) error {
		_, _ = io.WriteString(out, prefix)
		for _, term := range terms {
			if err := writeExpression(p.syntax.manager, p.syntax.config, out, term); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\n")
		}
		return nil
	})
}

func (p *legacyArrayProjection) arrayOp(h ast.ASTHandle, op int32) ([]ast.ASTHandle, bool, error) {
	return p.syntax.operation(h, p.syntax.family, op)
}

// mevEval is model_evaluator_array_util::eval with model_completion=true.
// The result is owned by out.
func (p *legacyArrayProjection) mevEval(md *model.ModelCore, e ast.ASTHandle, out *legacyArrayRefs) (result ast.ASTHandle, resultErr error) {
	m := p.syntax.manager
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.syntax.config.Context, Globals: p.syntax.config.Globals,
		Memory: m.NumeralService().RuntimeOwner(), Cooperation: p.syntax.config.Cooperation, Trace: p.syntax.config.Trace,
	})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	err = evaluator.SetModelCompletion(true)
	var value ast.ASTRef
	evaluated := false
	if err == nil {
		value, err = evaluator.EvaluateValue(e)
		if err == nil {
			evaluated = true
		} else if rewriter.IsRewriterException(err) {
			err = nil
		}
	}
	err = closeFailedLegacyArrayEvaluator(m, evaluator, err)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !evaluated {
		return ast.ASTHandle{}, diagnostic.BorrowProcessDebugContext().VerifyFailure("src/muz/spacer/spacer_mev_array.cpp", 195, "eval_result")
	}
	out.adopt(value)
	result, err = m.RefGet(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	array, err := p.arrayExpression(e)
	if err != nil || !array {
		return result, err
	}
	local := &legacyArrayRefs{manager: m}
	defer func() { resultErr = closeLegacyArrayRefs(local, resultErr) }()
	stores, elseCase, ok, err := p.mevExtractArrayFuncInterp(md, result, nil, local)
	if err != nil || !ok {
		return result, err
	}
	sort, err := m.GetSort(e)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := m.MkConstArray(sort, elseCase)
	if err == nil {
		err = out.pin(r)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for len(stores) != 0 {
		row := stores[len(stores)-1]
		if row[len(row)-1] != elseCase {
			break
		}
		stores = stores[:len(stores)-1]
	}
	for i := len(stores); i > 0; {
		i--
		args := append([]ast.ASTHandle{r}, stores[i]...)
		r, err = m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, args)
		if err == nil {
			err = out.pin(r)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return r, nil
}

// mevEvalExprs is model_evaluator_array_util::eval_exprs.
func (p *legacyArrayProjection) mevEvalExprs(md *model.ModelCore, es []ast.ASTHandle, out *legacyArrayRefs) error {
	for j := range es {
		_, asArray, err := p.arrayOp(es[j], ast.ASTArrayOpAsArray)
		if err != nil {
			return err
		}
		if asArray {
			r, err := p.mevEval(md, es[j], out)
			if err != nil {
				return err
			}
			es[j] = r
		}
	}
	return nil
}

// mevExtractArrayFuncInterp is
// model_evaluator_array_util::extract_array_func_interp; stores grows by the
// rows found, owned by out.
func (p *legacyArrayProjection) mevExtractArrayFuncInterp(md *model.ModelCore, a ast.ASTHandle, stores [][]ast.ASTHandle, out *legacyArrayRefs) (_ [][]ast.ASTHandle, elseCase ast.ASTHandle, ok bool, err error) {
	m := p.syntax.manager
	const function = "extract_array_func_interp"
	if err = p.traceMevTerms(function, 50, "", a); err != nil {
		return stores, ast.ASTHandle{}, false, err
	}
	for {
		args, store, err := p.arrayOp(a, ast.ASTArrayOpStore)
		if err != nil {
			return stores, ast.ASTHandle{}, false, err
		}
		if !store {
			break
		}
		row := append([]ast.ASTHandle(nil), args[1:]...)
		if err = p.mevEvalExprs(md, row, out); err != nil {
			return stores, ast.ASTHandle{}, false, err
		}
		stores = append(stores, row)
		a = args[0]
	}
	args, constant, err := p.arrayOp(a, ast.ASTArrayOpConst)
	if err != nil {
		return stores, ast.ASTHandle{}, false, err
	}
	if constant {
		return stores, args[0], true, nil
	}
	_, asArray, err := p.arrayOp(a, ast.ASTArrayOpAsArray)
	if err != nil {
		return stores, ast.ASTHandle{}, false, err
	}
	if asArray {
		f, err := m.AsArrayFunction(a)
		if err != nil {
			return stores, ast.ASTHandle{}, false, err
		}
		g, err := md.FunctionInterpretation(f)
		if err != nil {
			return stores, ast.ASTHandle{}, false, err
		}
		if g == nil {
			return stores, ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		arity := g.Arity()
		for i := uint32(0); i < g.NumEntries(); i++ {
			fe, err := g.Entry(i)
			if err != nil {
				return stores, ast.ASTHandle{}, false, err
			}
			row := make([]ast.ASTHandle, 0, arity+1)
			for k := uint32(0); k < arity; k++ {
				arg, err := fe.Argument(k)
				if err != nil {
					return stores, ast.ASTHandle{}, false, err
				}
				row = append(row, arg)
			}
			value, err := fe.Result()
			if err != nil {
				return stores, ast.ASTHandle{}, false, err
			}
			row = append(row, value)
			for _, term := range row {
				ground, err := m.IsGround(term)
				if err != nil {
					return stores, ast.ASTHandle{}, false, err
				}
				if !ground {
					err = p.traceMev(function, 76, func(w io.Writer) error {
						_, _ = io.WriteString(w, "could not extract array interpretation: ")
						if err := writeExpression(m, p.syntax.config, w, a); err != nil {
							return err
						}
						_, _ = io.WriteString(w, "\n")
						if err := writeExpression(m, p.syntax.config, w, term); err != nil {
							return err
						}
						_, _ = io.WriteString(w, "\n")
						return nil
					})
					return stores, ast.ASTHandle{}, false, err
				}
			}
			// The row copies the entry's terms: pin them like the source
			// expr_ref_vector store.
			for _, term := range row {
				if err = out.pin(term); err != nil {
					return stores, ast.ASTHandle{}, false, err
				}
			}
			if err = p.mevEvalExprs(md, row, out); err != nil {
				return stores, ast.ASTHandle{}, false, err
			}
			stores = append(stores, row)
		}
		elseCase, err = g.Else()
		if err != nil {
			return stores, ast.ASTHandle{}, false, err
		}
		if elseCase.IsNull() {
			return stores, ast.ASTHandle{}, false, p.traceMevTerms(function, 85, "no else case ", a)
		}
		if err = out.pin(elseCase); err != nil {
			return stores, ast.ASTHandle{}, false, err
		}
		ground, err := m.IsGround(elseCase)
		if err != nil {
			return stores, ast.ASTHandle{}, false, err
		}
		if !ground {
			err = p.traceMev(function, 89, func(w io.Writer) error {
				_, _ = io.WriteString(w, "non-ground else case ")
				if err := writeExpression(m, p.syntax.config, w, a); err != nil {
					return err
				}
				_, _ = io.WriteString(w, "\n")
				if err := writeExpression(m, p.syntax.config, w, elseCase); err != nil {
					return err
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			})
			return stores, ast.ASTHandle{}, false, err
		}
		_, asArray, err = p.arrayOp(elseCase, ast.ASTArrayOpAsArray)
		if err != nil {
			return stores, ast.ASTHandle{}, false, err
		}
		if asArray {
			elseCase, err = p.mevEval(md, elseCase, out)
			if err != nil {
				return stores, ast.ASTHandle{}, false, err
			}
		}
		if err = p.traceMevTerms(function, 97, "else case: ", elseCase); err != nil {
			return stores, ast.ASTHandle{}, false, err
		}
		return stores, elseCase, true, nil
	}
	return stores, ast.ASTHandle{}, false, p.traceMevTerms(function, 100, "no translation: ", a)
}

// mevEvalArrayEq is model_evaluator_array_util::eval_array_eq. A null result
// means the equality is unknown.
func (p *legacyArrayProjection) mevEvalArrayEq(md *model.ModelCore, e, arg1, arg2 ast.ASTHandle) (res ast.ASTHandle, resultErr error) {
	m := p.syntax.manager
	const function = "eval_array_eq"
	if err := p.traceMevTerms(function, 106, "array equality: ", e); err != nil {
		return ast.ASTHandle{}, err
	}
	local := &legacyArrayRefs{manager: m}
	defer func() { resultErr = closeLegacyArrayRefs(local, resultErr) }()
	v1, err := p.mevEval(md, arg1, local)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	v2, err := p.mevEval(md, arg2, local)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if v1 == v2 {
		return m.MkTrue()
	}
	s, err := m.GetSort(arg1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := m.ArrayRange(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var flags [4]bool
	for n, query := range []func() (bool, error){
		func() (bool, error) { return m.SortIsInfinite(r) },
		func() (bool, error) { return m.SortIsVeryBig(r) },
		func() (bool, error) { return m.SortIsInfinite(s) },
		func() (bool, error) { return m.SortIsVeryBig(s) },
	} {
		if flags[n], err = query(); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	// give up evaluating finite domain/range arrays
	if !flags[0] && !flags[1] && !flags[2] && !flags[3] {
		return ast.ASTHandle{}, p.traceMevTerms(function, 118, "equality is unknown: ", e)
	}
	var store [][]ast.ASTHandle
	var else1, else2 ast.ASTHandle
	var ok bool
	store, else1, ok, err = p.mevExtractArrayFuncInterp(md, v1, store, local)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if ok {
		store, else2, ok, err = p.mevExtractArrayFuncInterp(md, v2, store, local)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if !ok {
		return ast.ASTHandle{}, p.traceMevTerms(function, 126, "equality is unknown: ", e)
	}
	if else1 != else2 {
		value1, err := m.IsValue(else1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		value2, err := m.IsValue(else2)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		nested, err := p.arrayExpression(else1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if value1 && value2 {
			if err = p.traceMev(function, 133, func(w io.Writer) error {
				_, _ = io.WriteString(w, "defaults are different: ")
				for n, term := range []ast.ASTHandle{e, else1, else2} {
					if n != 0 {
						_, _ = io.WriteString(w, " ")
					}
					if err := writeExpression(m, p.syntax.config, w, term); err != nil {
						return err
					}
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); err != nil {
				return ast.ASTHandle{}, err
			}
			return m.MkFalse()
		} else if nested {
			return p.mevEvalArrayEq(md, e, else1, else2)
		}
		return ast.ASTHandle{}, p.traceMevTerms(function, 142, "equality is unknown: ", e)
	}
	for i := range store {
		row := store[i]
		args1 := append([]ast.ASTHandle{v1}, row[:len(row)-1]...)
		args2 := append([]ast.ASTHandle{v2}, row[:len(row)-1]...)
		s1, err := m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, args1)
		if err == nil {
			err = local.pin(s1)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		s2, err := m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, args2)
		if err == nil {
			err = local.pin(s2)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		w1, err := p.mevEval(md, s1, local)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		w2, err := p.mevEval(md, s2, local)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if w1 == w2 {
			continue
		}
		value1, err := m.IsValue(w1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		value2, err := m.IsValue(w2)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if value1 && value2 {
			if err = p.traceMev(function, 165, func(w io.Writer) error {
				_, _ = io.WriteString(w, "Equality evaluation: ")
				if err := writeExpression(m, p.syntax.config, w, e); err != nil {
					return err
				}
				_, _ = io.WriteString(w, "\n")
				for _, pair := range [][2]ast.ASTHandle{{s1, w1}, {s2, w2}} {
					if err := writeExpression(m, p.syntax.config, w, pair[0]); err != nil {
						return err
					}
					_, _ = io.WriteString(w, " |-> ")
					if err := writeExpression(m, p.syntax.config, w, pair[1]); err != nil {
						return err
					}
					_, _ = io.WriteString(w, "\n")
				}
				return nil
			}); err != nil {
				return ast.ASTHandle{}, err
			}
			return m.MkFalse()
		}
		nested, err := p.arrayExpression(w1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if nested {
			res, err = p.mevEvalArrayEq(md, e, w1, w2)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if m.IsTrue(res) {
				continue
			}
			return res, nil
		}
		return ast.ASTHandle{}, p.traceMevTerms(function, 177, "equality is unknown: ", e)
	}
	return m.MkTrue()
}
