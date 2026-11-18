package opt

// Source: src/opt/opt_parse.cpp opb parser.

type OPBLiteral struct {
	Variable int32 // parse_int's value; the source makes symbol(id)
	Negative bool
}

// OPBTerm's Coefficient is parse_coeff_r's digit text (an optional '-'
// then decimal digits, "0" when no digit follows); the loader makes
// rational(num.c_ptr()) from it.
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

// OPBStatement is one objective (opb::parse_objective) or hard constraint
// (opb::parse_constraint). The source builds and registers each statement
// as soon as it is read, so the statements keep file order.
type OPBStatement struct {
	Objective      bool
	MinimizeSyntax bool // objective only: min: (true) or max: (false)
	Terms          []OPBTerm
	Relation       OPBRelation // constraint only
	Bound          string      // constraint only: parse_coeff_r's digit text
}

type OPBDocument struct {
	Statements []OPBStatement
}

// ObjectiveCount is the number of objectives the source registers.
func (d OPBDocument) ObjectiveCount() int {
	n := 0
	for i := range d.Statements {
		if d.Statements[i].Objective {
			n++
		}
	}
	return n
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
	result := OPBDocument{Statements: make([]OPBStatement, len(d.Statements))}
	for i := range d.Statements {
		result.Statements[i] = d.Statements[i]
		result.Statements[i].Terms = copyOPBTerms(d.Statements[i].Terms)
	}
	return result
}
