package realclosure

// Source: src/math/realclosure/realclosure.cpp:4609-4670. The caller owns
// (and deletes) the returned matrices.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func solveCompletedSignExtension(existing *signDet, counts multiRootSignCounts, expanded []int) (signs signMatrix, cardinalities []int, qMatrix signMatrix, err error) {
	if existing == nil || existing.deleted || existing.matrix.rows == 0 || len(expanded) == 0 {
		return signMatrix{}, nil, signMatrix{}, arena.ErrInvalidSlot
	}
	z := existing.matrix.z
	var useful bool
	qMatrix, useful, err = signDetMatrix(z, counts.equal, counts.positive, counts.negative)
	if err != nil || !useful {
		return signMatrix{}, nil, signMatrix{}, errors.Join(err, arena.ErrInvalidSlot)
	}
	signs = signMatrix{z: z}
	fail := func(cause error) (signMatrix, []int, signMatrix, error) {
		return signMatrix{}, nil, signMatrix{}, errors.Join(cause, signs.del(), qMatrix.del())
	}
	if err = signs.tensorProduct(&existing.matrix, &qMatrix); err != nil {
		return fail(err)
	}
	if len(expanded) != signs.rows {
		return fail(arena.ErrInvalidSlot)
	}
	var solvable bool
	cardinalities, solvable, err = signs.solveInt(expanded)
	if err != nil {
		return fail(err)
	}
	if !solvable || len(cardinalities) != signs.cols {
		return fail(arena.ErrInvalidSlot)
	}
	return signs, cardinalities, qMatrix, nil
}
