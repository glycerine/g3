package model

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// implicantNumberIs reports whether m_numbers holds exactly the rational
// written as text for h.
func implicantNumberIs(i *ModelImplicant, h ast.ASTHandle, text string) (equal bool, err error) {
	got := i.numbers[h]
	if got == nil {
		return false, nil
	}
	q := i.q()
	want := unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&want)) }()
	if err = q.SetString(&want, text); err != nil {
		return false, err
	}
	return q.Equal(got, &want)
}

func arrayTableMustBool(t interface{ Fatal(...any) }) func(bool, error) bool {
	return func(v bool, err error) bool {
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}
