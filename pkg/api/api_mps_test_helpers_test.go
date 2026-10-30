package api

import "fmt"

func apiMPSPair(column, row string, value float64) string {
	return fmt.Sprintf("    %-8s  %-8s  %.12g\n", column, row, value)
}

func apiMPSBound(kind, data string) string {
	return fmt.Sprintf(" %-2s %-8s  %s\n", kind, "BND", data)
}

func apiOneVariableMPS(rowKind string, rhs float64, bounds string) string {
	return "NAME ONE\nROWS\n N COST\n " + rowKind + " LIMIT\nCOLUMNS\n" +
		apiMPSPair("x", "COST", 2) + apiMPSPair("x", "LIMIT", 1) +
		"RHS\n" + apiMPSPair("RHS1", "LIMIT", rhs) + bounds + "ENDATA\n"
}
