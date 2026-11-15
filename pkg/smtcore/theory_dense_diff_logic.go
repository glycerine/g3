package smtcore

// Source: src/smt/theory_dense_diff_logic.h and theory_dense_diff_logic_def.h.

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type smtDenseAtom struct {
	variable       int32
	source, target int
	offset         smtInfRational
	num            smtDenseNum // offset
}
type smtDenseScope struct {
	atoms       int
	unsupported bool
}

// SMTDenseDiffTheory is the concrete arbitrary-precision dense arithmetic
// specialization. Syntax helpers borrow its base; no sparse graph is created.
type SMTDenseDiffTheory struct {
	*SMTTheoryBase
	syntax      SMTIntegerDiffTheory
	matrix      *smtDenseDiffMatrix
	adapter     *smtArithmeticEqualityAdapter
	config      SMTArithState
	atoms       []smtDenseAtom
	byBool      map[int32]int
	integer     []bool
	scopes      []smtDenseScope
	unsupported bool
	values      []smtInfRational
	delta       big.Rat
	statistics  smtDenseDiffStatistics
}

func NewSMTDenseDiffTheory(family ast.FamilyID) (*SMTDenseDiffTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	adapter, err := newSMTArithmeticEqualityAdapter(base)
	if err != nil {
		return nil, err
	}
	t := &SMTDenseDiffTheory{SMTTheoryBase: base, matrix: newSMTDenseDiffMatrix(), adapter: adapter, byBool: make(map[int32]int)}
	t.syntax.SMTTheoryBase, t.syntax.equalityAdapter = base, adapter
	// theory_dense_diff_logic uses arith_eq_adapter::mk_axioms itself.
	adapter.source = true
	t.matrix.owner = t
	return t, nil
}

func (t *SMTDenseDiffTheory) ConfigureArithmetic(config SMTArithState) error {
	t.config = config
	t.adapter.lazy = config.LazyAdapter
	t.adapter.binary = config.AddBinaryBounds
	return nil
}

// Name is theory_dense_diff_logic::get_name (AUDIT.md FUZZ-F5).
func (*SMTDenseDiffTheory) Name() string { return "difference-logic" }

func (t *SMTDenseDiffTheory) createVariable(node *SMTEnode) (SMTTheoryVar, error) {
	if t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	v, err := t.SMTTheoryBase.CreateVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if int(v) != t.matrix.addVariable() {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	sort, err := t.state.manager.GetSort(node.owner)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	integer, err := t.syntax.integerSort(sort)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	t.integer = append(t.integer, integer)
	return v, nil
}

func (t *SMTDenseDiffTheory) term(expression ast.ASTHandle) (SMTTheoryVar, error) {
	m := t.state.manager
	if node := t.state.context.enodes.get(expression); node != nil && t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	value, numeral, err := t.syntax.numeral(expression)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	base, offset, shifted, err := t.syntax.offset(expression)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	var source SMTTheoryVar
	if shifted {
		if t.config.Reflect {
			for i := uint32(0); i < 2; i++ {
				argument, err := m.GetArg(expression, i)
				if err != nil {
					return SMTNullTheoryVar, err
				}
				if _, numeral, err := t.syntax.numeral(argument); err != nil {
					return SMTNullTheoryVar, err
				} else if numeral {
					if _, err := t.term(argument); err != nil {
						return SMTNullTheoryVar, err
					}
				}
			}
		}
		source, err = t.term(base)
		if err != nil || source == SMTNullTheoryVar {
			return source, err
		}
	} else if !numeral {
		_, arithmetic, err := t.syntax.operation(expression)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		if arithmetic {
			return SMTNullTheoryVar, nil
		}
		if err := t.state.Internalize(expression, false); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	node := t.state.context.enodes.get(expression)
	if node == nil {
		node, err = t.state.MkEnode(expression, shifted && !t.config.Reflect, false, true)
		if err != nil {
			return SMTNullTheoryVar, err
		}
	}
	v, err := t.createVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if numeral && value.Sign() != 0 {
		zero, err := m.ArithmeticNumeralRat(new(big.Rat), t.integer[v])
		if err != nil {
			return SMTNullTheoryVar, err
		}
		source, err = t.term(zero)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		offset, shifted = value, true
	}
	if shifted {
		weight := newSMTInfRational(offset, nil)
		if err = t.addEdge(int(source), int(v), &weight, SMTNullLiteral); err != nil {
			return SMTNullTheoryVar, err
		}
		weight.Neg(&weight)
		if err = t.addEdge(int(v), int(source), &weight, SMTNullLiteral); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	return v, nil
}

// foundNonDiffLogicExpr is theory_dense_diff_logic<Ext>::found_non_diff_logic_expr.
func (t *SMTDenseDiffTheory) foundNonDiffLogicExpr(n ast.ASTHandle) error {
	if t.unsupported {
		return nil
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	if smtTraceEnabled("non_diff_logic") {
		var body string
		tracePPTo(state.manager, n, func(text string) { body = text })
		emitSMTTrace("non_diff_logic", "found_non_diff_logic_expr", "src/smt/theory_dense_diff_logic_def.h", 128,
			"found non diff logic expression:\n"+body+"\n")
	}
	// IF_VERBOSE(0, verbose_stream() << "(smt.diff_logic: non-diff logic
	// expression " << mk_pp(n, get_manager()) << ")\n";)
	enabled, err := diagnostic.ProcessVerboseEnabled(0)
	if err != nil {
		return err
	}
	if enabled {
		stream, err := diagnostic.BorrowProcessVerboseStream()
		if err != nil {
			return err
		}
		if err = stream.AppendCString("(smt.diff_logic: non-diff logic expression "); err != nil {
			return err
		}
		var writeErr error
		tracePPTo(state.manager, n, func(text string) {
			if writeErr == nil {
				writeErr = stream.AppendCString(text)
			}
		})
		if writeErr != nil {
			return writeErr
		}
		if err = stream.AppendCString(")\n"); err != nil {
			return err
		}
		if err = stream.Flush(); err != nil {
			return err
		}
	}
	t.unsupported = true
	return nil
}

func (t *SMTDenseDiffTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	t.traceDDL("ddl", "internalize_term", 214, func() string { return "internalizing term: " + t.ddlPP(expression) + "\n" })
	v, err := t.term(expression)
	if err != nil {
		return false, err
	}
	t.traceDDL("ddl", "internalize_term", 216, func() string {
		result := "0"
		if v != SMTNullTheoryVar {
			result = "1"
		}
		return t.ddlPP(expression) + "\ninternalization result: " + result + "\n"
	})
	if v == SMTNullTheoryVar {
		if err = t.foundNonDiffLogicExpr(expression); err != nil {
			return false, err
		}
	}
	return v != SMTNullTheoryVar, nil
}

func (*SMTDenseDiffTheory) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error { return nil }

// traceDDL emits a TRACE record of src/smt/theory_dense_diff_logic_def.h;
// body runs only when the tag is enabled.
func (t *SMTDenseDiffTheory) traceDDL(tag, function string, line int, body func() string) {
	if smtTraceEnabled(tag) {
		emitSMTTrace(tag, function, "src/smt/theory_dense_diff_logic_def.h", line, body())
	}
}

// traceDeleteAtom emits the two del_atoms records of one atom around
// m_bv2atoms[a->get_bool_var()] = 0 (clear).
func (t *SMTDenseDiffTheory) traceDeleteAtom(a *smtDenseAtom, clear func()) {
	t.traceDDL("del_atoms", "del_atoms", 345, func() string { return fmt.Sprintf("deleting: p%d\n", a.variable) })
	clear()
	t.traceDDL("del_atoms", "del_atoms", 349, func() string {
		return fmt.Sprintf("m_matrix.size() %d, m_matrix[s].size() %d, m_matrix[t].size(): %d, t: %d, s: %d\n",
			len(t.matrix.rows), len(t.matrix.rows[a.source]), len(t.matrix.rows[a.target]), a.target, a.source)
	})
}

// ddlModelText is the "ddl model" body of the ddl_model records.
func (t *SMTDenseDiffTheory) ddlModelText() string {
	text := "ddl model\n"
	for v, node := range t.variables {
		text += "#" + t.ddlPP(node.owner) + " = " + smtDenseInfText(&t.values[v]) + "\n"
	}
	return text
}

// ddlPP is mk_pp(n, get_manager()) for a trace operand.
func (t *SMTDenseDiffTheory) ddlPP(n ast.ASTHandle) string {
	var text string
	tracePPTo(t.state.manager, n, func(s string) { text = s })
	return text
}

func (t *SMTDenseDiffTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	t.traceDDL("ddl", "internalize_atom", 141, func() string { return "internalizing atom:\n" + t.ddlPP(expression) + "\n" })
	op, own, err := t.syntax.operation(expression)
	if err != nil {
		return false, err
	}
	if !own || op != smtArithOpLE && op != smtArithOpGE {
		t.unsupported = true
		return false, nil
	}
	m := t.state.manager
	lhs, err := m.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	rhs, err := m.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	bound, numeral, err := t.syntax.numeral(rhs)
	if err != nil {
		return false, err
	}
	if !numeral {
		t.unsupported = true
		return false, nil
	}
	// theory_dense_diff_logic::internalize_atom matches four fixed shapes.
	var source, target ast.ASTHandle
	matched := false
	if arg1, arg2, ok, err := t.denseBinaryApp(lhs, smtArithOpAdd); err != nil {
		return false, err
	} else if ok {
		// (+ t (* -1 s)) then (+ (* -1 s) t)
		if r, ok, err := t.denseTimesMinusOne(arg2); err != nil {
			return false, err
		} else if ok {
			source, target, matched = r, arg1, true
		} else if r, ok, err := t.denseTimesMinusOne(arg1); err != nil {
			return false, err
		} else if ok {
			source, target, matched = r, arg2, true
		}
	}
	if !matched {
		if arg1, arg2, ok, err := t.denseBinaryApp(lhs, smtArithOpMul); err != nil {
			return false, err
		} else if ok {
			// (* -1 s): t = mk_zero_for(s)
			value, numeral, err := t.syntax.numeral(arg1)
			if err != nil {
				return false, err
			}
			if numeral && value.Cmp(big.NewRat(-1, 1)) == 0 {
				zero, err := t.denseZeroFor(arg2)
				if err != nil {
					return false, err
				}
				source, target, matched = arg2, zero, true
			}
		}
	}
	if !matched {
		_, arithmetic, err := t.syntax.operation(lhs)
		if err != nil {
			return false, err
		}
		if arithmetic {
			t.traceDDL("ddl", "internalize_atom", 170, func() string { return "failed to internalize:\n" + t.ddlPP(expression) + "\n" })
			return false, t.foundNonDiffLogicExpr(expression)
		}
		// !is_arith_expr(lhs): s = mk_zero_for(t)
		zero, err := t.denseZeroFor(lhs)
		if err != nil {
			return false, err
		}
		source, target = zero, lhs
	}
	t.traceDDL("arith", "internalize_atom", 174, func() string {
		return t.ddlPP(lhs) + " " + t.ddlPP(source) + " " + t.ddlPP(target) + "\n"
	})
	s, err := t.term(source)
	if err != nil {
		return false, err
	}
	u, err := t.term(target)
	if err != nil {
		return false, err
	}
	if s == SMTNullTheoryVar || u == SMTNullTheoryVar {
		t.traceDDL("ddl", "internalize_atom", 178, func() string { return "failed to internalize:\n" + t.ddlPP(expression) + "\n" })
		return false, t.foundNonDiffLogicExpr(expression)
	}
	weight := newSMTInfRational(bound, nil)
	if op == smtArithOpGE {
		s, u = u, s
		weight.Neg(&weight)
	}
	v, err := t.state.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err = t.state.SetVariableTheory(v, t.FamilyID()); err != nil {
		return false, err
	}
	a := smtDenseAtom{v, int(s), int(u), weight, smtDenseNumOf(&weight)}
	index := len(t.atoms)
	t.byBool[v] = index
	t.atoms = append(t.atoms, a)
	// m_matrix[source][target].m_occs.push_back(a), and the reverse cell.
	t.matrix.occs[s][u] = append(t.matrix.occs[s][u], index)
	t.matrix.occs[u][s] = append(t.matrix.occs[u][s], index)
	t.traceDDL("ddl", "internalize_atom", 194, func() string { return "succeeded internalizing:\n" + t.ddlPP(expression) + "\n" })
	return true, nil
}

func (t *SMTDenseDiffTheory) Assign(variable int32, truth bool) error {
	data, err := t.state.BooleanData(variable)
	if err != nil {
		return err
	}
	if justification, ok := data.Justification().Justification(); ok && justification != nil && justification.FromTheory() == t.FamilyID() {
		t.traceDDL("ddl", "assign_eh", 255, func() string { return "ignoring atom propagated by the theory.\n" })
		return nil
	}
	i, found := t.byBool[variable]
	if !found {
		return nil
	}
	a := &t.atoms[i]
	t.statistics.assertions++
	t.traceDDL("assign_profile", "assign_eh", 268, func() string {
		sourceID, _ := t.variables[a.source].OwnerID()
		targetID, _ := t.variables[a.target].OwnerID()
		return fmt.Sprintf("#%d #%d %s\n", sourceID, targetID, smtDenseInfText(&a.offset))
	})
	source, target := a.source, a.target
	var offset smtInfRational
	offset.Set(&a.offset)
	if !truth {
		source, target = target, source
		offset.Neg(&offset)
		if t.integer[a.source] {
			offset.rational.Sub(&offset.rational, big.NewRat(1, 1))
		} else {
			offset.infinitesimal.Sub(&offset.infinitesimal, big.NewRat(1, 1))
		}
	}
	if err = t.addEdge(source, target, &offset, NewSMTLiteral(variable, !truth)); err != nil {
		return err
	}
	t.traceDDL("ddl_detail", "assign_eh", 277, func() string {
		var out strings.Builder
		_ = t.Display(&out)
		return out.String()
	})
	return nil
}

func (t *SMTDenseDiffTheory) addEdge(source, target int, offset *smtInfRational, literal SMTLiteral) error {
	ok, reason, err := t.matrix.addEdge(source, target, offset, literal)
	if err != nil {
		return err
	}
	if !ok {
		if err = t.state.SetTheoryConflict(t.FamilyID(), reason); err != nil {
			return err
		}
		if !t.config.DumpLemmas {
			return nil
		}
		_, err = t.state.DumpLemmaAsSMTProblem(reason, nil, SMTFalseLiteral, usymbol.Symbol{})
		var dump *SMTLemmaDumpError
		if errors.As(err, &dump) {
			return dump.OperandError
		}
		return err
	}
	return nil
}

func (t *SMTDenseDiffTheory) propagateCell(source, target int) error {
	if !t.matrix.valid(source) || !t.matrix.valid(target) {
		return nil
	}
	cell := &t.matrix.rows[source][target]
	occurrences := t.matrix.occs[source][target]
	if cell.edge < 0 || len(occurrences) == 0 {
		return nil
	}
	distance := cell.distance
	for _, index := range occurrences {
		a := &t.atoms[index]
		literal := NewSMTLiteral(a.variable, false)
		assignment, err := t.state.Assignment(literal)
		if err != nil {
			return err
		}
		if assignment != lbool.Undef {
			continue
		}
		implied := smtDenseNumCmp(distance, a.num) <= 0
		if a.source != source {
			implied = smtDenseNumCmp(smtDenseNumNeg(distance), a.num) > 0
			literal = literal.Negated()
		}
		if !implied {
			continue
		}
		t.statistics.propagations++
		line := 566
		if a.source != source {
			line = 577
		}
		t.traceDDL("ddl", "propagate_using_cell", line, func() string {
			sourceID, _ := t.variables[source].OwnerID()
			targetID, _ := t.variables[target].OwnerID()
			return "asserting atom to true: " + t.displayAtomText(a) +
				fmt.Sprintf("distance(#%d, #%d): %s\n", sourceID, targetID, smtDenseNumText(distance))
		})
		reason, err := t.matrix.antecedents(source, target)
		if err != nil {
			return err
		}
		owner, err := NewSMTTheoryPropagationJustification(t.state.manager, t.FamilyID(), reason, literal, nil)
		if err != nil {
			return err
		}
		justification, err := NewSMTBooleanJustification(owner)
		if err != nil {
			return errors.Join(err, owner.Release(t.state.manager))
		}
		if err = t.state.AssignLiteral(literal, justification); err != nil {
			return errors.Join(err, owner.Release(t.state.manager))
		}
		t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	}
	return nil
}

func (t *SMTDenseDiffTheory) InitializeSearch() error { return t.adapter.InitializeSearch() }

func (t *SMTDenseDiffTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.adapter.PushScope(); err != nil {
		return err
	}
	t.matrix.push()
	t.scopes = append(t.scopes, smtDenseScope{len(t.atoms), t.unsupported})
	return nil
}
func (t *SMTDenseDiffTheory) PopScopes(count uint32) error {
	if count == 0 || uint64(count) > uint64(len(t.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	s := t.scopes[level]
	// del_atoms(s.m_atoms_lim), before del_vars shrinks the matrix.
	for i := len(t.atoms) - 1; i >= s.atoms; i-- {
		a := &t.atoms[i]
		t.traceDeleteAtom(a, func() { delete(t.byBool, a.variable) })
		for _, key := range [2][2]int{{a.source, a.target}, {a.target, a.source}} {
			list := &t.matrix.occs[key[0]][key[1]]
			*list = (*list)[:len(*list)-1]
		}
	}
	t.atoms, t.unsupported = t.atoms[:s.atoms], s.unsupported
	t.scopes = t.scopes[:level]
	if err := t.matrix.pop(count); err != nil {
		return err
	}
	t.integer = t.integer[:len(t.matrix.rows)]
	if err := t.adapter.PopScopes(count); err != nil {
		return err
	}
	return t.SMTTheoryBase.PopScopes(count)
}

// smtDenseVarValueKey is var_value_hash/var_value_eq: two variables share a
// table entry when their assignments and their is_int flags are equal.
type smtDenseVarValueKey struct {
	integer             bool
	main, infinitesimal string
}

// FinalCheck is theory_dense_diff_logic<Ext>::final_check_eh.
func (t *SMTDenseDiffTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	t.values = t.matrix.assignment() // init_model()
	t.traceDDL("ddl_model", "init_model", 733, t.ddlModelText)
	assumed, err := t.assumeEqs()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if assumed {
		return SMTFinalCheckContinue, nil
	}
	// logical context contains arithmetic expressions that are not
	// in the difference logic fragment.
	if t.unsupported {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

// assumeEqs is theory::assume_eqs(m_var_value_table): every relevant and
// shared variable is looked up by value; the first variable with a value
// stays its representative, and assume_eq(n, n2) is tried for every later
// one.
func (t *SMTDenseDiffTheory) assumeEqs() (bool, error) {
	table := make(map[smtDenseVarValueKey]int)
	result := false
	for v, node := range t.variables {
		if node == nil || !t.state.isRelevant(node.owner) {
			continue
		}
		shared, err := smtSharedArithmeticNode(t.SMTTheoryBase, node, false)
		if err != nil {
			return false, err
		}
		if !shared {
			continue
		}
		key := smtDenseVarValueKey{t.integer[v], t.values[v].rational.RatString(), t.values[v].infinitesimal.RatString()}
		other, found := table[key] // insert_if_not_there(v)
		if !found {
			table[key] = v
			continue
		}
		progress, err := t.state.assumeEquality(node, t.variables[other])
		if err != nil {
			return false, err
		}
		if progress {
			result = true
		}
	}
	return result, nil
}

func (t *SMTDenseDiffTheory) InitializeModel(generator *SMTModelGenerator) error {
	if generator == nil || generator.manager != t.state.manager || generator.model == nil {
		return ast.ErrASTDomain
	}
	if len(t.values) != t.NumVariables() {
		return ast.ErrASTDomain
	}
	factory, err := protomodel.NewArithmeticFactory(t.state.manager)
	if err != nil {
		return err
	}
	if err = generator.RegisterFactory(factory); err != nil {
		return err
	}
	for v, node := range t.variables {
		if value, numeral, err := t.syntax.numeral(node.owner); err != nil {
			return err
		} else if numeral && value.Sign() == 0 {
			var shift smtInfRational
			shift.Set(&t.values[v])
			sort, err := t.state.manager.GetSort(node.owner)
			if err != nil {
				return err
			}
			for other, n := range t.variables {
				otherSort, err := t.state.manager.GetSort(n.owner)
				if err != nil {
					return err
				}
				if otherSort == sort {
					t.values[other].Sub(&t.values[other], &shift)
				}
			}
		}
	}
	t.traceDDL("ddl_model", "fix_zero", 815, t.ddlModelText)
	// compute_epsilon
	t.delta.SetInt64(1)
	for _, e := range t.matrix.edges[1:] {
		x, y := &t.values[e.target], &t.values[e.source]
		t.traceDDL("epsilon", "compute_epsilon", 786, func() string {
			return fmt.Sprintf("(n_x,k_x): %s, %s, (n_y,k_y): %s, %s, (n_c,k_c): %s, %s\n",
				x.rational.RatString(), x.infinitesimal.RatString(), y.rational.RatString(), y.infinitesimal.RatString(),
				e.offset.rational.RatString(), e.offset.infinitesimal.RatString())
		})
		var limit, infinitesimal big.Rat
		limit.Add(&y.rational, &e.offset.rational)
		infinitesimal.Add(&y.infinitesimal, &e.offset.infinitesimal)
		if x.rational.Cmp(&limit) < 0 && x.infinitesimal.Cmp(&infinitesimal) > 0 {
			var numerator, denominator, candidate big.Rat
			numerator.Sub(&limit, &x.rational)
			denominator.Sub(&x.infinitesimal, &infinitesimal)
			candidate.Quo(&numerator, &denominator)
			if candidate.Cmp(&t.delta) < 0 {
				t.traceDDL("epsilon", "compute_epsilon", 790, func() string { return "new epsilon: " + candidate.RatString() + "\n" })
				t.delta.Set(&candidate)
			}
		}
	}
	return nil
}
func (t *SMTDenseDiffTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if node == nil || generator == nil || generator.manager != t.state.manager {
		return nil, ast.ErrASTDomain
	}
	v := node.TheoryVar(t.FamilyID())
	if v == SMTNullTheoryVar || int(v) >= len(t.values) {
		return nil, ast.ErrASTDomain
	}
	var epsilon, value big.Rat
	epsilon.Mul(&t.delta, &t.values[v].infinitesimal)
	value.Add(&t.values[v].rational, &epsilon)
	expression, err := t.state.manager.ArithmeticNumeralRat(&value, t.integer[v])
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(expression)
}

// denseBinaryApp is arith_util::is_add(n, a1, a2) / is_mul(n, a1, a2): an
// application of op with exactly two arguments.
func (t *SMTDenseDiffTheory) denseBinaryApp(expression ast.ASTHandle, op int32) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, arithmetic, err := t.syntax.operation(expression)
	if err != nil || !arithmetic || kind != op {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	m := t.state.manager
	count, err := m.GetNumArgs(expression)
	if err != nil || count != 2 {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	first, err := m.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	second, err := m.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return first, second, true, nil
}

// denseTimesMinusOne is arith_util::is_times_minus_one: (* -1 r).
func (t *SMTDenseDiffTheory) denseTimesMinusOne(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	coefficient, r, ok, err := t.denseBinaryApp(expression, smtArithOpMul)
	if err != nil || !ok {
		return ast.ASTHandle{}, false, err
	}
	value, numeral, err := t.syntax.numeral(coefficient)
	if err != nil || !numeral || value.Cmp(big.NewRat(-1, 1)) != 0 {
		return ast.ASTHandle{}, false, err
	}
	return r, true, nil
}

// denseZeroFor is mk_zero_for: the numeral 0 of n's sort.
func (t *SMTDenseDiffTheory) denseZeroFor(expression ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.state.manager
	sort, err := m.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	integer, err := t.syntax.integerSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ArithmeticNumeralRat(new(big.Rat), integer)
}
