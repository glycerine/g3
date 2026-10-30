package transforms_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestUnboundCompressorProjectsHeadAndTailArgument(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	q := declareTransformPredicate(t, f, "compress-Q", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	p := declareTransformPredicate(t, f, "compress-P", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{x, y})
	goal := declareTransformPredicate(t, f, "compress-Goal", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	addTransformRule(t, f, p.application, []ast.ASTHandle{q.application}, nil, "definition")
	addTransformRule(t, f, goal.application, []ast.ASTHandle{p.application}, nil, "consumer")
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("compressor fixture rejected")
	}
	plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("unbound head argument was not compressed")
	}
	defer func() { coneMust(t, result.Close()) }()
	mapping := plugin.CompressedPredicates()
	compressed := mapping[p.declaration]
	if compressed.IsNull() {
		t.Fatal("missing compressed predicate")
	}
	if arity, err := f.manager.GetArity(compressed); err != nil || arity != 1 {
		t.Fatal("compressed arity", arity, err)
	}
	definitions, err := result.PredicateRules(compressed)
	coneMust(t, err)
	if len(definitions) != 1 {
		t.Fatalf("compressed definitions %d, want 1", len(definitions))
	}
	coneMust(t, definitions[0].Close())
	goals, err := result.PredicateRules(goal.declaration)
	coneMust(t, err)
	if len(goals) != 2 {
		t.Fatalf("goal rules %d, want source rule and decompression alternative", len(goals))
	}
	foundCompressed := false
	for _, goalRule := range goals {
		declaration, getErr := goalRule.TailDeclaration(0)
		if getErr != nil {
			coneMust(t, getErr)
		}
		foundCompressed = foundCompressed || declaration == compressed
		coneMust(t, goalRule.Close())
	}
	if !foundCompressed {
		t.Fatal("goal decompression alternative was not generated")
	}
}

func TestUnboundCompressorSkipsOutputsDisabledAndBoundHeads(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	p := declareTransformPredicate(t, f, "compress-skip-P", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	addTransformRule(t, f, p.application, []ast.ASTHandle{p.application}, nil, "bound")
	coneMust(t, f.set.SetOutputPredicate(p.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("skip fixture rejected")
	}
	for _, enabled := range []bool{false, true} {
		plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, enabled, 0)
		result, transformErr := plugin.Transform(f.set)
		coneMust(t, transformErr)
		if result != nil {
			coneMust(t, result.Close())
			t.Fatal("skip fixture changed", enabled)
		}
		coneMust(t, plugin.Close())
	}
}

func TestUnboundCompressorRepeatsForNewlyProjectedUnboundArguments(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	z, err := f.manager.MkVar(2, boolean)
	coneMust(t, err)
	q := declareTransformPredicate(t, f, "compress-repeat-Q", []ast.ASTHandle{boolean}, []ast.ASTHandle{z})
	p := declareTransformPredicate(t, f, "compress-repeat-P", []ast.ASTHandle{boolean, boolean, boolean}, []ast.ASTHandle{x, y, z})
	goal := declareTransformPredicate(t, f, "compress-repeat-Goal", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	addTransformRule(t, f, p.application, []ast.ASTHandle{q.application}, nil, "repeat-definition")
	addTransformRule(t, f, goal.application, []ast.ASTHandle{p.application}, nil, "repeat-consumer")
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("repeat-compression fixture rejected")
	}
	plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("unbound head arguments were not compressed")
	}
	defer func() { coneMust(t, result.Close()) }()
	compressed := plugin.CompressedPredicates()[p.declaration]
	if compressed.IsNull() {
		t.Fatal("missing first compressed predicate")
	}
	if got, getErr := f.manager.GetArity(compressed); getErr != nil || got != 2 {
		t.Fatalf("first projected arity %d, want 2: %v", got, getErr)
	}
	second := plugin.CompressedPredicates()[compressed]
	if second.IsNull() {
		t.Fatal("newly exposed unbound argument was not compressed")
	}
	if got, getErr := f.manager.GetArity(second); getErr != nil || got != 1 {
		t.Fatalf("second projected arity %d, want 1: %v", got, getErr)
	}
	goalRules, err := result.PredicateRules(goal.declaration)
	coneMust(t, err)
	defer func() {
		for _, rule := range goalRules {
			coneMust(t, rule.Close())
		}
	}()
	if len(goalRules) != 3 {
		t.Fatalf("iterative consumer alternatives %d, want original and two projections", len(goalRules))
	}
	seen := make(map[ast.ASTHandle]bool)
	for _, rule := range goalRules {
		declaration, getErr := rule.TailDeclaration(0)
		coneMust(t, getErr)
		seen[declaration] = true
	}
	if !seen[p.declaration] || !seen[compressed] || !seen[second] {
		t.Fatal("iterative decompression alternatives lost a source or projected predicate", seen)
	}
}

func TestUnboundCompressorUsesRuleSpecificHeadArguments(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	z, err := f.manager.MkVar(2, boolean)
	coneMust(t, err)
	p := declareTransformPredicate(t, f, "compress-specific-P", []ast.ASTHandle{boolean, boolean, boolean}, []ast.ASTHandle{x, y, z})
	q := declareTransformPredicate(t, f, "compress-specific-Q", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{y, z})
	r := declareTransformPredicate(t, f, "compress-specific-R", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{x, z})
	goal := declareTransformPredicate(t, f, "compress-specific-Goal", []ast.ASTHandle{boolean}, []ast.ASTHandle{z})
	addTransformRule(t, f, p.application, []ast.ASTHandle{q.application}, nil, "first-unbound-position")
	addTransformRule(t, f, p.application, []ast.ASTHandle{r.application}, nil, "second-unbound-position")
	addTransformRule(t, f, goal.application, []ast.ASTHandle{p.application}, nil, "specific-consumer")
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("rule-specific compression fixture rejected")
	}
	plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("unbound arguments were not compressed")
	}
	defer func() { coneMust(t, result.Close()) }()
	definitions := make(map[ast.ASTHandle]bool)
	count, err := result.Len()
	coneMust(t, err)
	for index := 0; index < count; index++ {
		rule, getErr := result.Rule(index)
		coneMust(t, getErr)
		head, getErr := rule.HeadDeclaration()
		if getErr == nil && head != goal.declaration && head != p.declaration {
			arity, arityErr := f.manager.GetArity(head)
			if arityErr == nil && arity == 2 {
				definitions[head] = true
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		coneMust(t, getErr)
	}
	if len(definitions) != 2 {
		t.Fatalf("rule-specific compressed definitions %d, want two", len(definitions))
	}
	goalRules, err := result.PredicateRules(goal.declaration)
	coneMust(t, err)
	defer func() {
		for _, rule := range goalRules {
			coneMust(t, rule.Close())
		}
	}()
	if len(goalRules) != 3 {
		t.Fatalf("goal rule alternatives %d, want original plus both decompressions", len(goalRules))
	}
	compressedTails := make(map[ast.ASTHandle]bool)
	for _, rule := range goalRules {
		declaration, getErr := rule.TailDeclaration(0)
		coneMust(t, getErr)
		if declaration != p.declaration {
			compressedTails[declaration] = true
		}
	}
	if len(compressedTails) != 2 {
		t.Fatalf("distinct decompressed tail predicates %d, want two", len(compressedTails))
	}
}

func TestUnboundCompressorAddsCompressedNegatedTail(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	p := declareTransformPredicate(t, f, "compress-negative-P", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{x, y})
	q := declareTransformPredicate(t, f, "compress-negative-Q", []ast.ASTHandle{boolean}, []ast.ASTHandle{y})
	e := declareTransformPredicate(t, f, "compress-negative-E", []ast.ASTHandle{boolean}, []ast.ASTHandle{y})
	goal := declareTransformPredicate(t, f, "compress-negative-Goal", []ast.ASTHandle{boolean}, []ast.ASTHandle{y})
	addTransformRule(t, f, p.application, []ast.ASTHandle{q.application}, nil, "negative-definition")
	addTransformRule(t, f, goal.application, []ast.ASTHandle{e.application, p.application}, []bool{false, true}, "negative-consumer")
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("negative-tail fixture rejected")
	}
	plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("unbound argument was not compressed")
	}
	defer func() { coneMust(t, result.Close()) }()
	compressed := plugin.CompressedPredicates()[p.declaration]
	if compressed.IsNull() {
		t.Fatal("missing compressed predicate")
	}
	goals, err := result.PredicateRules(goal.declaration)
	coneMust(t, err)
	defer func() {
		for _, rule := range goals {
			coneMust(t, rule.Close())
		}
	}()
	foundPair := false
	for _, rule := range goals {
		count, getErr := rule.TailSize()
		coneMust(t, getErr)
		var originalNegative, compressedNegative bool
		for index := 0; index < count; index++ {
			tail, negative, tailErr := rule.Tail(index)
			coneMust(t, tailErr)
			declaration, declErr := f.manager.GetDecl(tail)
			coneMust(t, declErr)
			originalNegative = originalNegative || (negative && declaration == p.declaration)
			compressedNegative = compressedNegative || (negative && declaration == compressed)
		}
		foundPair = foundPair || (originalNegative && compressedNegative)
	}
	if !foundPair {
		t.Fatal("compressed negative consumer did not retain both source predicates")
	}
}

func TestUnboundCompressorTransfersProofToDecompressionAlternative(t *testing.T) {
	f := newConeFixture(t)
	coneMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	p := declareTransformPredicate(t, f, "compress-proof-P", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{x, y})
	q := declareTransformPredicate(t, f, "compress-proof-Q", []ast.ASTHandle{boolean}, []ast.ASTHandle{y})
	goal := declareTransformPredicate(t, f, "compress-proof-Goal", []ast.ASTHandle{boolean}, []ast.ASTHandle{y})
	addTransformRule(t, f, p.application, []ast.ASTHandle{q.application}, nil, "proof-definition")
	goalRule, err := f.rules.NewRule(goal.application, []ast.ASTHandle{p.application}, nil, "proof-consumer")
	coneMust(t, err)
	formula, err := goalRule.ToFormula()
	coneMust(t, err)
	fact, err := f.manager.RefGet(formula)
	coneMust(t, err)
	proof, err := f.manager.MkProofAsserted(fact)
	coneMust(t, err)
	coneMust(t, goalRule.SetProof(proof))
	coneMust(t, f.set.Add(goalRule))
	coneMust(t, goalRule.Close())
	coneMust(t, f.manager.DelRef(&formula))
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("proof-compression fixture rejected")
	}
	plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("unbound argument was not compressed")
	}
	defer func() { coneMust(t, result.Close()) }()
	goals, err := result.PredicateRules(goal.declaration)
	coneMust(t, err)
	defer func() {
		for _, rule := range goals {
			coneMust(t, rule.Close())
		}
	}()
	if len(goals) != 2 {
		t.Fatalf("proof-preserving goal alternatives %d, want two", len(goals))
	}
	for _, rule := range goals {
		proof, found, proofErr := rule.Proof()
		if proofErr != nil || !found {
			t.Fatalf("decompression alternative lost its proof: found=%v err=%v", found, proofErr)
		}
		proved, proofErr := f.manager.GetProofFact(proof)
		coneMust(t, proofErr)
		formula, formulaErr := rule.ToFormula()
		coneMust(t, formulaErr)
		fact, formulaErr := f.manager.RefGet(formula)
		coneMust(t, formulaErr)
		equal, formulaErr := f.manager.AreEqual(proved, fact)
		coneMust(t, formulaErr)
		coneMust(t, f.manager.DelRef(&formula))
		if !equal {
			t.Fatal("decompression proof does not establish its rule")
		}
	}
	if mode, modeErr := f.manager.ProofMode(); modeErr != nil || mode != ast.ASTProofEnabled {
		t.Fatal("compressor changed ambient proof mode", mode, modeErr)
	}
}

func TestUnboundCompressorKeepsZeroArityCompressedFact(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	p := declareTransformPredicate(t, f, "compress-ground-P", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	goal := declareTransformPredicate(t, f, "compress-ground-Goal", nil, nil)
	addTransformRule(t, f, p.application, nil, nil, "unbound-fact")
	addTransformRule(t, f, goal.application, []ast.ASTHandle{p.application}, nil, "ground-consumer")
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("zero-arity fact fixture rejected")
	}
	plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("zero-arity compressed fact was not retained")
	}
	defer func() { coneMust(t, result.Close()) }()
	compressed := plugin.CompressedPredicates()[p.declaration]
	if compressed.IsNull() {
		t.Fatal("missing compressed predicate")
	}
	if arity, getErr := f.manager.GetArity(compressed); getErr != nil || arity != 0 {
		t.Fatalf("compressed fact arity %d, want 0: %v", arity, getErr)
	}
	facts, err := result.PredicateRules(compressed)
	coneMust(t, err)
	if len(facts) != 1 {
		t.Fatalf("compressed fact rules %d, want 1", len(facts))
	}
	tailCount, err := facts[0].TailSize()
	coneMust(t, err)
	if tailCount != 0 {
		t.Fatalf("compressed fact tails %d, want zero", tailCount)
	}
	coneMust(t, facts[0].Close())
}
