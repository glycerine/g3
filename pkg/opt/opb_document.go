package opt

// Source: src/opt/opt_parse.cpp opb parser.

type OPBLiteral struct {
	Variable int32
	Negative bool
}

// OPBTerm's Coefficient is parse_coeff_r's digit text (an optional '-'
// then decimal digits); the loader makes rational(num.c_ptr()) from it.
type OPBTerm struct {
	Coefficient string
	Literals    []OPBLiteral
}

type OPBRelation uint8

const (
	OPBGreaterEqual OPBRelation = iota
	OPBEqual
	OPBLessEqual
)

type OPBConstraint struct {
	Terms    []OPBTerm
	Relation OPBRelation
	Bound    string // parse_coeff_r's digit text
}

type OPBObjective struct {
	MinimizeSyntax bool
	Terms          []OPBTerm
}

type OPBDocument struct {
	Objectives  []OPBObjective
	Constraints []OPBConstraint
}

func copyOPBTerms(terms []OPBTerm) []OPBTerm {
	result := make([]OPBTerm, len(terms))
	for i := range terms {
		result[i].Coefficient = terms[i].Coefficient
		result[i].Literals = append([]OPBLiteral(nil), terms[i].Literals...)
	}
	return result
}

func (d OPBDocument) Copy() OPBDocument {
	result := OPBDocument{Objectives: make([]OPBObjective, len(d.Objectives)), Constraints: make([]OPBConstraint, len(d.Constraints))}
	for i := range d.Objectives {
		result.Objectives[i].MinimizeSyntax = d.Objectives[i].MinimizeSyntax
		result.Objectives[i].Terms = copyOPBTerms(d.Objectives[i].Terms)
	}
	for i := range d.Constraints {
		result.Constraints[i].Terms = copyOPBTerms(d.Constraints[i].Terms)
		result.Constraints[i].Relation = d.Constraints[i].Relation
		result.Constraints[i].Bound = d.Constraints[i].Bound
	}
	return result
}
