package ast

import "testing"

func TestASTSortParameterCountRetainsSourceSortInfoCount(t *testing.T) {
	m := astTestManager(t)
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	if n, e := m.GetSortNumParameters(b); e != nil || n != 0 {
		t.Fatal(n, e)
	}
	p, err := m.NewParameterInt(13)
	astTestMust(t, err)
	defer m.DelParameter(&p)
	name := astTestSymbol(t, m, "parameterized-sort")
	one, err := m.MkUninterpretedSort(name, []ASTParameter{p})
	astTestMust(t, err)
	if n, e := m.GetSortNumParameters(one); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	two, err := m.MkUninterpretedSort(name, []ASTParameter{p, p})
	astTestMust(t, err)
	if n, e := m.GetSortNumParameters(two); e != nil || n != 2 {
		t.Fatal(n, e)
	}
}
