package duality_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityBackgroundConstrainsInterpolationAndSafeModel(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(owner.Close()) }()
	m, err := owner.Manager()
	must(err)
	config, err := owner.SolverTranslationConfig()
	must(err)
	backend, err := duality.NewBackend(m, config, true)
	must(err)
	defer func() { must(backend.Close()) }()

	var refs []ast.ASTRef
	defer func() {
		for i := range refs {
			must(m.DelRef(&refs[i]))
		}
	}()
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		ref, err := m.NewRef(h)
		must(err)
		refs = append(refs, ref)
		return h
	}

	seven := pin(m.ArithmeticNumeralRat(big.NewRat(7, 1), true))
	integer := pin(m.GetSort(seven))
	boolean := pin(m.MkBoolSort())
	kName, err := m.Symbols.Intern("background-interpolation-k")
	must(err)
	k := pin(m.MkConst(kName, integer))
	equality := pin(m.MkEq(k, seven))
	notEquality := pin(m.MkNot(equality))
	goalName, err := m.Symbols.Intern("background-interpolation-Goal")
	must(err)
	goalDecl := pin(m.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{}))
	goal := pin(m.MkApp(goalDecl, nil))
	rule := pin(m.MkImplies(notEquality, goal))

	withBackground, err := duality.PrepareQuery(
		m,
		[]ast.ASTHandle{rule},
		[]uint32{math.MaxUint32},
		goal,
		[]ast.ASTHandle{goalDecl},
		math.MaxUint32,
	)
	must(err)
	defer func() { must(withBackground.Close()) }()
	must(withBackground.AddBackground(equality))
	status, queryErr := backend.Query(withBackground)
	must(withBackground.Close())
	must(queryErr)
	if status != lbool.False {
		t.Fatal("background-constrained query", status, "want", lbool.False)
	}
	md, err := backend.GetModel()
	must(err)
	goalValue, err := md.Core().ConstantInterpretation(goalDecl)
	must(err)
	if !m.IsFalse(goalValue) {
		t.Fatal("safe model does not refute Goal", goalValue)
	}
	must(md.Close())

	withoutBackground, err := duality.PrepareQuery(
		m,
		[]ast.ASTHandle{rule},
		[]uint32{math.MaxUint32},
		goal,
		[]ast.ASTHandle{goalDecl},
		math.MaxUint32,
	)
	must(err)
	defer func() { must(withoutBackground.Close()) }()
	status, queryErr = backend.Query(withoutBackground)
	must(withoutBackground.Close())
	must(queryErr)
	if status != lbool.True {
		t.Fatal("unconstrained query", status, "want", lbool.True)
	}
}
