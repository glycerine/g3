package pdr

// Source: model_node::mk_instantiate, model_search::get_proof_trace,
// context::get_proof and context::mk_sat_answer in
// src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// ProofRuleServices is the datalog::context that get_proof_trace reaches
// through ctx.get_context(): its rule manager and a fresh rule_unifier
// (datalog::rule_unifier unif(dctx)) built with the context's services.
type ProofRuleServices interface {
	NewProofRuleUnifier() (*transforms.NormalizingRuleUnifier, error)
}

// ProofConverter is the datalog context's proof converter (m_pc), installed
// by dl_interface::query through set_proof_converter. ConvertProof is
// apply(m, m_pc.get(), proof) and returns an owning reference.
type ProofConverter interface {
	ConvertProof(proof ast.ASTHandle) (ast.ASTRef, error)
}

// SetProofRuleServices installs the datalog context used by get_proof_trace.
// The context borrows the services.
func (c *Context) SetProofRuleServices(services ProofRuleServices) error {
	if err := c.open(); err != nil {
		return err
	}
	c.proofRules = services
	return nil
}

// SetProofConverter is context::set_proof_converter. A nil converter is the
// null proof_converter_ref. The context borrows the converter.
func (c *Context) SetProofConverter(converter ProofConverter) error {
	if err := c.open(); err != nil {
		return err
	}
	c.proofConverter = converter
	return nil
}

func (c *Context) completeTraceAnswer(root *ModelNode) (result ast.ASTRef, resultErr error) {
	constraints, err := c.unifiedTraceConstraints(root, true, func(handles []ast.ASTHandle) error {
		formula, x := c.manager.MkAndArray(handles)
		if x == nil {
			result, x = c.manager.NewRef(formula)
		}
		return x
	})
	if err != nil {
		if result != (ast.ASTRef{}) {
			err = errors.Join(err, c.manager.DelRef(&result))
		}
		return ast.ASTRef{}, fmt.Errorf("pdr: unify complete proof trace: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, closeASTRefs(c.manager, constraints))
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	return result, nil
}

// Proof is context::get_proof: the model search's proof trace with the
// datalog context's proof converter applied. The caller releases the
// returned reference through ASTManager.DelRef.
func (c *Context) Proof(queryDeclaration ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	if !c.generateProofTrace || c.lastResult != lbool.True || c.queryDecl != queryDeclaration || c.query == nil {
		return result, ast.ErrASTDomain
	}
	root, found, err := c.search.Root()
	if err != nil || !found {
		if err != nil {
			return result, err
		}
		return result, ast.ErrASTDomain
	}
	// context::get_proof's scoped_proof covers get_proof_trace, including
	// its update_models, and the proof converter.
	proofsEnabled, err := c.manager.ProofsEnabled()
	if err != nil {
		return result, err
	}
	if !proofsEnabled {
		if err = c.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return result, err
		}
		defer func() { resultErr = errors.Join(resultErr, c.manager.ToggleProofMode(ast.ASTProofDisabled)) }()
	}
	result, err = c.proofTrace(root)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	if err = c.traceProof(result, 1969); err != nil {
		return result, err
	}
	// apply(m, m_pc.get(), proof)
	if c.proofConverter != nil {
		proof, x := c.manager.RefGet(result)
		if x != nil {
			return result, x
		}
		converted, x := c.proofConverter.ConvertProof(proof)
		if x != nil {
			return result, x
		}
		if x = c.manager.DelRef(&result); x != nil {
			return converted, x
		}
		result = converted
	}
	return result, c.traceProof(result, 1971)
}

// traceProof is TRACE("pdr", tout << "PDR trace: " << proof << "\n").
func (c *Context) traceProof(proof ast.ASTRef, line int) error {
	tags := c.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "get_proof", "src/muz/pdr/pdr_context.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := pdrTraceOstream{out: raw}
			handle, x := c.manager.RefGet(proof)
			if x != nil {
				return x
			}
			printer, x := ast.NewSimpleASTPrinterContext(c.manager, nil, nil)
			if x != nil {
				return x
			}
			if _, x = io.WriteString(out, "PDR trace: "); x != nil {
				return x
			}
			if x = printer.DisplayExpr(out, handle, 0); x != nil {
				return x
			}
			_, x = io.WriteString(out, "\n")
			return x
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// proofTraceOwner holds get_proof_trace's locals: the rule_unifier, the
// proof trail, the rules trail and the per-node temporaries.
type proofTraceOwner struct {
	manager    *ast.ASTManager
	unifier    *transforms.NormalizingRuleUnifier
	references []ast.ASTRef
	rules      []*base.Rule
}

// own reserves a reference before building a node, so an allocation
// failure cannot strand the fresh node, then stores the node in it.
func (o *proofTraceOwner) own(build func() (ast.ASTHandle, error)) (ast.ASTHandle, error) {
	reference, err := o.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	o.references = append(o.references, reference)
	handle, err := build()
	if !handle.IsNull() {
		err = errors.Join(err, o.manager.SetRef(reference, handle))
	}
	return handle, err
}

func (o *proofTraceOwner) close() (result error) {
	for index := len(o.rules) - 1; index >= 0; index-- {
		result = errors.Join(result, o.rules[index].Close())
	}
	o.rules = nil
	result = errors.Join(result, closeASTRefs(o.manager, o.references))
	o.references = nil
	if o.unifier != nil {
		result = errors.Join(result, o.unifier.Close())
		o.unifier = nil
	}
	return result
}

// defaultProofRuleServices stands in for a datalog context when the PDR
// context is driven directly: a rule manager over the context's predicates
// and a rule unifier without parameter-driven normalization.
type defaultProofRuleServices struct {
	context *Context
	rules   *base.RuleManager
}

func (s *defaultProofRuleServices) NewProofRuleUnifier() (*transforms.NormalizingRuleUnifier, error) {
	return transforms.NewNormalizingRuleUnifier(transforms.NewRuleInlinerPlugin(s.context.manager, s.rules, nil, 0))
}

// mkInstantiate is model_node::mk_instantiate: r0 is the node's rule, the
// binding maps each rule variable to its value in the node state (or keeps
// the variable), and r1 is r0 with the binding substituted.
func (c *Context) mkInstantiate(owner *proofTraceOwner, rules *base.RuleManager, node *ModelNode) (r0 *normalizedRule, r1 *base.Rule, binding []ast.ASTHandle, resultErr error) {
	values, err := c.traceStateValues(node)
	if err != nil {
		return nil, nil, nil, err
	}
	r0, err = c.selectedTraceRule(node)
	if err != nil {
		return nil, nil, nil, err
	}
	instances := r0.instances
	if err = c.traceInstantiate(node, len(instances)); err != nil {
		return nil, nil, nil, err
	}
	// Source instances are never null; a gap left by an unused variable
	// index takes the rule's variable sort (Bool for an unused index).
	var sorts []ast.ASTHandle
	for index, instance := range instances {
		if value, found := values[instance]; found && !instance.IsNull() {
			binding = append(binding, value)
			continue
		}
		var sort ast.ASTHandle
		if !instance.IsNull() {
			sort, err = c.manager.GetSort(instance)
		} else {
			if sorts == nil {
				sorts, err = r0.rule.VariableSorts()
			}
			if err == nil && index < len(sorts) {
				sort = sorts[index]
			} else if err == nil {
				sort, err = c.manager.MkBoolSort()
			}
		}
		if err != nil {
			return nil, nil, nil, err
		}
		variable, err := owner.own(func() (ast.ASTHandle, error) { return c.manager.MkVar(uint32(index), sort) })
		if err != nil {
			return nil, nil, nil, err
		}
		binding = append(binding, variable)
	}
	if len(instances) == 0 {
		r1, err = r0.rule.Retain()
	} else {
		r1, err = rules.SubstituteRule(r0.rule, binding)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	owner.rules = append(owner.rules, r1)
	return r0, r1, binding, nil
}

// proofTrace is model_search::get_proof_trace. It returns an owning
// reference to the root's proof.
func (c *Context) proofTrace(root *ModelNode) (result ast.ASTRef, resultErr error) {
	services := c.proofRules
	if services == nil {
		rules, err := base.NewRuleManager(c.manager)
		if err != nil {
			return result, err
		}
		defer func() { resultErr = errors.Join(resultErr, rules.Close()) }()
		for _, declaration := range c.order {
			if err = rules.RegisterPredicate(declaration); err != nil {
				return result, err
			}
		}
		services = &defaultProofRuleServices{context: c, rules: rules}
	}
	owner := &proofTraceOwner{manager: c.manager}
	defer func() { resultErr = errors.Join(resultErr, owner.close()) }()
	unifier, err := services.NewProofRuleUnifier()
	if err != nil {
		return result, err
	}
	owner.unifier = unifier
	rules := unifier.RuleManager()
	// The source cache is keyed by n->state() alone. Distinct nullary
	// predicates share the true state there, so a later node can take
	// another predicate's proof; key by the transformer as well, as the
	// search cache does.
	cache := make(map[modelStateKey]ast.ASTHandle)
	ruleCache := make(map[modelStateKey]*base.Rule)
	todo := []*ModelNode{root}
	if err = c.updateTraceModels(root); err != nil {
		return result, err
	}
	// The search closes a repeated obligation without expanding it. In the
	// source such a childless duplicate can be reached first and cached
	// with its unresolved rule, so the proof keeps a predicate tail and no
	// longer concludes the query. Resolve the expanded node of the same
	// state first and share its derivation (a z3 4.7.1 defect; g3 keeps
	// the proof valid).
	expanded := make(map[modelStateKey]*ModelNode)
	for stack := []*ModelNode{root}; len(stack) != 0; {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		children, x := node.Children()
		if x != nil {
			return result, x
		}
		if len(children) != 0 {
			if _, found := expanded[modelState(node)]; !found {
				expanded[modelState(node)] = node
			}
		}
		stack = append(stack, children...)
	}
	for len(todo) != 0 {
		node := todo[len(todo)-1]
		if err = c.traceProofNode(node); err != nil {
			return result, err
		}
		if _, found := cache[modelState(node)]; found {
			todo = todo[:len(todo)-1]
			continue
		}
		children, err := node.Children()
		if err != nil {
			return result, err
		}
		if representative := expanded[modelState(node)]; len(children) == 0 && representative != nil && representative != node {
			todo = append(todo, representative)
			continue
		}
		premises := []ast.ASTHandle{{}}
		sources := []*base.Rule{nil}
		for _, child := range children {
			if proof, found := cache[modelState(child)]; found {
				premises = append(premises, proof)
				sources = append(sources, ruleCache[modelState(child)])
			} else {
				todo = append(todo, child)
			}
		}
		if len(premises) != 1+len(children) {
			continue
		}
		r0, r1, binding, err := c.mkInstantiate(owner, rules, node)
		if err != nil {
			return result, err
		}
		p1, found, err := r0.rule.Proof()
		if err != nil {
			return result, err
		}
		if !found {
			// IF_VERBOSE(0, if (!p1) r0->display(dctx, verbose_stream()););
			if out, enabled, x := pdrVerboseStream(0); x != nil {
				return result, x
			} else if enabled {
				_ = rules.WriteRuleDisplay(pdrDiagnosticWriter{out: out}, r0.rule)
			}
			// SASSERT(p1): z3 4.7.1 goes on with a null premise and
			// crashes (print_boogie_certificate without
			// generate_proof_trace). g3 asserts the rule instead.
			formula, x := r0.rule.ToFormula()
			if x != nil {
				return result, x
			}
			owner.references = append(owner.references, formula)
			fact, x := c.manager.RefGet(formula)
			if x == nil {
				p1, x = owner.own(func() (ast.ASTHandle, error) { return c.manager.MkProofAsserted(fact) })
			}
			if x != nil {
				return result, x
			}
		}
		premises[0] = p1
		sources[0] = r1
		if err = c.traceProofRoot(p1, binding, premises[1:]); err != nil {
			return result, err
		}
		reduced := sources[0]
		// check if binding is identity.
		identity := true
		for index := 0; identity && index < len(binding); index++ {
			kind, x := c.manager.GetKind(binding[index])
			if x != nil {
				return result, x
			}
			identity = kind == ast.ASTVar
			if identity {
				variable, x := c.manager.GetVarIndex(binding[index])
				if x != nil {
					return result, x
				}
				identity = variable == uint32(index)
			}
		}
		if len(sources) > 1 || !identity {
			substitutions := [][]ast.ASTHandle{append([]ast.ASTHandle(nil), binding...)}
			var positions []ast.ASTProofPosition
			resolvedPremises := []ast.ASTHandle{premises[0]}
			for index := 1; index < len(sources); index++ {
				source := sources[index]
				tails, x := reduced.UninterpretedTailSize()
				if x != nil {
					return result, x
				}
				unified := false
				if tails != 0 {
					if unified, x = unifier.UnifyRules(reduced, 0, source); x != nil {
						return result, x
					}
				}
				if !unified {
					if out, enabled, y := pdrVerboseStream(0); y != nil {
						return result, y
					} else if enabled {
						w := pdrDiagnosticWriter{out: out}
						_, _ = io.WriteString(w, "Could not unify rules: ")
						_ = rules.WriteRuleDisplay(w, reduced)
						_ = rules.WriteRuleDisplay(w, source)
					}
					// z3 4.7.1 goes on with the stale unifier and builds a
					// proof of a garbled rule. The child's tail was merged
					// with an identical, already resolved one by
					// remove_duplicate_tails: g3 leaves the premise out.
					continue
				}
				first, x := c.proofRuleSubstitution(owner, unifier, reduced, true)
				if x != nil {
					return result, x
				}
				if x = c.traceProofSubstitution(first); x != nil {
					return result, x
				}
				if x = c.reapplyProofSubstitutions(owner, substitutions, first); x != nil {
					return result, x
				}
				resolvedPremises = append(resolvedPremises, premises[index])
				positions = append(positions, ast.ASTProofPosition{First: uint32(len(resolvedPremises) - 1), Second: 0})
				second, x := c.proofRuleSubstitution(owner, unifier, source, false)
				if x != nil {
					return result, x
				}
				substitutions = append(substitutions, second)
				resolved, applied, x := unifier.Apply(reduced, 0, source)
				if resolved != nil {
					owner.rules = append(owner.rules, resolved)
				}
				if x != nil {
					return result, x
				}
				if !applied {
					// VERIFY(unif.apply(*reduced_rule.get(), 0, src, r3))
					return result, fmt.Errorf("pdr: proof trace resolution is unsatisfiable: %w", ast.ErrASTDomain)
				}
				reduced = resolved
			}
			conclusion, x := reduced.ToFormula()
			if x != nil {
				return result, x
			}
			owner.references = append(owner.references, conclusion)
			conclusionHandle, x := c.manager.RefGet(conclusion)
			if x != nil {
				return result, x
			}
			p1, x = owner.own(func() (ast.ASTHandle, error) {
				return c.manager.MkProofHyperResolve(resolvedPremises, conclusionHandle, positions, substitutions)
			})
			if x != nil {
				return result, x
			}
		}
		cache[modelState(node)] = p1
		ruleCache[modelState(node)] = reduced
		todo = todo[:len(todo)-1]
	}
	return c.manager.NewRef(cache[modelState(root)])
}

// proofRuleSubstitution is unif.get_rule_subst(r, is_tgt), kept alive in
// the trace owner.
func (c *Context) proofRuleSubstitution(owner *proofTraceOwner, unifier *transforms.NormalizingRuleUnifier, rule *base.Rule, isTarget bool) ([]ast.ASTHandle, error) {
	references, err := unifier.RuleSubstitution(rule, isTarget)
	owner.references = append(owner.references, references...)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(references))
	for index := range references {
		if result[index], err = c.manager.RefGet(references[index]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// reapplyProofSubstitutions composes every earlier substitution with sub1
// (var_subst(m, false)) and extends it to sub1's length.
func (c *Context) reapplyProofSubstitutions(owner *proofTraceOwner, substitutions [][]ast.ASTHandle, sub1 []ast.ASTHandle) (resultErr error) {
	substitution, err := rewriter.NewVarSubstitution(c.manager, false)
	if substitution != nil {
		defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	}
	if err != nil {
		return err
	}
	for j := range substitutions {
		for k := range substitutions[j] {
			reference, x := c.manager.NewRef(ast.ASTHandle{})
			if x != nil {
				return x
			}
			owner.references = append(owner.references, reference)
			if x = substitution.Substitute(substitutions[j][k], sub1, reference); x != nil {
				return x
			}
			if substitutions[j][k], x = c.manager.RefGet(reference); x != nil {
				return x
			}
		}
		for len(substitutions[j]) < len(sub1) {
			substitutions[j] = append(substitutions[j], sub1[len(substitutions[j])])
		}
	}
	return nil
}

// traceProofNode is TRACE("pdr", n->display(tout, 0);).
func (c *Context) traceProofNode(node *ModelNode) error {
	tags := c.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "get_proof_trace", "src/muz/pdr/pdr_context.cpp", 1309,
		func() bool { return true }, func(raw io.Writer) error {
			return node.WriteDisplay(pdrTraceOstream{out: raw}, 0)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// emitProofTrace writes one get_proof_trace/mk_instantiate TRACE("pdr") body.
func (c *Context) emitProofTrace(function string, line int, body func(out io.Writer, printer *ast.SimpleASTPrinterContext) error) error {
	tags := c.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", function, "src/muz/pdr/pdr_context.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			printer, x := ast.NewSimpleASTPrinterContext(c.manager, nil, nil)
			if x != nil {
				return x
			}
			return body(pdrTraceOstream{out: raw}, printer)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// traceInstantiate is TRACE("pdr", tout << state() << " instance: " <<
// inst.size() << "\n";) in model_node::mk_instantiate.
func (c *Context) traceInstantiate(node *ModelNode, instances int) error {
	return c.emitProofTrace("mk_instantiate", 853, func(out io.Writer, printer *ast.SimpleASTPrinterContext) error {
		if x := printer.DisplayExpr(out, node.state, 0); x != nil {
			return x
		}
		_, x := fmt.Fprintf(out, " instance: %d\n", instances)
		return x
	})
}

// traceProofRoot prints the rule proof, the binding and the child proofs.
func (c *Context) traceProofRoot(root ast.ASTHandle, binding, children []ast.ASTHandle) error {
	return c.emitProofTrace("get_proof_trace", 1340, func(out io.Writer, printer *ast.SimpleASTPrinterContext) error {
		if _, x := io.WriteString(out, "root: "); x != nil {
			return x
		}
		if x := printer.DisplayExpr(out, root, 0); x != nil {
			return x
		}
		if _, x := io.WriteString(out, "\n"); x != nil {
			return x
		}
		for _, group := range [][]ast.ASTHandle{binding, children} {
			for _, expression := range group {
				if x := printer.DisplayExpr(out, expression, 0); x != nil {
					return x
				}
				if _, x := io.WriteString(out, "\n"); x != nil {
					return x
				}
			}
		}
		return nil
	})
}

// traceProofSubstitution prints sub1, each term followed by a space.
func (c *Context) traceProofSubstitution(substitution []ast.ASTHandle) error {
	return c.emitProofTrace("get_proof_trace", 1372, func(out io.Writer, printer *ast.SimpleASTPrinterContext) error {
		for _, expression := range substitution {
			if x := printer.DisplayExpr(out, expression, 0); x != nil {
				return x
			}
			if _, x := io.WriteString(out, " "); x != nil {
				return x
			}
		}
		_, x := io.WriteString(out, "\n")
		return x
	})
}
