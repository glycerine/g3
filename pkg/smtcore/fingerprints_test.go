package smtcore

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTFingerprintExactThenRootIdentityAndScopes(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2100, boolean, 0)
	_, bn := smtEnodeConstant(t, m, 2101, boolean, 0)
	bn.root = an
	bound, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	q1, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(2102)}, Body: bound})
	astTestMust(t, err)
	q2, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(2103)}, Body: bound})
	astTestMust(t, err)
	q1ID, err := m.GetASTID(q1)
	astTestMust(t, err)
	q2ID, err := m.GetASTID(q2)
	astTestMust(t, err)
	set := NewSMTFingerprintSet()
	first, err := set.Insert(q1, q1ID, []*SMTEnode{bn})
	astTestMust(t, err)
	if first == nil {
		t.Fatal("root-normalized insertion returned nil")
	}
	firstArg, err := first.Arg(0)
	astTestMust(t, err)
	if firstArg != an || first.Data() != q1 || first.DataHash() != q1ID {
		t.Fatal("root-normalized insertion", first)
	}
	if duplicate, err := set.Insert(q1, q1ID, []*SMTEnode{an}); err != nil || duplicate != nil {
		t.Fatal("duplicate normalized fingerprint", duplicate, err)
	}
	if found, err := set.Contains(q1, q1ID, []*SMTEnode{bn}); err != nil || !found {
		t.Fatal("two-stage contains", found, err)
	}
	astTestMust(t, set.PushScope())
	second, err := set.Insert(q2, q2ID, []*SMTEnode{an})
	astTestMust(t, err)
	if second == nil || set.Size() != 2 {
		t.Fatal("scoped insertion")
	}
	astTestMust(t, set.PopScopes(1))
	if set.Size() != 1 {
		t.Fatal("scope rollback size", set.Size())
	}
	if found, err := set.Contains(q2, q2ID, []*SMTEnode{an}); err != nil || found {
		t.Fatal("scope rollback membership", found, err)
	}
	var output bytes.Buffer
	astTestMust(t, set.Display(m, &output))
	aID, err := m.GetASTID(a)
	astTestMust(t, err)
	if !strings.HasPrefix(output.String(), "fingerprints:\n#") || !strings.Contains(output.String(), fmt.Sprintf(" #%d\n", aID)) || strings.Count(output.String(), "\n") != 2 {
		t.Fatal("fingerprint display", output.String())
	}
}

func TestSMTFingerprintResetAndGuards(t *testing.T) {
	set := NewSMTFingerprintSet()
	if _, err := set.Insert(ast.ASTHandle{}, 0, nil); err == nil {
		t.Fatal("null data accepted")
	}
	if err := set.PopScopes(0); err == nil {
		t.Fatal("zero-scope pop accepted")
	}
	set.Reset()
	if set.Size() != 0 {
		t.Fatal("reset retained fingerprints")
	}
}

func TestSMTFingerprintScratchReleasesRootsOnPopAndReset(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	body, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body,
	})
	astTestMust(t, err)
	const width = 8
	var args [width]*SMTEnode
	for i := range args {
		_, args[i] = smtEnodeConstant(t, m, uint32(2200+i), boolean, 0)
	}
	set := NewSMTFingerprintSet()
	base, err := set.Insert(q, 0, args[:1])
	astTestMust(t, err)
	astTestMust(t, set.PushScope())
	_, alias := smtEnodeConstant(t, m, 2300, boolean, 0)
	alias.root = args[width-1]
	input := args
	input[width-1] = alias
	wide, err := set.Insert(q, 0, input[:])
	astTestMust(t, err)
	astTestMust(t, set.PushScope())
	if found, err := set.Contains(q, 0, args[1:2]); err != nil || found {
		t.Fatal("narrow miss", found, err)
	}
	astTestMust(t, set.PopScopes(2))
	for i, node := range set.dummy[:cap(set.dummy)] {
		if node != nil {
			t.Fatalf("scratch slot %d retained a root after pop", i)
		}
	}
	if set.Size() != 1 || base == nil || wide == nil || wide.NumArgs() != width {
		t.Fatal("popping removed an outer fingerprint or changed the returned tuple")
	}
	for i := range args {
		if node, err := wide.Arg(uint32(i)); err != nil || node != args[i] {
			t.Fatal("saved argument", i, node, err)
		}
	}
	if found, err := set.Contains(q, 0, input[1:]); err != nil || found {
		t.Fatal("miss before reset", found, err)
	}
	set.Reset()
	if set.dummy != nil || set.Size() != 0 {
		t.Fatal("reset retained fingerprint storage")
	}
}

func TestSMTFingerprintOwnsBindingStorage(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	body, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body,
	})
	astTestMust(t, err)
	_, root := smtEnodeConstant(t, m, 2290, boolean, 0)
	_, alias := smtEnodeConstant(t, m, 2291, boolean, 0)
	alias.root = root
	set := NewSMTFingerprintSet()
	for _, width := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 16} {
		args := make([]*SMTEnode, width)
		for i := range args {
			args[i] = alias
		}
		fingerprint, err := set.Insert(q, uint32(width), args)
		astTestMust(t, err)
		if fingerprint == nil {
			t.Fatalf("missing fingerprint of width %d", width)
		}
		clear(args)
		clear(fingerprint.Args())
		set.Reset()
		if fingerprint.NumArgs() != uint32(width) || fingerprint.DataHash() != uint32(width) {
			t.Fatalf("lost width or data hash for width %d", width)
		}
		for i := range args {
			node, err := fingerprint.Arg(uint32(i))
			if err != nil || node != root {
				t.Fatalf("width %d argument %d lost its root", width, i)
			}
		}
	}
}

// Distinct enodes with the same owner hash to the same position. Mix these
// with ordinary tuples, grow the table inside nested scopes and reuse the
// vacated positions after each pop.
func TestSMTFingerprintCollisionsAcrossScopeGrowth(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	body, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body,
	})
	astTestMust(t, err)
	const total = 176
	var nodes [total]*SMTEnode
	var results [total]*SMTFingerprint
	for i := range nodes {
		name := uint32(3000)
		if i%3 == 0 {
			name += uint32(i)
		}
		_, nodes[i] = smtEnodeConstant(t, m, name, boolean, 0)
	}
	set := NewSMTFingerprintSet()
	insert := func(lo, hi int) {
		t.Helper()
		for i := lo; i < hi; i++ {
			f, err := set.Insert(q, uint32(i), nodes[i:i+1])
			astTestMust(t, err)
			if f == nil {
				t.Fatal("distinct colliding tuple rejected", i)
			}
			results[i] = f
		}
	}
	check := func(live int) {
		t.Helper()
		if set.Size() != uint32(live) {
			t.Fatal("wrong number of fingerprints", set.Size(), live)
		}
		for i := range nodes {
			found, err := set.Contains(q, 0, nodes[i:i+1])
			astTestMust(t, err)
			if found != (i < live) {
				t.Fatal("wrong membership", i, live)
			}
			if f := results[i]; f != nil {
				arg, err := f.Arg(0)
				astTestMust(t, err)
				if arg != nodes[i] || f.Data() != q || f.DataHash() != uint32(i) {
					t.Fatal("returned tuple changed", i)
				}
			}
		}
	}
	insert(0, 9)
	astTestMust(t, set.PushScope())
	insert(9, 57)
	astTestMust(t, set.PushScope())
	insert(57, total)
	check(total)
	astTestMust(t, set.PopScopes(1))
	check(57)
	astTestMust(t, set.PushScope())
	insert(57, 120)
	check(120)
	astTestMust(t, set.PopScopes(2))
	check(9)
	set.Reset()
	check(0)
}
