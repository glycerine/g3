package cmd_context

// Source: src/muz/fp/datalog_parser.cpp dtoken, dtoken_strings and
// reserved_symbols.

type dtoken uint8

const (
	dtkLP dtoken = iota
	dtkRP
	dtkString
	dtkID
	dtkNum
	dtkPeriod
	dtkInclude
	dtkComma
	dtkColon
	dtkWild
	dtkLeftArrow
	dtkEOS
	dtkNewline
	dtkError
	dtkNEQ
	dtkLT
	dtkGT
	dtkEQ
	dtkNeg
)

var dtokenStrings = [...]string{"(", ")", "<string>", "<id>", "<num>", ".", ".include", ",", ":", "_", ":-", "<eos>", "\\n", "<error>", "!=", "<", ">", "=", "!"}

// datalogReservedSymbols is reserved_symbols::m_str2token.
var datalogReservedSymbols = map[string]dtoken{
	":-":       dtkLeftArrow,
	"_":        dtkWild,
	".":        dtkPeriod,
	"!=":       dtkNEQ,
	"=":        dtkEQ,
	"<":        dtkLT,
	">":        dtkGT,
	":":        dtkColon,
	".include": dtkInclude,
	"!":        dtkNeg,
}

// string2dtoken is reserved_symbols::string2dtoken.
func string2dtoken(str string) dtoken {
	if tok, found := datalogReservedSymbols[str]; found {
		return tok
	}
	return dtkID
}
