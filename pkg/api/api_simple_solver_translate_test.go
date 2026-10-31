package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverTranslateIndependentContext(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	astTestMust(t, source.SetLogic("QF_UF"))
	s, err := source.NewSimpleSolver()
	astTestMust(t, err)
	m, err := source.Manager()
	astTestMust(t, err)
	sort, err := source.BoolSort()
	astTestMust(t, err)
	p, err := source.Constant("translate-p", sort)
	astTestMust(t, err)
	pin, err := m.NewRef(p)
	astTestMust(t, err)
	astTestMust(t, s.Assert(p))
	params, err := source.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetUInt("random_seed", 17))
	astTestMust(t, params.SetBool("model", false))
	astTestMust(t, s.SetParameters(params))
	astTestMust(t, params.Close())
	copy, err := s.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, copy.Close()) }()
	targetGlobals, err := target.command.GlobalParameters()
	astTestMust(t, err)
	copiedParams, err := copy.solver.Params()
	astTestMust(t, err)
	key := "random_seed"
	seed, err := targetGlobals.Context.GetUIntName(copiedParams, &key, 0)
	astTestMust(t, err)
	if seed != 17 {
		t.Fatal("translated parameter", seed)
	}
	astTestMust(t, m.DelRef(&pin))
	astTestMust(t, s.Close())
	astTestMust(t, source.Close())
	status, err := copy.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("translated status", status)
	}
	if _, err = copy.Model(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("translated model setting", err)
	}
	assertions, err := copy.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("assertions", len(assertions))
	}
	targetManager, err := target.Manager()
	astTestMust(t, err)
	defer releaseAPIRefs(t, targetManager, assertions)
	h, err := targetManager.RefGet(assertions[0])
	astTestMust(t, err)
	neg, err := targetManager.MkNot(h)
	astTestMust(t, err)
	astTestMust(t, copy.Assert(neg))
	status, err = copy.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("independent translated assertion", status)
	}
}

func TestAPISimpleSolverTranslateRejectsActiveScope(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	astTestMust(t, s.Push())
	if _, err = s.Translate(target); err == nil {
		t.Fatal("translated active scope")
	}
	astTestMust(t, s.Pop(1))
	translated, err := s.Translate(target)
	astTestMust(t, err)
	astTestMust(t, translated.Close())
}

func TestAPISimpleSolverTranslateWithinContext(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	b := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("same-context-p", b))
	astTestMust(t, s.Assert(p))
	other, err := s.Translate(f.c)
	astTestMust(t, err)
	defer func() { astTestMust(t, other.Close()) }()
	notP := f.pin(f.m.MkNot(p))
	astTestMust(t, other.Assert(notP))
	status, err := other.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("translated solver", status)
	}
	status, err = s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("original solver changed", status)
	}
}

func TestAPISimpleSolverTranslateConfiguredTheoriesAfterSourceClose(t *testing.T) {
	for _, tc := range []struct{ name, logic, script string }{
		{"bitvector", "QF_BV", `(declare-const x (_ BitVec 4)) (assert (= (bvadd x #x1) #x3))`},
		{"linear arithmetic", "QF_LIA", `(declare-const x Int) (assert (> x 2)) (assert (< x 5))`},
		{"array", "QF_AX", `(declare-const a (Array Int Int)) (assert (= (select (store a 1 2) 1) 2))`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := NewSMTContext()
			astTestMust(t, err)
			target, err := NewSMTContext()
			astTestMust(t, err)
			defer func() { astTestMust(t, target.Close()) }()
			astTestMust(t, source.SetLogic(tc.logic))
			s, err := source.NewSimpleSolver()
			astTestMust(t, err)
			astTestMust(t, s.LoadSMT2String(tc.script))
			status, err := s.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("source", status)
			}
			translated, err := s.Translate(target)
			astTestMust(t, err)
			defer func() { astTestMust(t, translated.Close()) }()
			astTestMust(t, s.Close())
			astTestMust(t, source.Close())
			status, err = translated.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("translated", status)
			}
			astTestMust(t, translated.Push())
			m, err := target.Manager()
			astTestMust(t, err)
			no, err := m.MkFalse()
			astTestMust(t, err)
			astTestMust(t, translated.Assert(no))
			status, err = translated.Check()
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("child scope", status)
			}
			astTestMust(t, translated.Pop(1))
			status, err = translated.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("after pop", status)
			}
		})
	}
}

func TestAPISolverTranslateWithinContextFactories(t *testing.T) {
	for _, kind := range []string{"default", "logic", "tactic"} {
		t.Run(kind, func(t *testing.T) {
			f := newSMTAPIFixture(t, "QF_UF", nil)
			var s *APISimpleSolver
			var err error
			switch kind {
			case "default":
				s, err = f.c.NewSolver()
			case "logic":
				s, err = f.c.NewSolverForLogic("QF_UF")
			case "tactic":
				var tac *APITactic
				tac, err = f.c.Tactic("smt")
				astTestMust(t, err)
				s, err = f.c.NewSolverFromTactic(tac)
				astTestMust(t, tac.Close())
			}
			astTestMust(t, err)
			defer func() { astTestMust(t, s.Close()) }()
			b := f.pin(f.c.BoolSort())
			p := f.pin(f.c.Constant("factory-translate-p", b))
			astTestMust(t, s.Assert(p))
			copy, err := s.Translate(f.c)
			astTestMust(t, err)
			defer func() { astTestMust(t, copy.Close()) }()
			notP := f.pin(f.m.MkNot(p))
			astTestMust(t, copy.Assert(notP))
			status, err := copy.Check()
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("translated solver", status)
			}
			status, err = s.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("original solver", status)
			}
		})
	}
}

func TestAPIStrategicSolverCrossContextTranslation(t *testing.T) {
	for _, kind := range []string{"default", "logic", "incremental"} {
		t.Run(kind, func(t *testing.T) {
			source, err := NewSMTContext()
			astTestMust(t, err)
			target, err := NewSMTContext()
			astTestMust(t, err)
			defer func() { astTestMust(t, target.Close()) }()
			var s *APISimpleSolver
			if kind == "logic" {
				s, err = source.NewSolverForLogic("QF_UF")
			} else {
				s, err = source.NewSolver()
			}
			astTestMust(t, err)
			astTestMust(t, s.LoadSMT2String(`(declare-const p Bool) (assert p)`))
			params, err := source.NewParameterSet()
			astTestMust(t, err)
			astTestMust(t, params.SetUInt("random_seed", 17))
			astTestMust(t, s.SetParameters(params))
			astTestMust(t, params.Close())
			if kind == "incremental" {
				astTestMust(t, s.Push())
				astTestMust(t, s.Pop(1))
			}
			status, err := s.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("source", status)
			}
			translated, err := s.Translate(target)
			astTestMust(t, err)
			defer func() { astTestMust(t, translated.Close()) }()
			globals, err := target.command.GlobalParameters()
			astTestMust(t, err)
			p, err := translated.solver.Params()
			astTestMust(t, err)
			key := "random_seed"
			seed, err := globals.Context.GetUIntName(p, &key, 0)
			astTestMust(t, err)
			if seed != 17 {
				t.Fatal("translated parameter", seed)
			}
			astTestMust(t, s.Close())
			astTestMust(t, source.Close())
			status, err = translated.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("translated", status)
			}
			refs, err := translated.Assertions()
			astTestMust(t, err)
			if len(refs) != 1 {
				t.Fatal("assertions", len(refs))
			}
			m, err := target.Manager()
			astTestMust(t, err)
			defer releaseAPIRefs(t, m, refs)
			h, err := m.RefGet(refs[0])
			astTestMust(t, err)
			notH, err := m.MkNot(h)
			astTestMust(t, err)
			astTestMust(t, translated.Assert(notH))
			status, err = translated.Check()
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("contradictory translated solver", status)
			}
		})
	}
}

func TestAPIStrategicSolverCrossContextConfiguredTheories(t *testing.T) {
	for _, tc := range []struct{ logic, script string }{
		{"QF_BV", `(declare-const x (_ BitVec 4)) (assert (= (bvadd x #x1) #x3))`},
		{"QF_LIA", `(declare-const x Int) (assert (> x 2)) (assert (< x 5))`},
	} {
		for _, incremental := range []bool{false, true} {
			name := tc.logic
			if incremental {
				name += "-incremental"
			}
			t.Run(name, func(t *testing.T) {
				source, err := NewSMTContext()
				astTestMust(t, err)
				target, err := NewSMTContext()
				astTestMust(t, err)
				defer func() { astTestMust(t, target.Close()) }()
				s, err := source.NewSolverForLogic(tc.logic)
				astTestMust(t, err)
				astTestMust(t, s.LoadSMT2String(tc.script))
				if incremental {
					astTestMust(t, s.Push())
					astTestMust(t, s.Pop(1))
				}
				status, err := s.Check()
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("source", status)
				}
				translated, err := s.Translate(target)
				astTestMust(t, err)
				defer func() { astTestMust(t, translated.Close()) }()
				astTestMust(t, s.Close())
				astTestMust(t, source.Close())
				status, err = translated.Check()
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("translated", status)
				}
				astTestMust(t, translated.Push())
				m, err := target.Manager()
				astTestMust(t, err)
				no, err := m.MkFalse()
				astTestMust(t, err)
				astTestMust(t, translated.Assert(no))
				status, err = translated.Check()
				astTestMust(t, err)
				if status != lbool.False {
					t.Fatal("child scope", status)
				}
				astTestMust(t, translated.Pop(1))
				status, err = translated.Check()
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("after pop", status)
				}
			})
		}
	}
}

func TestAPIStrategicSolverCrossContextTranslationRejectsActiveScope(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	s, err := source.NewSolverForLogic("QF_UF")
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	astTestMust(t, s.Push())
	if _, err = s.Translate(target); err == nil {
		t.Fatal("translated active scope")
	}
	astTestMust(t, s.Pop(1))
	translated, err := s.Translate(target)
	astTestMust(t, err)
	astTestMust(t, translated.Close())
}

func TestAPIFiniteDomainCrossContextTranslation(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	s, err := source.NewSolverForLogic("QF_FD")
	astTestMust(t, err)
	astTestMust(t, s.LoadSMT2String(`(declare-const p Bool) (assert p) (assert (not p))`))
	status, err := s.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("source", status)
	}
	translated, err := s.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, translated.Close()) }()
	astTestMust(t, s.Close())
	astTestMust(t, source.Close())
	status, err = translated.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("translated", status)
	}
}

func TestAPIFiniteDomainTranslationDoesNotFlushPendingAssertions(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	s, err := source.NewSolverForLogic("QF_FD")
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	m, err := source.Manager()
	astTestMust(t, err)
	no, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, s.Assert(no))
	translated, err := s.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, translated.Close()) }()
	status, err := translated.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("pending assertion was copied", status)
	}
	status, err = s.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("source lost pending assertion", status)
	}
}

func TestAPIFiniteDomainCrossContextTranslationRejectsActiveScope(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	s, err := source.NewSolverForLogic("QF_FD")
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	astTestMust(t, s.Push())
	if _, err = s.Translate(target); err == nil {
		t.Fatal("translated active scope")
	}
	astTestMust(t, s.Pop(1))
	translated, err := s.Translate(target)
	astTestMust(t, err)
	astTestMust(t, translated.Close())
}

func TestAPISMTTacticSolverCrossContextTranslation(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	tac, err := source.Tactic("smt")
	astTestMust(t, err)
	s, err := source.NewSolverFromTactic(tac)
	astTestMust(t, err)
	astTestMust(t, tac.Close())
	astTestMust(t, s.LoadSMT2String(`(declare-const p Bool) (assert p)`))
	params, err := source.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetUInt("random_seed", 17))
	astTestMust(t, s.SetParameters(params))
	astTestMust(t, params.Close())
	status, err := s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("source", status)
	}
	translated, err := s.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, translated.Close()) }()
	astTestMust(t, s.Close())
	astTestMust(t, source.Close())
	status, err = translated.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("translated", status)
	}
	refs, err := translated.Assertions()
	astTestMust(t, err)
	if len(refs) != 1 {
		t.Fatal("assertions", len(refs))
	}
	m, err := target.Manager()
	astTestMust(t, err)
	defer releaseAPIRefs(t, m, refs)
	h, err := m.RefGet(refs[0])
	astTestMust(t, err)
	notH, err := m.MkNot(h)
	astTestMust(t, err)
	astTestMust(t, translated.Assert(notH))
	status, err = translated.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("contradiction", status)
	}
}

func TestAPIComposedTacticCrossContextTranslation(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	smt, err := source.Tactic("smt")
	astTestMust(t, err)
	params, err := source.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetUInt("random_seed", 17))
	configured, err := smt.UsingParameters(params)
	astTestMust(t, err)
	astTestMust(t, params.Close())
	skip, err := source.SkipTactic()
	astTestMust(t, err)
	sequence, err := skip.AndThen(configured)
	astTestMust(t, err)
	fail, err := source.FailTactic()
	astTestMust(t, err)
	fallback, err := fail.OrElse(sequence)
	astTestMust(t, err)
	repeated, err := fallback.Repeat(2)
	astTestMust(t, err)
	timed, err := repeated.TryFor(10000)
	astTestMust(t, err)
	s, err := source.NewSolverFromTactic(timed)
	astTestMust(t, err)
	for _, value := range []*APITactic{timed, repeated, fallback, fail, sequence, skip, configured, smt} {
		astTestMust(t, value.Close())
	}
	m, err := source.Manager()
	astTestMust(t, err)
	no, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, s.Assert(no))
	translated, err := s.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, translated.Close()) }()
	astTestMust(t, s.Close())
	astTestMust(t, source.Close())
	status, err := translated.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("translated tactic", status)
	}
}

func TestAPIUnsupportedTacticLeafCrossContextTranslation(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	smt, err := source.Tactic("smt")
	astTestMust(t, err)
	simplify, err := source.Tactic("propagate-ineqs")
	astTestMust(t, err)
	combined, err := smt.AndThen(simplify)
	astTestMust(t, err)
	s, err := source.NewSolverFromTactic(combined)
	astTestMust(t, err)
	astTestMust(t, combined.Close())
	astTestMust(t, simplify.Close())
	astTestMust(t, smt.Close())
	defer func() { astTestMust(t, s.Close()) }()
	if _, err = s.Translate(target); err == nil {
		t.Fatal("expected unsupported tactic leaf")
	}
	status, err := s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("source after rejection", status)
	}
}

func TestAPISMTTacticSolverCrossContextTranslationRejectsActiveScope(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	tac, err := source.Tactic("smt")
	astTestMust(t, err)
	s, err := source.NewSolverFromTactic(tac)
	astTestMust(t, err)
	astTestMust(t, tac.Close())
	defer func() { astTestMust(t, s.Close()) }()
	astTestMust(t, s.Push())
	if _, err = s.Translate(target); err == nil {
		t.Fatal("translated active scope")
	}
	astTestMust(t, s.Pop(1))
	translated, err := s.Translate(target)
	astTestMust(t, err)
	astTestMust(t, translated.Close())
}

func TestAPISimplifySMTTacticCrossContextTranslation(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	simplify, err := source.Tactic("simplify")
	astTestMust(t, err)
	params, err := source.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetBool("elim_and", true))
	configured, err := simplify.UsingParameters(params)
	astTestMust(t, err)
	astTestMust(t, params.Close())
	smt, err := source.Tactic("smt")
	astTestMust(t, err)
	sequence, err := configured.AndThen(smt)
	astTestMust(t, err)
	s, err := source.NewSolverFromTactic(sequence)
	astTestMust(t, err)
	for _, tac := range []*APITactic{sequence, smt, configured, simplify} {
		astTestMust(t, tac.Close())
	}
	astTestMust(t, s.LoadSMT2String(`(declare-const p Bool) (assert p) (assert (not p))`))
	translated, err := s.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, translated.Close()) }()
	astTestMust(t, s.Close())
	astTestMust(t, source.Close())
	status, err := translated.Check()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("translated simplify-smt", status)
	}
}

func TestAPICoreTacticSolversCrossContextTranslation(t *testing.T) {
	for _, name := range []string{"propagate-values", "solve-eqs", "elim-uncnstr", "elim-term-ite", "tseitin-cnf-core", "ctx-simplify", "bit-blast", "nnf", "snf", "der", "symmetry-reduce", "aig", "injectivity", "add-bounds", "occf", "distribute-forall", "reduce-args", "factor", "lia2pb", "eq2bv", "card2bv", "pb2bv", "blast-term-ite", "cofactor-term-ite", "pb-preprocess", "unit-subsume-simplify"} {
		t.Run(name, func(t *testing.T) {
			source, err := NewSMTContext()
			astTestMust(t, err)
			target, err := NewSMTContext()
			astTestMust(t, err)
			defer func() { astTestMust(t, target.Close()) }()
			prefix, err := source.Tactic(name)
			astTestMust(t, err)
			smt, err := source.Tactic("smt")
			astTestMust(t, err)
			sequence, err := prefix.AndThen(smt)
			astTestMust(t, err)
			s, err := source.NewSolverFromTactic(sequence)
			astTestMust(t, err)
			for _, value := range []*APITactic{sequence, smt, prefix} {
				astTestMust(t, value.Close())
			}
			astTestMust(t, s.LoadSMT2String(`(declare-const p Bool) (assert p) (assert (not p))`))
			translated, err := s.Translate(target)
			astTestMust(t, err)
			defer func() { astTestMust(t, translated.Close()) }()
			astTestMust(t, s.Close())
			astTestMust(t, source.Close())
			status, err := translated.Check()
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("translated tactic", status)
			}
		})
	}
}

func TestAPICoreTacticTranslatedModelsUseTargetServices(t *testing.T) {
	for _, name := range []string{"propagate-values", "solve-eqs", "elim-uncnstr", "elim-term-ite", "tseitin-cnf-core", "ctx-simplify", "bit-blast", "nnf", "snf", "der", "symmetry-reduce", "aig", "injectivity", "add-bounds", "occf", "distribute-forall", "reduce-args", "factor", "lia2pb", "eq2bv", "card2bv", "pb2bv", "blast-term-ite", "cofactor-term-ite", "pb-preprocess", "unit-subsume-simplify"} {
		t.Run(name, func(t *testing.T) {
			source, err := NewSMTContext()
			astTestMust(t, err)
			target, err := NewSMTContext()
			astTestMust(t, err)
			defer func() { astTestMust(t, target.Close()) }()
			prefix, err := source.Tactic(name)
			astTestMust(t, err)
			smt, err := source.Tactic("smt")
			astTestMust(t, err)
			sequence, err := prefix.AndThen(smt)
			astTestMust(t, err)
			s, err := source.NewSolverFromTactic(sequence)
			astTestMust(t, err)
			for _, value := range []*APITactic{sequence, smt, prefix} {
				astTestMust(t, value.Close())
			}
			formula := `(declare-const p Bool) (assert p)`
			if name == "bit-blast" {
				formula = `(declare-const x (_ BitVec 8)) (assert (= (bvadd x #x01) #x05))`
			} else if name == "add-bounds" {
				formula = `(declare-const x Int) (assert (and (>= x 2) (<= x 8)))`
			} else if name == "reduce-args" {
				formula = `(declare-fun f ((_ BitVec 8)) Int) (assert (= (f #x01) 3))`
			} else if name == "factor" {
				formula = `(declare-const x Real) (assert (= (* (- x 1) (+ x 1)) 0))`
			} else if name == "lia2pb" || name == "eq2bv" {
				formula = `(declare-const x Int) (assert (and (>= x 0) (<= x 3) (= x 2)))`
			} else if name == "card2bv" {
				formula = `(declare-const p Bool) (declare-const q Bool) (assert ((_ at-most 1) p q))`
			} else if name == "pb2bv" {
				formula = `(declare-const x Int) (assert (>= x 0)) (assert (<= x 1)) (assert (= x 1))`
			} else if name == "blast-term-ite" {
				formula = `(declare-const p Bool) (declare-fun f (Int) Int) (assert (= (f (ite p 1 2)) 3))`
			} else if name == "cofactor-term-ite" {
				formula = `(declare-const p Bool) (assert (= (+ (ite p 1 2) 3) 4))`
			} else if name == "pb-preprocess" {
				formula = `(declare-const p Bool) (declare-const q Bool) (assert ((_ at-least 1) p q))`
			}
			astTestMust(t, s.LoadSMT2String(formula))
			translated, err := s.Translate(target)
			astTestMust(t, err)
			defer func() { astTestMust(t, translated.Close()) }()
			astTestMust(t, s.Close())
			astTestMust(t, source.Close())
			status, err := translated.Check()
			astTestMust(t, err)
			if status != lbool.True {
				reason, _ := translated.ReasonUnknown()
				t.Fatal("translated status", status, reason)
			}
			refs, err := translated.Assertions()
			astTestMust(t, err)
			if len(refs) == 0 {
				t.Fatal("missing assertions")
			}
			m, err := target.Manager()
			astTestMust(t, err)
			defer releaseAPIRefs(t, m, refs)
			md, err := translated.Model()
			astTestMust(t, err)
			defer func() { astTestMust(t, md.Close()) }()
			for _, ref := range refs {
				p, err := m.RefGet(ref)
				astTestMust(t, err)
				v, err := md.Evaluate(p, true)
				astTestMust(t, err)
				h, err := m.RefGet(v)
				astTestMust(t, err)
				if !m.IsTrue(h) {
					t.Fatal("model fails assertion", h)
				}
				astTestMust(t, m.DelRef(&v))
			}
		})
	}
}

func TestAPIConditionalTacticCrossContextTranslation(t *testing.T) {
	for _, invert := range []bool{false, true} {
		t.Run(fmt.Sprintf("invert=%v", invert), func(t *testing.T) {
			source, err := NewSMTContext()
			astTestMust(t, err)
			target, err := NewSMTContext()
			astTestMust(t, err)
			defer func() { astTestMust(t, target.Close()) }()
			size, err := source.GoalProbe(APIGoalProbeSize)
			astTestMust(t, err)
			zero, err := source.ConstantProbe(0)
			astTestMust(t, err)
			positive, err := size.Compare(APIProbeGreater, zero)
			astTestMust(t, err)
			probe, err := positive.Not()
			astTestMust(t, err)
			smt, err := source.Tactic("smt")
			astTestMust(t, err)
			fail, err := source.FailTactic()
			astTestMust(t, err)
			var conditional *APITactic
			if invert {
				conditional, err = probe.Conditional(fail, smt)
			} else {
				conditional, err = positive.Conditional(smt, fail)
			}
			astTestMust(t, err)
			s, err := source.NewSolverFromTactic(conditional)
			astTestMust(t, err)
			for _, value := range []*APITactic{conditional, smt, fail} {
				astTestMust(t, value.Close())
			}
			for _, value := range []*APIProbe{probe, positive, zero, size} {
				astTestMust(t, value.Close())
			}
			astTestMust(t, s.LoadSMT2String(`(declare-const p Bool) (assert p)`))
			translated, err := s.Translate(target)
			astTestMust(t, err)
			defer func() { astTestMust(t, translated.Close()) }()
			astTestMust(t, s.Close())
			astTestMust(t, source.Close())
			status, err := translated.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("translated conditional", status)
			}
		})
	}
}

func TestAPISubsystemProbeTacticCrossContextTranslation(t *testing.T) {
	for _, name := range []string{"is-qflia", "arith-max-deg", "is-unbounded", "is-pb", "is-quasi-pb", "ackr-bound-probe", "is-qfbv-eq", "is-qffp", "is-qffpbv", "is-qffplra"} {
		t.Run(name, func(t *testing.T) {
			source, err := NewSMTContext()
			astTestMust(t, err)
			target, err := NewSMTContext()
			astTestMust(t, err)
			defer func() { astTestMust(t, target.Close()) }()
			p, err := source.NamedProbe(name)
			astTestMust(t, err)
			one, err := source.ConstantProbe(1)
			astTestMust(t, err)
			combined, err := p.And(one)
			astTestMust(t, err)
			smt, err := source.Tactic("smt")
			astTestMust(t, err)
			condition, err := combined.Conditional(smt, smt)
			astTestMust(t, err)
			s, err := source.NewSolverFromTactic(condition)
			astTestMust(t, err)
			for _, v := range []*APITactic{condition, smt} {
				astTestMust(t, v.Close())
			}
			for _, v := range []*APIProbe{combined, one, p} {
				astTestMust(t, v.Close())
			}
			astTestMust(t, s.LoadSMT2String(`(declare-const x Int) (assert (> x 1))`))
			translated, err := s.Translate(target)
			astTestMust(t, err)
			defer func() { astTestMust(t, translated.Close()) }()
			astTestMust(t, s.Close())
			astTestMust(t, source.Close())
			status, err := translated.Check()
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("translated arithmetic probe", status)
			}
		})
	}
}

func TestAPISolverTranslationUsesExplicitParameterSnapshot(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	third, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, third.Close()) }()
	s, err := source.NewSimpleSolver()
	astTestMust(t, err)
	p, err := source.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, p.SetUInt("random_seed", 17))
	astTestMust(t, s.SetParameters(p))
	astTestMust(t, p.SetUInt("random_seed", 99))
	astTestMust(t, s.solver.UpdateParams(&p.value))
	astTestMust(t, p.Close())
	translated, err := s.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, translated.Close()) }()
	astTestMust(t, s.Close())
	astTestMust(t, source.Close())
	globals, err := target.command.GlobalParameters()
	astTestMust(t, err)
	backend, err := translated.solver.Params()
	astTestMust(t, err)
	name := "random_seed"
	seed, err := globals.Context.GetUIntName(backend, &name, 0)
	astTestMust(t, err)
	if seed != 17 {
		t.Fatalf("translated backend seed = %d, want explicit 17", seed)
	}
	if translated.explicitReady {
		t.Fatal("translated API solver inherited explicit parameter set")
	}
	again, err := translated.Translate(third)
	astTestMust(t, err)
	defer func() { astTestMust(t, again.Close()) }()
	globals, err = third.command.GlobalParameters()
	astTestMust(t, err)
	backend, err = again.solver.Params()
	astTestMust(t, err)
	seed, err = globals.Context.GetUIntName(backend, &name, 0)
	astTestMust(t, err)
	if seed != 0 {
		t.Fatalf("second translation seed = %d, want default 0", seed)
	}
}

func TestAPITacticSolverTranslateCollectStatisticsAfterSourceClose(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	astTestMust(t, source.SetLogic("QF_UF"))
	stats, err := source.Tactic("collect-statistics")
	astTestMust(t, err)
	smt, err := source.Tactic("smt")
	astTestMust(t, err)
	combined, err := stats.AndThen(smt)
	astTestMust(t, err)
	solver, err := source.NewSolverFromTactic(combined)
	astTestMust(t, err)
	for _, owner := range []*APITactic{combined, smt, stats} {
		astTestMust(t, owner.Close())
	}
	astTestMust(t, solver.LoadSMT2String("(declare-const p Bool) (assert p)"))
	translated, err := solver.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, translated.Close()) }()
	astTestMust(t, solver.Close())
	astTestMust(t, source.Close())
	status, err := translated.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("translated solver", status)
	}
}

func TestAPITacticSolverTranslateSplitClauseAfterSourceClose(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	astTestMust(t, source.SetLogic("QF_UF"))
	split, err := source.Tactic("split-clause")
	astTestMust(t, err)
	smt, err := source.Tactic("smt")
	astTestMust(t, err)
	combined, err := split.AndThen(smt)
	astTestMust(t, err)
	solver, err := source.NewSolverFromTactic(combined)
	astTestMust(t, err)
	for _, owner := range []*APITactic{combined, smt, split} {
		astTestMust(t, owner.Close())
	}
	astTestMust(t, solver.LoadSMT2String("(declare-const p Bool) (declare-const q Bool) (assert (or p q))"))
	translated, err := solver.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, translated.Close()) }()
	astTestMust(t, solver.Close())
	astTestMust(t, source.Close())
	status, err := translated.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("translated solver", status)
	}
}

func TestAPITacticSolverTranslateDegreeShiftAfterSourceClose(t *testing.T) {
	source, err := NewSMTContext()
	astTestMust(t, err)
	target, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	astTestMust(t, source.SetLogic("QF_NRA"))
	shift, err := source.Tactic("degree-shift")
	astTestMust(t, err)
	smt, err := source.Tactic("smt")
	astTestMust(t, err)
	combined, err := shift.AndThen(smt)
	astTestMust(t, err)
	solver, err := source.NewSolverFromTactic(combined)
	astTestMust(t, err)
	for _, owner := range []*APITactic{combined, smt, shift} {
		astTestMust(t, owner.Close())
	}
	astTestMust(t, solver.LoadSMT2String("(declare-const x Real) (assert (= (* x x) 4)) (assert (> x 0))"))
	translated, err := solver.Translate(target)
	astTestMust(t, err)
	defer func() { astTestMust(t, translated.Close()) }()
	astTestMust(t, solver.Close())
	astTestMust(t, source.Close())
	status, err := translated.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("translated solver", status)
	}
	model, err := translated.Model()
	astTestMust(t, err)
	defer func() { astTestMust(t, model.Close()) }()
	manager, err := target.Manager()
	astTestMust(t, err)
	assertions, err := translated.Assertions()
	astTestMust(t, err)
	defer releaseAPIRefs(t, manager, assertions)
	if len(assertions) != 2 {
		t.Fatalf("translated assertion count = %d, want 2", len(assertions))
	}
	for _, assertion := range assertions {
		h, err := manager.RefGet(assertion)
		astTestMust(t, err)
		value, err := model.Evaluate(h, true)
		astTestMust(t, err)
		actual, err := manager.RefGet(value)
		astTestMust(t, err)
		if !manager.IsTrue(actual) {
			t.Fatal("translated model violates original assertion")
		}
		astTestMust(t, manager.DelRef(&value))
	}
}
