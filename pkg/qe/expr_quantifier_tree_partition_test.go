package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExprQETreePartitionSeparatesIndependentVariables(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	p := f.constant("partition-p", f.boolean)
	q := f.constant("partition-q", f.boolean)
	r := f.constant("partition-free", f.boolean)
	truth := f.own(m.MkTrue())
	variables, err := newExprQEBranchVariables(m)
	fmMust(t, err)
	defer func() { fmMust(t, variables.Close()) }()
	fmMust(t, variables.add(q))
	fmMust(t, variables.add(p))
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	formula := f.own(m.MkAndArray([]ast.ASTHandle{p, q, r}))
	fmMust(t, root.initialize(formula))
	fmMust(t, variables.consume(root))
	partitioner, err := NewConjunctionPartitioner(m)
	fmMust(t, err)
	defer func() { fmMust(t, partitioner.Close()) }()
	part, err := partitionExprQETree(root, variables, partitioner)
	fmMust(t, err)
	defer func() { fmMust(t, part.Close()) }()
	closed, err := m.RefGet(part.closed)
	fmMust(t, err)
	mixed, err := m.RefGet(part.mixed)
	fmMust(t, err)
	if closed != r || !m.IsTrue(mixed) || !part.split || len(part.groups) != 2 || len(part.groups[0]) != 1 || len(part.groups[1]) != 1 || part.groups[0][0] == part.groups[1][0] {
		t.Fatal("independent open variables were not partitioned")
	}
	child, handled, err := closeExprQETreePartition(root, part, solver, nil, nil, f.r.config, false)
	fmMust(t, err)
	if handled || child != nil || len(root.children) != 0 {
		t.Fatal("split partition was prematurely closed")
	}
	lastIndex := part.groups[len(part.groups)-1][0]
	want, err := m.RefGet(root.variables[lastIndex])
	fmMust(t, err)
	group, ok, err := part.popVariables(root)
	fmMust(t, err)
	if !ok || len(group) != 1 || group[0] != want {
		t.Fatal("partition extraction did not start with the last group")
	}
	group, ok, err = part.popVariables(root)
	fmMust(t, err)
	if !ok || len(group) != 1 || group[0] == want {
		t.Fatal("second partition group was not extracted")
	}
	_, ok, err = part.popVariables(root)
	fmMust(t, err)
	if ok {
		t.Fatal("exhausted partition returned another group")
	}
	open, err := m.RefGet(part.open)
	fmMust(t, err)
	for _, variable := range []ast.ASTHandle{p, q} {
		contains, e := ast.NewContainsApp(m, variable)
		fmMust(t, e)
		found, e := contains.Check(open)
		fmMust(t, e)
		fmMust(t, contains.Close())
		if !found {
			t.Fatal("open partition omitted a variable")
		}
	}
}
