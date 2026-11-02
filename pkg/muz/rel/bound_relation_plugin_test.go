package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBoundRelationPluginCapabilityAndConstruction(t *testing.T) {
	manager := astTestManager(t)
	plugin, err := NewBoundRelationPlugin(manager)
	astTestMust(t, err)
	if plugin.Name() != "bound_relation" {
		t.Fatal("plugin name", plugin.Name())
	}
	arithmetic, err := manager.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := manager.MkFamilySort(arithmetic, 0, nil)
	astTestMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	for _, tc := range []struct {
		name  string
		sorts []ast.ASTHandle
		want  bool
	}{
		{name: "nullary", want: true},
		{name: "integer", sorts: []ast.ASTHandle{integer}, want: true},
		{name: "real", sorts: []ast.ASTHandle{real}, want: true},
		{name: "mixed-arithmetic", sorts: []ast.ASTHandle{integer, real}, want: true},
		{name: "Boolean", sorts: []ast.ASTHandle{boolean}},
		{name: "not-a-sort", sorts: []ast.ASTHandle{ast.ASTHandle{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := plugin.CanHandleSignature(tc.sorts)
			if tc.name == "not-a-sort" {
				if err == nil {
					t.Fatal("invalid handle accepted")
				}
				return
			}
			astTestMust(t, err)
			if got != tc.want {
				t.Fatalf("capability=%v, want %v", got, tc.want)
			}
		})
	}

	empty, err := plugin.NewEmpty([]ast.ASTHandle{integer, real})
	astTestMust(t, err)
	if !empty.Empty() || len(empty.columns) != 2 {
		t.Fatal("empty construction", empty.Empty(), len(empty.columns))
	}
	full, err := plugin.NewFull([]ast.ASTHandle{integer, real})
	astTestMust(t, err)
	if full.Empty() || len(full.columns) != 2 {
		t.Fatal("full construction", full.Empty(), len(full.columns))
	}
	if err = full.AddLess(0, 1); err != nil {
		t.Fatal("constructed relation does not own independent state", err)
	}
	if strict, _, err := empty.Bounds(0); err != nil || len(strict) != 0 {
		t.Fatal("constructed relations share column state", strict, err)
	}
	if _, err = plugin.NewFull([]ast.ASTHandle{boolean}); err == nil {
		t.Fatal("unsupported signature constructed")
	}
}

func TestBoundRelationPluginRejectsNilManager(t *testing.T) {
	if _, err := NewBoundRelationPlugin(nil); err == nil {
		t.Fatal("nil manager accepted")
	}
	var plugin *BoundRelationPlugin
	if _, err := plugin.CanHandleSignature(nil); err == nil {
		t.Fatal("nil plugin accepted")
	}
}

func TestBoundRelationPluginJoinDispatch(t *testing.T) {
	manager := astTestManager(t)
	plugin, err := NewBoundRelationPlugin(manager)
	astTestMust(t, err)
	left, right := NewBoundRelation(2, false), NewBoundRelation(2, false)
	astTestMust(t, left.AddLess(0, 1))
	astTestMust(t, right.AddLessEqual(0, 1))
	joined, err := plugin.Join(left, right, []uint32{0}, []uint32{1})
	astTestMust(t, err)
	if joined.find(0) != joined.find(3) {
		t.Fatal("join dispatch did not equate requested columns")
	}
	strict, _, err := joined.Bounds(0)
	astTestMust(t, err)
	if len(strict) != 1 || strict[0] != 1 {
		t.Fatal("join dispatch lost left bounds", strict)
	}
	// The result must not alias either operand's owned maps.
	astTestMust(t, joined.AddLess(2, 3))
	strict, weak, err := right.Bounds(0)
	astTestMust(t, err)
	if len(strict) != 0 || len(weak) != 1 || weak[0] != 1 {
		t.Fatal("join result mutated right operand", strict, weak)
	}
	if _, err = plugin.Join(left, right, []uint32{2}, []uint32{0}); err == nil {
		t.Fatal("invalid join column accepted")
	}
	if _, err = plugin.Join(left, right, []uint32{0}, nil); err == nil {
		t.Fatal("mismatched join columns accepted")
	}
	var nilPlugin *BoundRelationPlugin
	if _, err = nilPlugin.Join(left, right, nil, nil); err == nil {
		t.Fatal("nil plugin join accepted")
	}
}

func TestBoundRelationPluginProjectDispatch(t *testing.T) {
	manager := astTestManager(t)
	plugin, err := NewBoundRelationPlugin(manager)
	astTestMust(t, err)
	source := NewBoundRelation(4, false)
	astTestMust(t, source.AddLess(0, 3))
	astTestMust(t, source.AddLessEqual(0, 2))
	projected, err := plugin.Project(source, []uint32{1})
	astTestMust(t, err)
	strict, weak, err := projected.Bounds(0)
	astTestMust(t, err)
	if len(strict) != 1 || strict[0] != 2 || len(weak) != 1 || weak[0] != 1 {
		t.Fatal("project dispatch did not renumber bounds", strict, weak)
	}
	// Projection returns independently owned bound maps.
	astTestMust(t, projected.AddLess(1, 2))
	strict, weak, err = source.Bounds(2)
	astTestMust(t, err)
	if len(strict) != 0 || len(weak) != 0 {
		t.Fatal("projected result mutated source", strict, weak)
	}
	if _, err = plugin.Project(source, []uint32{2, 1}); err == nil {
		t.Fatal("unsorted projection accepted")
	}
	if _, err = plugin.Project(source, []uint32{4}); err == nil {
		t.Fatal("out-of-range projection accepted")
	}
	var nilPlugin *BoundRelationPlugin
	if _, err = nilPlugin.Project(source, nil); err == nil {
		t.Fatal("nil plugin projection accepted")
	}
}

func TestBoundRelationPluginRenameDispatch(t *testing.T) {
	manager := astTestManager(t)
	plugin, err := NewBoundRelationPlugin(manager)
	astTestMust(t, err)
	source := NewBoundRelation(3, false)
	astTestMust(t, source.AddLessEqual(1, 2))
	astTestMust(t, source.AddLess(0, 1))
	renamed, err := plugin.Rename(source, []uint32{0, 1, 2})
	astTestMust(t, err)
	strict, weak, err := renamed.Bounds(1)
	astTestMust(t, err)
	if len(strict) != 2 || strict[0] != 0 || strict[1] != 2 || len(weak) != 0 {
		t.Fatal("rename dispatch strict rotation", strict, weak)
	}
	strict, weak, err = renamed.Bounds(2)
	astTestMust(t, err)
	if len(strict) != 0 || len(weak) != 1 || weak[0] != 0 {
		t.Fatal("rename dispatch weak rotation", strict, weak)
	}
	astTestMust(t, renamed.AddLess(2, 1))
	strict, _, err = source.Bounds(0)
	astTestMust(t, err)
	if len(strict) != 2 || strict[0] != 1 || strict[1] != 2 {
		t.Fatal("renamed result mutated source", strict)
	}
	if _, err = plugin.Rename(source, nil); err == nil {
		t.Fatal("empty rename cycle accepted")
	}
	if _, err = plugin.Rename(source, []uint32{0, 0}); err == nil {
		t.Fatal("duplicate rename column accepted")
	}
	var nilPlugin *BoundRelationPlugin
	if _, err = nilPlugin.Rename(source, []uint32{0}); err == nil {
		t.Fatal("nil plugin rename accepted")
	}
}

func TestBoundRelationPluginUnionAndWidenDispatch(t *testing.T) {
	manager := astTestManager(t)
	plugin, err := NewBoundRelationPlugin(manager)
	astTestMust(t, err)
	target, source := NewBoundRelation(3, false), NewBoundRelation(3, false)
	astTestMust(t, target.AddLess(0, 1))
	astTestMust(t, target.AddLessEqual(0, 2))
	astTestMust(t, source.AddLess(0, 1))
	delta := NewBoundRelation(3, true)
	changed, err := plugin.Union(target, source, delta, false)
	astTestMust(t, err)
	if !changed {
		t.Fatal("union dispatch missed lattice change")
	}
	strict, weak, err := target.Bounds(0)
	astTestMust(t, err)
	if len(strict) != 1 || strict[0] != 1 || len(weak) != 0 {
		t.Fatal("union dispatch result", strict, weak)
	}
	strict, weak, err = delta.Bounds(0)
	astTestMust(t, err)
	if delta.Empty() || len(strict) != 1 || strict[0] != 1 || len(weak) != 0 {
		t.Fatal("union dispatch delta", delta.Empty(), strict, weak)
	}
	// The bound lattice's widening is source-identical to union.
	widenTarget := NewBoundRelation(3, false)
	astTestMust(t, widenTarget.AddLess(0, 1))
	astTestMust(t, widenTarget.AddLessEqual(0, 2))
	changed, err = plugin.Union(widenTarget, source, nil, true)
	astTestMust(t, err)
	if !changed {
		t.Fatal("widen dispatch missed lattice change")
	}
	strict, weak, err = widenTarget.Bounds(0)
	astTestMust(t, err)
	if len(strict) != 1 || strict[0] != 1 || len(weak) != 0 {
		t.Fatal("widen dispatch result", strict, weak)
	}
	// An empty source leaves the target unchanged but resets a supplied delta.
	delta = NewBoundRelation(3, false)
	changed, err = plugin.Union(target, NewBoundRelation(3, true), delta, false)
	astTestMust(t, err)
	if changed || !delta.Empty() {
		t.Fatal("empty-source union contract", changed, delta.Empty())
	}
	if _, err = plugin.Union(target, source, target, false); err == nil {
		t.Fatal("aliased union delta accepted")
	}
	var nilPlugin *BoundRelationPlugin
	if _, err = nilPlugin.Union(target, source, nil, false); err == nil {
		t.Fatal("nil plugin union accepted")
	}
}
