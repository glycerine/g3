package rel

// Source: src/muz/rel/dl_interval_relation.cpp and the concrete interval
// instantiation of dl_vector_relation.h.

import (
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// IntervalRelation is the concrete rational interval abstraction.
type IntervalRelation struct{ values *intervalColumns }

func NewIntervalEmpty(arity uint32) *IntervalRelation {
	return &IntervalRelation{newIntervalColumns(int(arity), true)}
}
func NewIntervalFull(arity uint32) *IntervalRelation {
	return &IntervalRelation{newIntervalColumns(int(arity), false)}
}
func (r *IntervalRelation) Empty() bool { return r == nil || r.values == nil || r.values.empty }

// Clone copies the relation using vector_relation::copy semantics. The source
// copy operation copies each representative's interval and does not recreate
// the equality classes, so the clone has independent columns.
func (r *IntervalRelation) Clone() (*IntervalRelation, error) {
	if r == nil || r.values == nil {
		return nil, ast.ErrASTDomain
	}
	return cloneIntervalRelation(r), nil
}

// Display writes the source vector_relation diagnostic format.
func (r *IntervalRelation) Display(out io.Writer) error {
	if r == nil || r.values == nil || out == nil {
		return ast.ErrASTDomain
	}
	if r.Empty() {
		_, err := fmt.Fprintln(out, "empty")
		return err
	}
	for i := range r.values.columns {
		root := r.values.find(uint32(i))
		if i != int(root) {
			if _, err := fmt.Fprintf(out, "%d = %d ", i, root); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(out, "%d in %s\n", i, intervalRelationString(r.values.columns[i])); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out)
	return err
}

func intervalRelationString(value smtcore.RelationInterval) string {
	lower, lowerOpen, upper, upperOpen := value.Bounds()
	left, right := "[", "]"
	if lowerOpen {
		left = "("
	}
	if upperOpen {
		right = ")"
	}
	lowerText, upperText := "-oo", "oo"
	if lower != nil {
		lowerText = lower.RatString()
	}
	if upper != nil {
		upperText = upper.RatString()
	}
	return left + lowerText + ", " + upperText + right
}

func (r *IntervalRelation) FilterEqual(column uint32, value *big.Rat) error {
	if r == nil || r.values == nil || value == nil {
		return ast.ErrASTDomain
	}
	return r.values.intersect(column, smtcore.NewRelationInterval(value, false, value, false))
}

func (r *IntervalRelation) FilterInterpreted(manager *ast.ASTManager, condition ast.ASTHandle) error {
	if r == nil || r.values == nil || manager == nil {
		return ast.ErrASTDomain
	}
	return r.values.filterInterpreted(manager, condition)
}

func (r *IntervalRelation) Union(source *IntervalRelation, widen bool) (bool, error) {
	if r == nil || source == nil {
		return false, ast.ErrASTDomain
	}
	return unionIntervalColumns(r.values, source.values, widen)
}

// UnionWithDelta applies a union or widening and updates delta when the source
// operation reports a change. An empty source resets delta to empty.
func (r *IntervalRelation) UnionWithDelta(source, delta *IntervalRelation, widen bool) (bool, error) {
	if r == nil || source == nil || delta == nil || r.values == nil || source.values == nil || delta.values == nil || r == delta || source == delta || len(r.values.columns) != len(source.values.columns) || len(r.values.columns) != len(delta.values.columns) {
		return false, ast.ErrASTDomain
	}
	changed, err := unionIntervalColumns(r.values, source.values, widen)
	if err != nil {
		return false, err
	}
	if source.Empty() {
		delta.values.setEmpty()
	} else if changed {
		// vector_relation::copy copies source values into fresh destination
		// columns; its second merge loop leaves those columns independent.
		copy := newIntervalColumns(len(r.values.columns), false)
		for i := range copy.columns {
			copy.columns[i] = r.values.columns[r.values.find(uint32(i))]
		}
		delta.values = copy
	}
	return changed, nil
}

func JoinIntervalRelations(left, right *IntervalRelation, leftColumns, rightColumns []uint32) (*IntervalRelation, error) {
	if left == nil || right == nil {
		return nil, ast.ErrASTDomain
	}
	values, err := joinIntervalColumns(left.values, right.values, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	return &IntervalRelation{values}, nil
}

func (r *IntervalRelation) Project(removed []uint32) (*IntervalRelation, error) {
	if r == nil {
		return nil, ast.ErrASTDomain
	}
	values, err := projectIntervalColumns(r.values, removed)
	if err != nil {
		return nil, err
	}
	return &IntervalRelation{values}, nil
}

// Rename rotates the columns in cycle, with each column moving to the next.
func (r *IntervalRelation) Rename(cycle []uint32) (*IntervalRelation, error) {
	if r == nil {
		return nil, ast.ErrASTDomain
	}
	values, err := renameIntervalColumns(r.values, cycle)
	if err != nil {
		return nil, err
	}
	return &IntervalRelation{values}, nil
}

// cloneIntervalRelation is interval_relation::clone (vector_relation copy).
func cloneIntervalRelation(source *IntervalRelation) *IntervalRelation {
	values := source.values
	copy := newIntervalColumns(len(values.columns), values.empty)
	if values.empty {
		return &IntervalRelation{values: copy}
	}
	// vector_relation::copy reads each representative's value, but its merge
	// loop uses the fresh destination's representatives, so classes split.
	for i := range values.columns {
		interval := values.columns[values.find(uint32(i))]
		lo, loOpen, hi, hiOpen := interval.Bounds()
		copy.columns[i] = smtcore.NewRelationInterval(lo, loOpen, hi, hiOpen)
	}
	return &IntervalRelation{values: copy}
}
