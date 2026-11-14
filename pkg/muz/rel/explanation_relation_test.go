package rel_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestExplanationRelationOwnsRowsAcrossResetAndClone(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	sort, err := m.MkDatalogRuleSort()
	must(err)
	name, err := m.Symbols.Intern("explanation-rule")
	must(err)
	value, err := m.MkConst(name, sort)
	must(err)
	input, err := m.NewRef(value)
	must(err)
	r, err := rel.NewExplanationRelation(m, []ast.ASTHandle{sort, sort})
	must(err)
	must(r.AddFact([]ast.ASTHandle{value, value}))
	must(m.DelRef(&input))
	if err = r.AddFact([]ast.ASTHandle{value, value}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("second explanation fact accepted", err)
	}
	copy, err := r.Clone()
	must(err)
	must(r.Reset())
	empty, err := r.Empty()
	must(err)
	if !empty {
		t.Fatal("reset relation is not empty")
	}
	staleClone, err := r.Clone()
	must(err)
	must(r.Close())
	must(copy.Close())
	if _, err = m.GetKind(value); err != nil {
		t.Fatal("empty clone lost its source-owned row", err)
	}
	must(staleClone.Close())
	if m.IsLiveHandle(value) {
		t.Fatal("closed relations kept the old row alive")
	}
}

func TestExplanationRelationComplementAndValidation(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	sort, err := m.MkDatalogRuleSort()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	if _, err = rel.NewExplanationRelation(m, []ast.ASTHandle{boolean}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-rule signature accepted", err)
	}
	r, err := rel.NewExplanationRelation(m, []ast.ASTHandle{sort, sort})
	must(err)
	defer func() { must(r.Close()) }()
	full, err := r.Complement()
	must(err)
	defer func() { must(full.Close()) }()
	data, err := full.Data()
	must(err)
	if len(data) != 2 || !data[0].IsNull() || !data[1].IsNull() {
		t.Fatal("complement lacks undefined columns", data)
	}
	defined, err := full.NoUndefined()
	must(err)
	if defined {
		t.Fatal("undefined complement was marked defined")
	}
	count, err := full.SizeEstimate()
	must(err)
	if count != 1 {
		t.Fatal("explanation size estimate", count)
	}
	empty, err := full.Complement()
	must(err)
	defer func() { must(empty.Close()) }()
	isEmpty, err := empty.Empty()
	must(err)
	if !isEmpty {
		t.Fatal("complement of nonempty relation is not empty")
	}
	defined, err = empty.NoUndefined()
	must(err)
	if !defined {
		t.Fatal("empty relation must not contain undefined values")
	}
	if err = r.AddFact([]ast.ASTHandle{boolean}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("wrong fact arity accepted", err)
	}
	zero, err := rel.NewExplanationRelation(m, nil)
	must(err)
	defer func() { must(zero.Close()) }()
	must(zero.AddFact(nil))
	zeroCopy, err := zero.Clone()
	must(err)
	defer func() { must(zeroCopy.Close()) }()
	isEmpty, err = zeroCopy.Empty()
	must(err)
	if isEmpty {
		t.Fatal("zero-column fact lost in clone")
	}
}
