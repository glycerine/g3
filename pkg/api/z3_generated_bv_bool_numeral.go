package api

// Source: generated api_commands.cpp, api_numeral.cpp and z3_replayer.cpp.

import (
	"encoding/binary"
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func replayBitVectorBoolNumeral(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	width, err := r.Uint(1)
	if err != nil {
		return err
	}
	words, err := r.UintArray(2)
	if err != nil {
		return err
	}
	if uint64(width) > uint64(len(words)) {
		return ErrZ3ReplayReference
	}
	// The source replayer casts its unsigned-int array to bool*. Preserve its
	// native byte order, including the four bool slots occupied by each word.
	// rational r(0); r += rational::power_of_two(i) for each set bit.
	m, err := owner.context.Manager()
	if err != nil {
		return err
	}
	var text string
	err = withRationals(m, 2, func(q *numeral.RationalManager, rs []numeral.Rational) error {
		r, power := &rs[0], &rs[1]
		var bytes [4]byte
		for i := uint32(0); i < width; i++ {
			if i%4 == 0 {
				binary.NativeEndian.PutUint32(bytes[:], words[i/4])
			}
			if bytes[i%4] != 0 {
				if e := errors.Join(powerOfTwo(q, i, power), q.Add(r, power, r)); e != nil {
					return e
				}
			}
		}
		var e error
		text, e = q.String(r)
		return e
	})
	if err != nil {
		return err
	}
	sort, err := owner.context.BitVectorSort(width)
	if err != nil {
		return err
	}
	value, err := owner.context.Numeral(text, sort)
	if err != nil {
		return err
	}
	return r.storeAST(owner, value)
}
