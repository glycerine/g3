package ast

// Source: src/ast/fpa_decl_plugin.cpp:591-606.

import "github.com/glycerine/g3/pkg/util/z3exception"

// FPAOpFP constructs a floating-point value from separate sign, exponent,
// and significand bit-vectors. Its format follows the latter two widths.
const FPAOpFP int32 = astFPAFP

// floatingPointDeclarationError is an fpa_decl_plugin raise_exception.
func floatingPointDeclarationError(message string) error {
	return ASTException{z3exception.NewDefaultException(message)}
}

// floatingPointToFPUnexpected is mk_to_fp's final else message, without
// "(Real)" as upstream z3 and the fork have it (AUDIT.md ast_05-F2).
const floatingPointToFPUnexpected = "Unexpected argument combination for (_ to_fp eb sb). Supported argument combinations are: " +
	"((_ BitVec 1) (_ BitVec eb) (_ BitVec sb-1)), " +
	"(_ BitVec (eb+sb)), " +
	"(RoundingMode (_ BitVec (eb+sb))), " +
	"(RoundingMode (_ FloatingPoint eb' sb')), " +
	"(RoundingMode Int Real), " +
	"(RoundingMode Real Int), " +
	"(RoundingMode Int), and " +
	"(RoundingMode Real)."

func (m *ASTManager) floatingPointFPDecl(domain []ASTHandle) (astNodeRef, error) {
	if len(domain) != 3 {
		return astNodeRef{}, floatingPointDeclarationError("invalid number of arguments to fp")
	}
	var widths [3]uint32
	for i, sort := range domain {
		ref, err := m.internal(sort)
		if err != nil {
			return astNodeRef{}, err
		}
		if ref.slot == 0 || m.nodeAt(ref.slot).kind != ASTSort {
			return astNodeRef{}, ErrASTDomain
		}
		isBV, err := m.bitVectorSortRecognized(ref)
		if err != nil {
			return astNodeRef{}, err
		}
		if !isBV {
			return astNodeRef{}, floatingPointDeclarationError("sort mismatch, expected three bit-vectors, the first one of size 1.")
		}
		widths[i], err = m.bitVectorSortSize(ref, nil)
		if err != nil {
			return astNodeRef{}, err
		}
	}
	if widths[0] != 1 {
		return astNodeRef{}, floatingPointDeclarationError("sort mismatch, expected three bit-vectors, the first one of size 1.")
	}
	fpSort, err := m.mkFloatSort(widths[1], widths[2]+1)
	if err != nil {
		return astNodeRef{}, err
	}
	name, err := m.Symbols.Intern("fp")
	if err != nil {
		return astNodeRef{}, err
	}
	return m.pluginDecl(name, m.floatingPointPlugin.family, astFPAFP, nil, domain, fpSort, ASTFuncFlags{}, false)
}
