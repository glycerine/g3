package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTQuantifierCandidateRegistrationLifetime(t *testing.T) {
	f := newQuantifierCandidateFixture(t, 4)
	q := f.manager
	for scope := 0; scope < 2; scope++ {
		astTestMust(t, q.state.PushScope())
		astTestMust(t, q.state.Internalize(f.quantifier, true))
		stat, err := q.registry.Stat(f.quantifier)
		astTestMust(t, err)
		if stat.candidateBody != nil {
			t.Fatal("new registration already has candidate syntax")
		}
		for i := 0; i < 32; i++ {
			n := i % len(f.bindings)
			p, err := q.builder.Prepare(SMTQIQueueEntry{Fingerprint: &SMTFingerprint{
				data: f.quantifier, args: f.bindings[n : n+1],
			}})
			astTestMust(t, err)
			if got, want := p.AlreadySatisfied(), n < len(f.bindings)-1; got != want {
				t.Fatalf("scope %d binding %d: satisfied %v, want %v", scope, n, got, want)
			}
			astTestMust(t, p.Close())
		}
		if !smtcoreSourceDebug && !ast.ASTValidateHandles && stat.candidateBody == nil {
			t.Fatal("repeated candidates did not prepare the registered body")
		}
		for _, bad := range []*SMTEnode{nil, {manager: astTestManager(t)}} {
			_, err := q.builder.Prepare(SMTQIQueueEntry{Fingerprint: &SMTFingerprint{
				data: f.quantifier, args: []*SMTEnode{bad},
			}})
			if !errors.Is(err, ast.ErrASTDomain) {
				t.Fatalf("invalid binding gave %v", err)
			}
		}
		astTestMust(t, q.state.PopScopes(1))
		if stat.candidateBody != nil {
			t.Fatal("popped registration retained its candidate syntax")
		}
	}
	// Reset can retire a standalone registration while its stats remain held.
	astTestMust(t, q.registry.Add(f.quantifier, 0))
	stat, err := q.registry.Stat(f.quantifier)
	astTestMust(t, err)
	for i := 0; i < 8; i++ {
		got, err := q.checker.run(f.body, true, f.bindings[:1], stat)
		if err != nil || !got {
			t.Fatal(got, err)
		}
	}
	astTestMust(t, q.registry.Reset())
	if stat.candidateBody != nil {
		t.Fatal("reset retained the candidate syntax")
	}
}

func TestSMTQuantifierCandidateUsesCurrentSolverState(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(symbol.Numerical(301), boolean)
	astTestMust(t, err)
	b, err := m.MkConst(symbol.Numerical(302), boolean)
	astTestMust(t, err)
	body, err := m.MkOr(a, b)
	astTestMust(t, err)
	var checkers [2]*SMTChecker
	for i := range checkers {
		ctx, err := NewSMTCheckerContext(m)
		astTestMust(t, err)
		checkers[i], err = NewSMTChecker(ctx)
		astTestMust(t, err)
		astTestMust(t, ctx.SetRelevant(a, true))
		astTestMust(t, ctx.SetRelevant(b, true))
		astTestMust(t, ctx.SetAssignment(b, lbool.False))
	}
	stat := &SMTQuantifierStat{}
	for round := 0; round < 24; round++ {
		for i, c := range checkers {
			values := [...]lbool.LBool{lbool.False, lbool.True, lbool.Undef}
			value := values[(round+i)%len(values)]
			relevant := (round+i)%4 != 0
			astTestMust(t, c.context.SetRelevant(a, relevant))
			astTestMust(t, c.context.SetAssignment(a, value))
		}
		for i, c := range checkers {
			want, err := c.IsSatisfiable(body, nil)
			astTestMust(t, err)
			got, err := c.run(body, true, nil, stat)
			if err != nil || got != want {
				t.Fatalf("checker %d round %d got %v, %v; want %v", i, round, got, err, want)
			}
		}
	}
}

// IFF can test its left operand twice. Only the native refcount rule decides
// whether those tests repeat a negative equality's temporary AST update.
func TestSMTQuantifierCandidateRepeatedDisequality(t *testing.T) {
	m := astTestManager(t)
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	checker, err := NewSMTChecker(ctx)
	astTestMust(t, err)
	var sorts, equal, negative [2]ast.ASTHandle
	var bindings [4]*SMTEnode
	for i := range sorts {
		sorts[i], err = m.MkUninterpretedSort(symbol.Numerical(uint32(310+i)), nil)
		astTestMust(t, err)
		var vars [2]ast.ASTHandle
		for j := range vars {
			k := 2*i + j
			owner, node := smtEnodeConstant(t, m, uint32(320+k), sorts[i], 0)
			astTestMust(t, m.IncRef(owner))
			t.Cleanup(func() { astTestMust(t, m.DecRef(owner)) })
			bindings[k] = node
			astTestMust(t, ctx.RegisterEnode(node, true))
			vars[j], err = m.MkVar(uint32(3-k), sorts[i])
			astTestMust(t, err)
		}
		equal[i], err = m.MkEq(vars[0], vars[1])
		astTestMust(t, err)
		negative[i], err = m.MkNot(equal[i])
		astTestMust(t, err)
	}
	astTestMust(t, ctx.SetDisequal(bindings[0], bindings[1], true))
	astTestMust(t, m.IncRef(equal[1]))
	t.Cleanup(func() { astTestMust(t, m.DecRef(equal[1])) })
	all, err := m.MkAnd(negative[0], negative[1])
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	unknown, err := m.MkConst(symbol.Numerical(340), boolean)
	astTestMust(t, err)
	inner, err := m.MkIff(all, unknown)
	astTestMust(t, err)
	body, err := m.MkIff(inner, unknown)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(body))
	t.Cleanup(func() { astTestMust(t, m.DecRef(body)) })
	t.Cleanup(func() { astTestMust(t, ctx.releaseDiseqTmp()) })
	stat := &SMTQuantifierStat{}
	for round := 0; round < 16; round++ {
		extra := round%2 == 1
		if extra {
			astTestMust(t, m.IncRef(equal[0]))
		}
		for _, compiled := range []bool{false, true} {
			astTestMust(t, ctx.releaseDiseqTmp())
			var got bool
			if compiled {
				got, err = checker.run(body, true, bindings[:], stat)
			} else {
				got, err = checker.IsSatisfiable(body, bindings[:])
			}
			if err != nil || got || !ctx.diseqTmpHeld {
				t.Fatalf("round %d prepared %v: %v, %v, held %v", round, compiled, got, err, ctx.diseqTmpHeld)
			}
			h, err := m.RefGet(ctx.diseqTmp)
			astTestMust(t, err)
			arg, err := m.GetArg(h, 0)
			astTestMust(t, err)
			sort, err := m.GetSort(arg)
			astTestMust(t, err)
			want := sorts[0]
			if extra {
				want = sorts[1]
			}
			if sort != want {
				t.Fatalf("round %d prepared %v: held sort %v, want %v", round, compiled, sort, want)
			}
		}
		if extra {
			astTestMust(t, m.DecRef(equal[0]))
		}
	}
}

// The lazy checker can discharge this OR on its first operand.
func TestSMTQuantifierCandidateLargeBodyFallback(t *testing.T) {
	m := astTestManager(t)
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	checker, err := NewSMTChecker(ctx)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	args := make([]ast.ASTHandle, 300)
	for i := range args {
		args[i], err = m.MkConst(symbol.Numerical(uint32(400+i)), boolean)
		astTestMust(t, err)
	}
	body, err := m.MkOrArray(args)
	astTestMust(t, err)
	astTestMust(t, ctx.SetAssignment(args[0], lbool.True))
	astTestMust(t, ctx.SetRelevant(args[0], true))
	stat := &SMTQuantifierStat{}
	for i := 0; i < 20; i++ {
		got, err := checker.run(body, true, nil, stat)
		if err != nil || !got {
			t.Fatal(got, err)
		}
	}
	if stat.candidateBody != nil {
		t.Fatal("oversized quantifier body was prepared")
	}
}

func TestSMTPreparedCandidateRejectsRetiredBody(t *testing.T) {
	m := astTestManager(t)
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	checker, err := NewSMTChecker(ctx)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	body, err := m.MkConst(symbol.Numerical(9101), boolean)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(body))
	p := prepareCandidateBody(m, body, 1)
	if p == nil || p.memoSize != 0 {
		t.Fatal("expected a prepared body with no memo slots")
	}
	astTestMust(t, m.DecRef(body))
	want := m.ValidateHandle(body)
	if want == nil {
		t.Fatal("body is still live")
	}
	if _, err := checker.runCandidate(p, true); !errors.Is(err, want) {
		t.Fatalf("retired body returned %v, want %v", err, want)
	}
}
