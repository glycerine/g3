package smtcore

// Source: src/smt/smt_implied_equalities.cpp (get_implied_equalities_impl,
// smt::implied_equalities), src/smt/smt_value_sort.cpp (is_value_sort) and
// the union_find<union_find_default_ctx> of src/util/union_find.h that
// assigns the class identifiers.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// ErrImpliedEqualitiesNoModel reports that the solver produced no model
// where get_implied_equalities_model_based evaluates terms in one. The C++
// only SASSERTs model.get() and then dereferences the null model_ref in a
// release build (undefined behavior); g3 stops with this error instead.
var ErrImpliedEqualitiesNoModel = errors.New("g3: get_implied_equalities: the solver produced no model")

// smtImpliedEqualityUnionFind is union_find<union_find_default_ctx> as
// get_implied_equalities_impl uses it: mk_var, find (no path compression)
// and merge, whose root on a size tie is the root of the second argument.
// The next-pointers and the trail stack are not observable here.
type smtImpliedEqualityUnionFind struct {
	find_ []uint32
	size  []uint32
}

func (u *smtImpliedEqualityUnionFind) mkVar() uint32 {
	r := uint32(len(u.find_))
	u.find_ = append(u.find_, r)
	u.size = append(u.size, 1)
	return r
}

func (u *smtImpliedEqualityUnionFind) find(v uint32) uint32 {
	for {
		next := u.find_[v]
		if next == v {
			return v
		}
		v = next
	}
}

func (u *smtImpliedEqualityUnionFind) merge(v1, v2 uint32) {
	r1 := u.find(v1)
	r2 := u.find(v2)
	if r1 == r2 {
		return
	}
	if u.size[r1] > u.size[r2] {
		r1, r2 = r2, r1
	}
	u.find_[r1] = r2
	u.size[r2] += u.size[r1]
}

// smtImpliedEqualityTerm is get_implied_equalities_impl::term_id.
type smtImpliedEqualityTerm struct {
	term ast.ASTHandle
	id   uint32
}

// smtImpliedEqualities is get_implied_equalities_impl.
type smtImpliedEqualities struct {
	m        *ast.ASTManager
	solver   solver.Solver
	services model.ModelEvaluatorConfig
	uf       smtImpliedEqualityUnionFind
	// refs are the expr_refs of the terms this procedure creates (the
	// selects of the array branch and its fresh index constants), released
	// when the procedure returns.
	refs       []ast.ASTRef
	statsCalls uint32
	timers     smtImpliedEqualityTimers
}

// ImpliedEqualities is smt::implied_equalities: for each term, the
// union-find root of its class of terms whose equality the solver's
// assertions imply. On an unsatisfiable first check the status is
// lbool.False and no identifiers are returned (the C++ leaves class_ids
// untouched); on unknown the identifiers are still computed. The solver's
// assertion stack is restored, and the produce_models parameter is left
// set to true.
// services are the model-evaluator services that the C++ model::eval takes
// from the process.
func ImpliedEqualities(s solver.Solver, services model.ModelEvaluatorConfig, terms []ast.ASTHandle) (status lbool.LBool, classIDs []uint32, result error) {
	if s == nil || services.Context == nil {
		return lbool.Undef, nil, ast.ErrASTHandle
	}
	manager, err := s.Manager()
	if err != nil {
		return lbool.Undef, nil, err
	}
	termRefs := make([]ast.ASTRef, 0, len(terms))
	defer func() {
		for index := range termRefs {
			result = errors.Join(result, manager.DelRef(&termRefs[index]))
		}
	}()
	for index, term := range terms {
		ref, err := manager.NewRef(term)
		if err != nil {
			return lbool.Undef, nil, fmt.Errorf("implied equality term %d reference: %w", index, err)
		}
		termRefs = append(termRefs, ref)
	}
	gi := &smtImpliedEqualities{m: manager, solver: s, services: services}
	defer func() {
		for index := range gi.refs {
			result = errors.Join(result, manager.DelRef(&gi.refs[index]))
		}
	}()
	classIDs = make([]uint32, len(terms))
	status, err = gi.run(terms, classIDs)
	if err != nil {
		return lbool.Undef, nil, err
	}
	if status == lbool.False {
		return status, nil, nil
	}
	return status, classIDs, nil
}

// ImpliedEqualities runs smt::implied_equalities on this solver with its
// own model-evaluator services.
func (s *SMTSolver) ImpliedEqualities(terms []ast.ASTHandle) (lbool.LBool, []uint32, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	services := s.config.ModelEvaluator
	if services.Context == nil {
		// The kernel's own evaluator services, as SetProduceModels uses.
		services = s.kernel.modelConfig
	}
	if services.Context == nil {
		services.Context, services.Globals = s.config.Params, s.config.Globals
	}
	services.Params = nil
	return ImpliedEqualities(s, services, terms)
}

// updtProduceModels is params_ref p; p.set_bool("produce_models", value);
// m_solver.updt_params(p).
func (gi *smtImpliedEqualities) updtProduceModels(value bool) (result error) {
	p, err := gi.services.Context.NewValue()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, gi.services.Context.Del(&p)) }()
	key := "produce_models"
	if err = gi.services.Context.SetBoolName(&p, &key, value); err != nil {
		return err
	}
	return gi.solver.UpdateParams(&p)
}

// run is get_implied_equalities_impl::operator().
func (gi *smtImpliedEqualities) run(terms []ast.ASTHandle, classIDs []uint32) (isSat lbool.LBool, result error) {
	if err := gi.updtProduceModels(true); err != nil {
		return lbool.Undef, err
	}
	timer := gi.timers.startCall()
	for range terms {
		gi.uf.mkVar()
	}
	if err := gi.solver.Push(); err != nil {
		return lbool.Undef, err
	}
	popped := false
	defer func() {
		if !popped {
			result = errors.Join(result, gi.solver.Pop(1))
		}
	}()
	if err := gi.assertRelevant(terms); err != nil {
		return lbool.Undef, err
	}
	isSat, err := gi.solver.CheckSat(nil)
	if err != nil {
		return lbool.Undef, err
	}
	if isSat != lbool.False {
		if err = gi.classify(terms, classIDs); err != nil {
			return lbool.Undef, err
		}
	}
	popped = true
	if err = gi.solver.Pop(1); err != nil {
		return lbool.Undef, err
	}
	gi.timers.stopCall(timer)
	if err = gi.timers.verbose(uint32(len(terms)), timer, gi.statsCalls); err != nil {
		return lbool.Undef, err
	}
	return isSat, nil
}

// classify is the l_true/l_undef block of operator(): the model of the first
// check, the sort partition and the per-sort procedure, then the class ids.
func (gi *smtImpliedEqualities) classify(terms []ast.ASTHandle, classIDs []uint32) (result error) {
	md, err := gi.solver.AcquireModel()
	if err != nil {
		return err
	}
	if md != nil {
		defer func() { result = errors.Join(result, md.DecRef()) }()
	}
	sortOrder, termids, err := gi.partitionTerms(terms)
	if err != nil {
		return err
	}
	for _, slot := range sortOrder.Entries() {
		termIDs := termids[slot.Key]
		if err = gi.modelBased(md, termIDs); err != nil {
			return err
		}
		for _, t := range termIDs {
			classIDs[t.id] = gi.uf.find(t.id)
		}
	}
	if smtTraceEnabled("get_implied_equalities") {
		var b strings.Builder
		for i, t := range terms {
			b.WriteString(tracePP(gi.m, t))
			fmt.Fprintf(&b, " |-> %d\n", classIDs[i])
		}
		emitSMTTrace("get_implied_equalities", "operator()", "src/smt/smt_implied_equalities.cpp", 354, b.String())
	}
	return nil
}

// partitionTerms is partition_terms: sort2term_ids is an obj_map<sort,
// term_ids>, walked in its table order.
func (gi *smtImpliedEqualities) partitionTerms(terms []ast.ASTHandle) (*ast.ObjMap, map[ast.ASTHandle][]smtImpliedEqualityTerm, error) {
	order := ast.NewObjMap(gi.m)
	termids := make(map[ast.ASTHandle][]smtImpliedEqualityTerm)
	for i, t := range terms {
		s, err := gi.m.GetSort(t)
		if err != nil {
			return nil, nil, err
		}
		if _, err = order.InsertIfNotThere(s); err != nil {
			return nil, nil, err
		}
		termids[s] = append(termids[s], smtImpliedEqualityTerm{term: t, id: uint32(i)})
	}
	return order, termids, nil
}

// isArraySort is array_util::is_array (is_sort_of(s, array family,
// ARRAY_SORT)).
func (gi *smtImpliedEqualities) isArraySort(s ast.ASTHandle) (bool, error) {
	family, err := gi.m.GetFamilyID(s)
	if err != nil {
		return false, err
	}
	arrayFamily, err := gi.m.ArrayFamilyID()
	if err != nil {
		return false, err
	}
	if family != arrayFamily {
		return false, nil
	}
	kind, err := gi.m.GetDeclKind(s)
	return kind == 0, err
}

// relevant is m.mk_app(m.mk_func_decl(symbol("Relevant!"), 1, &srt,
// m.mk_bool_sort()), t).
func (gi *smtImpliedEqualities) relevant(srt, t ast.ASTHandle) (ast.ASTHandle, error) {
	name, err := gi.m.Symbols.Intern("Relevant!")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := gi.m.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	decl, err := gi.m.MkFuncDecl(name, []ast.ASTHandle{srt}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return gi.m.MkApp(decl, []ast.ASTHandle{t})
}

// assertRelevant is both assert_relevant overloads: Relevant!(t) for every
// term that is not an array.
func (gi *smtImpliedEqualities) assertRelevant(terms []ast.ASTHandle) error {
	for _, t := range terms {
		srt, err := gi.m.GetSort(t)
		if err != nil {
			return err
		}
		isArray, err := gi.isArraySort(srt)
		if err != nil {
			return err
		}
		if isArray {
			continue
		}
		app, err := gi.relevant(srt, t)
		if err != nil {
			return err
		}
		if err = gi.solver.Assert(app); err != nil {
			return err
		}
	}
	return nil
}

func smtImpliedEqualityTermHandles(terms []smtImpliedEqualityTerm) []ast.ASTHandle {
	result := make([]ast.ASTHandle, len(terms))
	for i := range terms {
		result[i] = terms[i].term
	}
	return result
}

// impliedByNegation is m_solver.push(); m_solver.assert_expr(m.mk_not(
// m.mk_eq(lhs, rhs))); is_sat = m_solver.check_sat(0,nullptr);
// m_solver.pop(1).
func (gi *smtImpliedEqualities) impliedByNegation(lhs, rhs ast.ASTHandle) (lbool.LBool, error) {
	if err := gi.solver.Push(); err != nil {
		return lbool.Undef, err
	}
	equality, err := gi.m.MkEq(lhs, rhs)
	var negated ast.ASTHandle
	if err == nil {
		negated, err = gi.m.MkNot(equality)
	}
	if err == nil {
		err = gi.solver.Assert(negated)
	}
	status := lbool.Undef
	if err == nil {
		status, err = gi.solver.CheckSat(nil)
	}
	if popErr := gi.solver.Pop(1); popErr != nil || err != nil {
		return lbool.Undef, errors.Join(err, popErr)
	}
	return status, nil
}

// smtUintSet is the uint_set of get_implied_equalities_impl: membership and
// iteration in increasing order.
type smtUintSet map[uint32]struct{}

func (s smtUintSet) contains(v uint32) bool { _, ok := s[v]; return ok }

func smtUintSetMembers(s smtUintSet, limit int) []uint32 {
	result := make([]uint32, 0, len(s))
	for v := uint32(0); int(v) < limit; v++ {
		if s.contains(v) {
			result = append(result, v)
		}
	}
	return result
}

// filterBasic is get_implied_equalities_filter_basic.
func (gi *smtImpliedEqualities) filterBasic(nonValues smtUintSet, terms []smtImpliedEqualityTerm) error {
	gi.timers.stats.start()
	defer gi.timers.stats.stop()
	rootIndices := make(smtUintSet)
	for j := range terms {
		if terms[j].id == gi.uf.find(terms[j].id) {
			rootIndices[uint32(j)] = struct{}{}
		}
	}
	roots := smtUintSetMembers(rootIndices, len(terms))
	for _, i := range smtUintSetMembers(nonValues, len(terms)) {
		t := terms[i].term
		foundRootValue := false
		for _, j := range roots {
			if j == i {
				continue
			}
			if j < i && nonValues.contains(j) {
				continue
			}
			if foundRootValue && !nonValues.contains(j) {
				continue
			}
			s := terms[j].term
			gi.statsCalls++
			status, err := gi.impliedByNegation(s, t)
			if err != nil {
				return err
			}
			isEq := status == lbool.False
			if smtTraceEnabled("get_implied_equalities") {
				word := "unrelated"
				if isEq {
					word = "eq"
				}
				emitSMTTrace("get_implied_equalities", "get_implied_equalities_filter_basic", "src/smt/smt_implied_equalities.cpp", 101, tracePP(gi.m, t)+" = "+tracePP(gi.m, s)+" "+word+"\n")
			}
			if isEq {
				gi.uf.merge(terms[i].id, terms[j].id)
				if !nonValues.contains(j) {
					foundRootValue = true
				}
			}
		}
	}
	return nil
}

// modelBased is get_implied_equalities_model_based (reached through
// get_implied_equalities_core).
func (gi *smtImpliedEqualities) modelBased(md *model.ModelCore, terms []smtImpliedEqualityTerm) error {
	srt, err := gi.m.GetSort(terms[0].term)
	if err != nil {
		return err
	}
	isArray, err := gi.isArraySort(srt)
	if err != nil {
		return err
	}
	if isArray {
		return gi.arrayBranch(srt, terms)
	}
	nonValues := make(smtUintSet)
	valueSort, err := smtIsValueSort(gi.m, srt)
	if err != nil {
		return err
	}
	if !valueSort {
		for i := range terms {
			nonValues[uint32(i)] = struct{}{}
		}
		return gi.filterBasic(nonValues, terms)
	}
	if md == nil {
		return ErrImpliedEqualitiesNoModel
	}
	// vals keeps every value alive until the procedure returns; vl is the
	// expr_ref that model::eval assigns, which keeps its previous value
	// when the evaluator throws.
	var vals []ast.ASTRef
	vl, err := gi.m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() {
		_ = gi.m.DelRef(&vl)
		for i := range vals {
			_ = gi.m.DelRef(&vals[i])
		}
	}()
	valsMap := make(map[ast.ASTHandle][]uint32)
	gi.timers.valEq.start()
	gi.timers.startStaticValEq()
	if err = gi.updtProduceModels(false); err != nil {
		return err
	}
	for i := range terms {
		t := terms[i].term
		if err = gi.modelEval(md, t, vl); err != nil {
			return err
		}
		value, err := gi.m.RefGet(vl)
		if err != nil {
			return err
		}
		if smtTraceEnabled("get_implied_equalities") {
			emitSMTTrace("get_implied_equalities", "get_implied_equalities_model_based", "src/smt/smt_implied_equalities.cpp", 203, tracePP(gi.m, t)+" |-> "+tracePP(gi.m, value)+"\n")
		}
		if value, err = gi.reduceValue(md, value); err != nil {
			return err
		}
		if err = gi.m.SetRef(vl, value); err != nil {
			return err
		}
		// g3 divergence: a first evaluation that throws leaves the C++ vl
		// null, and m.is_ite(vl) then dereferences it (undefined
		// behavior). g3 treats the null term as a non-value.
		isValue := false
		if !value.IsNull() {
			if isValue, err = gi.m.IsValue(value); err != nil {
				return err
			}
		}
		if !isValue {
			if smtTraceEnabled("get_implied_equalities") {
				emitSMTTrace("get_implied_equalities", "get_implied_equalities_model_based", "src/smt/smt_implied_equalities.cpp", 206, "Not a value: "+tracePP(gi.m, value)+"\n")
			}
			nonValues[uint32(i)] = struct{}{}
			continue
		}
		kept, err := gi.m.NewRef(value)
		if err != nil {
			return err
		}
		vals = append(vals, kept)
		vec := valsMap[value]
		found := false
		for j := 0; !found && j < len(vec); j++ {
			s := terms[vec[j]].term
			isSat, err := gi.impliedByNegation(t, s)
			if err != nil {
				return err
			}
			if smtTraceEnabled("get_implied_equalities") {
				emitSMTTrace("get_implied_equalities", "get_implied_equalities_model_based", "src/smt/smt_implied_equalities.cpp", 220, tracePP(gi.m, t)+" = "+tracePP(gi.m, s)+" "+isSat.String()+"\n")
			}
			if isSat == lbool.False {
				found = true
				gi.uf.merge(terms[i].id, terms[vec[j]].id)
			}
		}
		if !found {
			valsMap[value] = append(vec, uint32(i))
		}
	}
	gi.timers.valEq.stop()
	gi.timers.stopStaticValEq()
	if err = gi.updtProduceModels(true); err != nil {
		return err
	}
	if len(nonValues) != 0 {
		if smtTraceEnabled("get_implied_equalities") {
			var b strings.Builder
			if err = md.WriteSMT2(&b, gi.services.Context, gi.services.Globals, 0); err != nil {
				return err
			}
			emitSMTTrace("get_implied_equalities", "get_implied_equalities_model_based", "src/smt/smt_implied_equalities.cpp", 237, b.String())
		}
		return gi.filterBasic(nonValues, terms)
	}
	return nil
}

// arrayBranch is the array case of get_implied_equalities_model_based:
// arrays are compared by extensionality on fresh index constants.
func (gi *smtImpliedEqualities) arrayBranch(srt ast.ASTHandle, terms []smtImpliedEqualityTerm) (result error) {
	if err := gi.solver.Push(); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, gi.solver.Pop(1)) }()
	arity, err := gi.m.ArrayArity(srt)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, 1, arity+1)
	for i := uint32(0); i < arity; i++ {
		srtI, err := gi.m.ArrayDomain(srt, i)
		if err != nil {
			return err
		}
		idx, err := gi.m.MkFreshConst("index", srtI)
		if err != nil {
			return err
		}
		if err = gi.keep(idx); err != nil {
			return err
		}
		args = append(args, idx)
	}
	arrayFamily, err := gi.m.ArrayFamilyID()
	if err != nil {
		return err
	}
	for i := range terms {
		args[0] = terms[i].term
		sel, err := gi.m.MkArrayAppArguments(arrayFamily, ast.ASTArrayOpSelect, ast.SliceArguments(args))
		if err != nil {
			return err
		}
		if err = gi.keep(sel); err != nil {
			return err
		}
		terms[i].term = sel
	}
	if err = gi.assertRelevant(smtImpliedEqualityTermHandles(terms)); err != nil {
		return err
	}
	status, err := gi.solver.CheckSat(nil)
	if err != nil {
		return err
	}
	if status == lbool.False {
		// VERIFY(m_solver.check_sat(0,nullptr) != l_false): the release
		// build reports the failure and continues.
		if err = diagnostic.BorrowProcessDebugContext().VerifyFailure("src/smt/smt_implied_equalities.cpp", 169, "m_solver.check_sat(0,nullptr) != l_false"); err != nil {
			return err
		}
	}
	model1, err := gi.solver.AcquireModel()
	if err != nil {
		return err
	}
	if model1 != nil {
		defer func() { result = errors.Join(result, model1.DecRef()) }()
	}
	return gi.modelBased(model1, terms)
}

func (gi *smtImpliedEqualities) keep(h ast.ASTHandle) error {
	ref, err := gi.m.NewRef(h)
	if err != nil {
		return err
	}
	gi.refs = append(gi.refs, ref)
	return nil
}

// modelEval is model->eval(t, vl): a fresh model_evaluator with default
// parameters and model completion off. A model_evaluator_exception is
// caught and leaves vl unchanged.
func (gi *smtImpliedEqualities) modelEval(md *model.ModelCore, t ast.ASTHandle, vl ast.ASTRef) (result error) {
	services := gi.services
	services.Params = nil
	evaluator, err := model.NewModelEvaluator(md, services)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return err
	}
	defer func() { result = errors.Join(result, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(false); err != nil {
		return err
	}
	out, err := gi.m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, gi.m.DelRef(&out)) }()
	if err = evaluator.Evaluate(t, out); err != nil {
		if !rewriter.IsRewriterException(err) {
			return err
		}
		if smtTraceEnabled("model_evaluator") {
			emitSMTTrace("model_evaluator", "eval", "src/model/model.cpp", 88, err.Error()+"\n")
		}
		return nil
	}
	value, err := gi.m.RefGet(out)
	if err != nil {
		return err
	}
	return gi.m.SetRef(vl, value)
}

// reduceValue is reduce_value: it follows ite branches whose condition
// reduce_cond decides.
func (gi *smtImpliedEqualities) reduceValue(md *model.ModelCore, vl ast.ASTHandle) (ast.ASTHandle, error) {
	for !vl.IsNull() {
		c, e1, e2, isIte, err := gi.basicApp3(vl, ast.ASTOpIte)
		if err != nil || !isIte {
			return vl, err
		}
		r, err := gi.reduceCond(md, c)
		if err != nil {
			return vl, err
		}
		switch r {
		case lbool.True:
			vl = e1
		case lbool.False:
			vl = e2
		default:
			return vl, nil
		}
	}
	return vl, nil
}

// basicApp3 matches an application of the basic family operator kind and
// returns its arguments (m.is_ite(e, c, t, e) / m.is_eq(e, lhs, rhs)).
func (gi *smtImpliedEqualities) basicApp3(e ast.ASTHandle, kind int32) (a0, a1, a2 ast.ASTHandle, matched bool, err error) {
	k, err := gi.m.GetKind(e)
	if err != nil || k != ast.ASTApp {
		return a0, a1, a2, false, err
	}
	family, err := gi.m.GetFamilyID(e)
	if err != nil || family != gi.m.BasicFamilyID() {
		return a0, a1, a2, false, err
	}
	op, err := gi.m.GetDeclKind(e)
	if err != nil || op != kind {
		return a0, a1, a2, false, err
	}
	n, err := gi.m.GetNumArgs(e)
	if err != nil {
		return a0, a1, a2, false, err
	}
	out := []*ast.ASTHandle{&a0, &a1, &a2}
	for i := uint32(0); i < n && i < 3; i++ {
		if *out[i], err = gi.m.GetArg(e, i); err != nil {
			return a0, a1, a2, false, err
		}
	}
	return a0, a1, a2, true, nil
}

// asArrayDecl is m_array_util.is_as_array(e) with
// get_as_array_func_decl(to_app(e)).
func (gi *smtImpliedEqualities) asArrayDecl(e ast.ASTHandle) (ast.ASTHandle, bool, error) {
	k, err := gi.m.GetKind(e)
	if err != nil || k != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	f, err := gi.m.AsArrayFunction(e)
	if errors.Is(err, ast.ErrASTDomain) {
		return ast.ASTHandle{}, false, nil
	}
	return f, err == nil, err
}

// reduceCond is reduce_cond.
func (gi *smtImpliedEqualities) reduceCond(md *model.ModelCore, e ast.ASTHandle) (lbool.LBool, error) {
	e1, e2, _, isEq, err := gi.basicApp3(e, ast.ASTOpEq)
	if err != nil || !isEq {
		return lbool.Undef, err
	}
	d1, isAsArray1, err := gi.asArrayDecl(e1)
	if err != nil || !isAsArray1 {
		return lbool.Undef, err
	}
	d2, isAsArray2, err := gi.asArrayDecl(e2)
	if err != nil || !isAsArray2 {
		return lbool.Undef, err
	}
	if e1 == e2 {
		return lbool.True, nil
	}
	fi1, err := md.FunctionInterpretation(d1)
	if err != nil {
		return lbool.Undef, err
	}
	fi2, err := md.FunctionInterpretation(d2)
	if err != nil {
		return lbool.Undef, err
	}
	if fi1 == fi2 {
		return lbool.True, nil
	}
	// g3 divergence: the C++ dereferences a missing interpretation
	// (undefined behavior); g3 leaves the condition undecided.
	if fi1 == nil || fi2 == nil {
		return lbool.Undef, nil
	}
	n1 := fi1.NumEntries()
	for i := uint32(0); i < n1; i++ {
		h1, err := fi1.Entry(i)
		if err != nil {
			return lbool.Undef, err
		}
		args := make([]ast.ASTHandle, fi1.Arity())
		for j := uint32(0); j < fi1.Arity(); j++ {
			if args[j], err = h1.Argument(j); err != nil {
				return lbool.Undef, err
			}
			isValue, err := gi.m.IsValue(args[j])
			if err != nil || !isValue {
				return lbool.Undef, err
			}
		}
		h2, err := fi2.FindEntry(args)
		if err != nil {
			return lbool.Undef, err
		}
		if h2 == nil {
			continue
		}
		r1, err := h1.Result()
		if err != nil {
			return lbool.Undef, err
		}
		r2, err := h2.Result()
		if err != nil {
			return lbool.Undef, err
		}
		if r1 == r2 {
			continue
		}
		v1, err := gi.m.IsValue(r1)
		if err != nil {
			return lbool.Undef, err
		}
		v2, err := gi.m.IsValue(r2)
		if err != nil {
			return lbool.Undef, err
		}
		if v1 && v2 {
			return lbool.False, nil
		}
	}
	return lbool.Undef, nil
}

// smtIsValueSort is smt::is_value_sort: arithmetic, Boolean and bit-vector
// sorts, and datatypes all of whose constructor argument sorts are value
// sorts.
func smtIsValueSort(m *ast.ASTManager, s ast.ASTHandle) (bool, error) {
	boolean, err := m.MkBoolSort()
	if err != nil {
		return false, err
	}
	arith, err := m.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	bv, err := m.BitVectorUtilityFamilyID()
	if err != nil {
		return false, err
	}
	sorts := []ast.ASTHandle{s}
	mark := make(map[ast.ASTHandle]bool)
	for len(sorts) != 0 {
		s = sorts[len(sorts)-1]
		sorts = sorts[:len(sorts)-1]
		if mark[s] {
			continue
		}
		mark[s] = true
		family, err := m.GetFamilyID(s)
		if err != nil {
			return false, err
		}
		kind, err := m.GetDeclKind(s)
		if err != nil {
			return false, err
		}
		if family == arith && (kind == 0 || kind == 1) {
			// simple (arith_util::is_int_real: REAL_SORT, INT_SORT)
			continue
		}
		if s == boolean {
			// simple
			continue
		}
		if family == bv && kind == 0 {
			// simple (bv_util::is_bv_sort: BV_SORT)
			continue
		}
		if datatype, e := m.DatatypeFamilyID(); e == nil && family == datatype && kind == ast.ASTDatatypeSort {
			cs, err := m.DatatypeConstructors(s)
			if err != nil {
				return false, err
			}
			for _, f := range cs {
				arity, err := m.GetArity(f)
				if err != nil {
					return false, err
				}
				for j := uint32(0); j < arity; j++ {
					d, err := m.GetDomain(f, j)
					if err != nil {
						return false, err
					}
					sorts = append(sorts, d)
				}
			}
			continue
		}
		return false, nil
	}
	return true, nil
}
