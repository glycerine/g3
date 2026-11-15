package transforms

// Source: src/muz/transforms/dl_mk_slice.cpp, mk_slice::slice_proof_converter:
// init_form2rule, translate_proof, translate_asserted, translate_hyper_res
// and operator().

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// sliceProofTranslation holds the converter members that translate_proof
// reads and extends: m_sliceform2rule, the pinned rules and expressions it
// creates, and m_unifier.
type sliceProofTranslation struct {
	form2rule   map[ast.ASTHandle]*base.Rule
	pinnedRules []*base.Rule
	pinnedExprs []ast.ASTRef
	unifier     *NormalizingRuleUnifier
}

// SetRuleUnifier installs m_unifier, the converter's rule_unifier built over
// the datalog context. The converter owns and closes it.
func (c *SliceProofConverter) SetRuleUnifier(unifier *NormalizingRuleUnifier) error {
	if c == nil || c.manager == nil || unifier == nil {
		return ast.ErrASTHandle
	}
	if c.translation.unifier != nil {
		if err := c.translation.unifier.Close(); err != nil {
			return errors.Join(err, unifier.Close())
		}
	}
	c.translation.unifier = unifier
	return nil
}

func (c *SliceProofConverter) closeTranslation() error {
	t := &c.translation
	if t.unifier != nil {
		if err := t.unifier.Close(); err != nil {
			return err
		}
		t.unifier = nil
	}
	for len(t.pinnedExprs) != 0 {
		last := len(t.pinnedExprs) - 1
		if err := c.manager.DelRef(&t.pinnedExprs[last]); err != nil {
			return err
		}
		t.pinnedExprs = t.pinnedExprs[:last]
	}
	for len(t.pinnedRules) != 0 {
		last := len(t.pinnedRules) - 1
		if err := t.pinnedRules[last].Close(); err != nil {
			return err
		}
		t.pinnedRules = t.pinnedRules[:last]
	}
	t.form2rule = nil
	return nil
}

// ConvertProof is slice_proof_converter::operator(): init_form2rule and then
// translate_proof. It returns an owning reference to the translated proof.
func (c *SliceProofConverter) ConvertProof(proof ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || c.rules == nil {
		return result, ast.ErrASTHandle
	}
	if err := c.initForm2Rule(); err != nil {
		return result, err
	}
	return c.translateProof(proof)
}

// initForm2Rule maps every sliced rule formula to its original rule. The
// source iterates m_rule2slice in pointer-hash order; insertion order is used
// here, with a later rule replacing an earlier one of the same formula.
func (c *SliceProofConverter) initForm2Rule() error {
	t := &c.translation
	if len(t.form2rule) != 0 {
		return nil
	}
	if t.form2rule == nil {
		t.form2rule = make(map[ast.ASTHandle]*base.Rule)
	}
	for index, formula := range c.formulas {
		fact, err := c.manager.RefGet(formula)
		if err != nil {
			return err
		}
		original := c.originals[index]
		if err = c.traceDL("init_form2rule", 126, func(out io.Writer) error {
			if err := c.writeTraceExpr(out, "orig: ", fact, "\n"); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "new:\n"); err != nil {
				return err
			}
			return c.rules.WriteRuleDisplay(out, original)
		}); err != nil {
			return err
		}
		t.form2rule[fact] = original
	}
	return nil
}

func (c *SliceProofConverter) translateProof(root ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	newProof := make(map[ast.ASTHandle]ast.ASTHandle)
	todo := []ast.ASTHandle{root}
	for len(todo) != 0 {
		p := todo[len(todo)-1]
		if _, found := newProof[p]; found {
			todo = todo[:len(todo)-1]
			continue
		}
		done, err := c.translateAsserted(p, newProof, &todo)
		if err != nil {
			return result, err
		}
		if done {
			continue
		}
		done, err = c.translateHyperRes(p, newProof, &todo)
		if err != nil {
			return result, err
		}
		if done {
			continue
		}
		newProof[p] = p
		todo = todo[:len(todo)-1]
		if err = c.traceDL("translate_proof", 151, func(out io.Writer) error {
			return c.writeTraceExpr(out, "unhandled proof term\n", p, "\n")
		}); err != nil {
			return result, err
		}
	}
	return c.manager.NewRef(newProof[root])
}

func (c *SliceProofConverter) translateAsserted(p ast.ASTHandle, newProof map[ast.ASTHandle]ast.ASTHandle, todo *[]ast.ASTHandle) (bool, error) {
	var fact ast.ASTHandle
	asserted, err := c.manager.MatchProofAsserted(p, &fact)
	if err != nil || !asserted {
		return false, err
	}
	rule, found := c.translation.form2rule[fact]
	if !found {
		return false, c.traceDL("translate_asserted", 164, func(out io.Writer) error {
			return c.writeTraceExpr(out, "does not have fact\n", fact, "\n")
		})
	}
	replacement, found, err := rule.Proof()
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("muz/transforms: sliced rule has no original proof: %w", ast.ErrASTDomain)
	}
	if err = c.pinExpr(replacement); err != nil {
		return false, err
	}
	*todo = (*todo)[:len(*todo)-1]
	newProof[p] = replacement
	return true, nil
}

func (c *SliceProofConverter) translateHyperRes(p ast.ASTHandle, newProof map[ast.ASTHandle]ast.ASTHandle, todo *[]ast.ASTHandle) (_ bool, resultErr error) {
	m := c.manager
	t := &c.translation
	conclusion, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&conclusion)) }()
	output, err := ast.NewASTProofHyperOutput(m)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, output.Close()) }()
	var premises0 []ast.ASTHandle
	defer func() {
		for _, premise := range premises0 {
			resultErr = errors.Join(resultErr, m.DecRef(premise))
		}
	}()
	matched, err := m.MatchProofHyperResolve(p, &premises0, conclusion, output)
	if err != nil || !matched {
		return false, err
	}
	sliceConclusion, err := m.RefGet(conclusion)
	if err != nil {
		return false, err
	}
	allFound := true
	for _, argument := range premises0 {
		if _, found := newProof[argument]; !found {
			*todo = append(*todo, argument)
			allFound = false
		}
	}
	if !allFound {
		return true, nil
	}
	if len(premises0) == 0 {
		return false, ast.ErrASTDomain
	}
	var premises []ast.ASTHandle
	p0 := premises0[0]
	p0New := newProof[p0]
	fact0, err := m.GetProofFact(p0)
	if err != nil {
		return false, err
	}
	if err = c.traceDL("translate_hyper_res", 204, func(out io.Writer) error {
		return c.writeTraceExpr(out, "fact0: ", fact0, "\n")
	}); err != nil {
		return false, err
	}
	orig0, found := t.form2rule[fact0]
	if !found {
		return false, nil
	}
	premises = append(premises, p0New)
	if t.unifier == nil {
		return false, ast.ErrASTHandle
	}
	// rule_ref r1, r2, r3: the resolvents are owned locally; the final one
	// is pinned below.
	var locals []*base.Rule
	defer func() {
		for index := len(locals) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, locals[index].Close())
		}
	}()
	var references []ast.ASTRef
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, m.DelRef(&references[index]))
		}
	}()
	r1 := orig0
	substitutions := [][]ast.ASTHandle{{}}
	for i := 1; i < len(premises0); i++ {
		p1 := premises0[i]
		p1New := newProof[p1]
		fact1, err := m.GetProofFact(p1)
		if err != nil {
			return false, err
		}
		if err = c.traceDL("translate_hyper_res", 217, func(out io.Writer) error {
			return c.writeTraceExpr(out, "fact1: ", fact1, "\n")
		}); err != nil {
			return false, err
		}
		orig1, found := t.form2rule[fact1]
		if !found {
			return false, nil
		}
		premises = append(premises, p1New)
		r2 := orig1
		// unsigned idx = 0; brittle. TBD get index from positions.
		unified, err := t.unifier.UnifyRules(r1, 0, r2)
		if err != nil {
			return false, err
		}
		if !unified {
			// VERIFY(m_unifier.unify_rules(*r1, idx, *r2))
			return false, fmt.Errorf("muz/transforms: slice proof premises do not unify: %w", ast.ErrASTDomain)
		}
		r3, applied, err := t.unifier.Apply(r1, 0, r2)
		if r3 != nil {
			locals = append(locals, r3)
		}
		if err != nil {
			return false, err
		}
		if !applied {
			return false, fmt.Errorf("muz/transforms: slice proof resolution is unsatisfiable: %w", ast.ErrASTDomain)
		}
		sub1Refs, err := t.unifier.RuleSubstitution(r1, true)
		references = append(references, sub1Refs...)
		if err != nil {
			return false, err
		}
		sub1, err := refHandles(m, sub1Refs)
		if err != nil {
			return false, err
		}
		for j := range substitutions {
			if substitutions[j], err = c.applySubstitution(&references, substitutions[j], sub1); err != nil {
				return false, err
			}
		}
		sub2Refs, err := t.unifier.RuleSubstitution(r2, false)
		references = append(references, sub2Refs...)
		if err != nil {
			return false, err
		}
		sub2, err := refHandles(m, sub2Refs)
		if err != nil {
			return false, err
		}
		substitutions = append(substitutions, sub2)
		if err = c.traceDL("translate_hyper_res", 236, func(out io.Writer) error {
			for _, item := range []struct {
				label string
				rule  *base.Rule
			}{{"rule1:", r1}, {"rule2:", r2}, {"res:", r3}} {
				if _, err := io.WriteString(out, item.label); err != nil {
					return err
				}
				if err := c.rules.WriteRuleDisplay(out, item.rule); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return false, err
		}
		r1 = r3
	}
	formula, err := r1.ToFormula()
	if err != nil {
		return false, err
	}
	references = append(references, formula)
	concl, err := m.RefGet(formula)
	if err != nil {
		return false, err
	}
	positions := make([]ast.ASTProofPosition, output.PositionsLen())
	for index := range positions {
		if positions[index], err = output.Position(uint32(index)); err != nil {
			return false, err
		}
	}
	// Reserve the pin first so an allocation failure cannot strand the node.
	pin, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	t.pinnedExprs = append(t.pinnedExprs, pin)
	newP, err := m.MkProofHyperResolve(premises, concl, positions, substitutions)
	if !newP.IsNull() {
		err = errors.Join(err, m.SetRef(pin, newP))
	}
	if err != nil {
		return false, err
	}
	pinned, err := r1.Retain()
	if err != nil {
		return false, err
	}
	t.pinnedRules = append(t.pinnedRules, pinned)
	if err = c.traceDL("translate_hyper_res", 246, func(out io.Writer) error {
		if err := c.writeTraceExpr(out, "orig: ", sliceConclusion, "\n"); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "new:"); err != nil {
			return err
		}
		return c.rules.WriteRuleDisplay(out, pinned)
	}); err != nil {
		return false, err
	}
	// m_sliceform2rule.insert(slice_concl, r1.get()); the conclusion stays
	// alive in the pinned proof.
	if err = c.pinExpr(sliceConclusion); err != nil {
		return false, err
	}
	t.form2rule[sliceConclusion] = pinned
	newProof[p] = newP
	*todo = (*todo)[:len(*todo)-1]
	return true, c.traceDL("translate_hyper_res", 254, func(out io.Writer) error {
		if err := c.writeTraceExpr(out, "translated:\n", p, "\nto\n"); err != nil {
			return err
		}
		return c.writeTraceExpr(out, "", newP, "\n")
	})
}

// applySubstitution is apply_subst(tgt, sub) from dl_util.cpp.
func (c *SliceProofConverter) applySubstitution(references *[]ast.ASTRef, target, sub []ast.ASTHandle) (_ []ast.ASTHandle, resultErr error) {
	substitution, err := rewriter.NewVarSubstitution(c.manager, false)
	if substitution != nil {
		defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	}
	if err != nil {
		return nil, err
	}
	result := append([]ast.ASTHandle(nil), target...)
	for index := range result {
		if result[index].IsNull() {
			result[index] = sub[index]
			continue
		}
		reference, err := c.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, err
		}
		*references = append(*references, reference)
		if err = substitution.Substitute(result[index], sub, reference); err != nil {
			return nil, err
		}
		if result[index], err = c.manager.RefGet(reference); err != nil {
			return nil, err
		}
	}
	for index := len(result); index < len(sub); index++ {
		result = append(result, sub[index])
	}
	return result, nil
}

func (c *SliceProofConverter) pinExpr(expression ast.ASTHandle) error {
	reference, err := c.manager.NewRef(expression)
	if err == nil {
		c.translation.pinnedExprs = append(c.translation.pinnedExprs, reference)
	}
	return err
}

func refHandles(manager *ast.ASTManager, references []ast.ASTRef) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(references))
	for index := range references {
		var err error
		if result[index], err = manager.RefGet(references[index]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// traceDL emits one TRACE("dl", ...) site of dl_mk_slice.cpp.
func (c *SliceProofConverter) traceDL(function string, line int, body func(io.Writer) error) error {
	tags := c.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("dl", function, "src/muz/transforms/dl_mk_slice.cpp", line,
		func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (c *SliceProofConverter) writeTraceExpr(out io.Writer, prefix string, expression ast.ASTHandle, suffix string) error {
	printer, err := ast.NewSimpleASTPrinterContext(c.manager, nil, nil)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, prefix); err != nil {
		return err
	}
	if err = printer.DisplayExpr(out, expression, 0); err != nil {
		return err
	}
	_, err = io.WriteString(out, suffix)
	return err
}
