package opt

// Source: src/util/sorting_network.h psort_nw::sorting/merge/dsorting, used
// by src/opt/sortmax.cpp. This is a concrete Boolean-wire implementation of
// the GE comparator mode (fork fix #A; see comparator).

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) network(inputs []ast.ASTHandle) ([]ast.ASTHandle, error) {
	switch len(inputs) {
	case 0:
		return nil, nil
	case 1:
		return append([]ast.ASTHandle(nil), inputs...), nil
	case 2:
		return e.merge(inputs[:1], inputs[1:])
	}
	if e.useDirectSort(len(inputs)) {
		return e.directSort(inputs)
	}
	middle := len(inputs) / 2
	left, err := e.network(inputs[:middle])
	if err != nil {
		return nil, err
	}
	right, err := e.network(inputs[middle:])
	if err != nil {
		return nil, err
	}
	return e.merge(left, right)
}

// directSort is dsorting(n, n, xs) in the GE mode fork fix #A gives
// sortmax's psort_nw (see comparator): each output implies that enough
// inputs are true.
func (e *sortMaxEncoder) directSort(inputs []ast.ASTHandle) ([]ast.ASTHandle, error) {
	outputs := make([]ast.ASTHandle, len(inputs))
	for i := range outputs {
		wire, err := e.fresh()
		if err != nil {
			return nil, err
		}
		outputs[i] = wire
	}
	for k := 1; k <= len(outputs); k++ {
		notOutput, err := e.negate(outputs[k-1])
		if err != nil {
			return nil, err
		}
		if err = e.addSubset(notOutput, inputs, len(inputs)-k+1); err != nil {
			return nil, err
		}
	}
	return outputs, nil
}

// addSubset is add_subset(false, k, 0, lits, n, xs) with lits = {first}:
// one clause first or xs[i1] or ... or xs[ik] per k-subset of the inputs.
func (e *sortMaxEncoder) addSubset(first ast.ASTHandle, inputs []ast.ASTHandle, k int) error {
	literals := []ast.ASTHandle{first}
	var visit func(k, offset int) error
	visit = func(k, offset int) error {
		if k == 0 {
			return e.clause(literals...)
		}
		for i := offset; i < len(inputs)-k+1; i++ {
			literals = append(literals, inputs[i])
			if err := visit(k-1, i+1); err != nil {
				return err
			}
			literals = literals[:len(literals)-1]
		}
		return nil
	}
	return visit(k, 0)
}

func (e *sortMaxEncoder) merge(left, right []ast.ASTHandle) ([]ast.ASTHandle, error) {
	a, b := len(left), len(right)
	if a == 0 {
		return append([]ast.ASTHandle(nil), right...), nil
	}
	if b == 0 {
		return append([]ast.ASTHandle(nil), left...), nil
	}
	if a == 1 && b == 1 {
		high, low, err := e.comparator(left[0], right[0])
		if err != nil {
			return nil, err
		}
		return []ast.ASTHandle{high, low}, nil
	}
	if e.useDirectMerge(a, b, a+b) {
		return e.directMerge(left, right)
	}
	if a%2 == 0 && b%2 == 1 {
		return e.merge(right, left)
	}
	evenLeft, oddLeft := splitSortMax(left)
	evenRight, oddRight := splitSortMax(right)
	first, err := e.merge(evenLeft, evenRight)
	if err != nil {
		return nil, err
	}
	second, err := e.merge(oddLeft, oddRight)
	if err != nil {
		return nil, err
	}
	return e.interleave(first, second)
}

// directMerge is dsmerge(a + b, a, as, b, bs) in GE mode (fork fix #A).
func (e *sortMaxEncoder) directMerge(left, right []ast.ASTHandle) ([]ast.ASTHandle, error) {
	a, b := len(left), len(right)
	c := a + b
	outputs := make([]ast.ASTHandle, c)
	for i := range outputs {
		wire, err := e.fresh()
		if err != nil {
			return nil, err
		}
		outputs[i] = wire
	}
	for k := 0; k < c; k++ {
		notOutput, err := e.negate(outputs[k])
		if err != nil {
			return nil, err
		}
		if a <= k {
			if err = e.clause(notOutput, right[k-a]); err != nil {
				return nil, err
			}
		}
		if b <= k {
			if err = e.clause(notOutput, left[k-b]); err != nil {
				return nil, err
			}
		}
		for i := 0; i < minInt(a, k+1); i++ {
			j := k - i
			if j < b {
				if err = e.clause(notOutput, left[i], right[j]); err != nil {
					return nil, err
				}
			}
		}
	}
	return outputs, nil
}

func splitSortMax(values []ast.ASTHandle) (even, odd []ast.ASTHandle) {
	even = make([]ast.ASTHandle, 0, (len(values)+1)/2)
	odd = make([]ast.ASTHandle, 0, len(values)/2)
	for i, value := range values {
		if i%2 == 0 {
			even = append(even, value)
		} else {
			odd = append(odd, value)
		}
	}
	return even, odd
}

func (e *sortMaxEncoder) interleave(even, odd []ast.ASTHandle) ([]ast.ASTHandle, error) {
	outputs := make([]ast.ASTHandle, 0, len(even)+len(odd))
	outputs = append(outputs, even[0])
	count := len(even) - 1
	if len(odd) < count {
		count = len(odd)
	}
	for i := 0; i < count; i++ {
		high, low, err := e.comparator(even[i+1], odd[i])
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, high, low)
	}
	if len(even) == len(odd) {
		outputs = append(outputs, odd[count])
	} else if len(even) == len(odd)+2 {
		outputs = append(outputs, even[count+1])
	}
	return outputs, nil
}
