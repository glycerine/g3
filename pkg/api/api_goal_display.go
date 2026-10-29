package api

// Source: src/api/api_goal.cpp:167-181.

import (
	"bytes"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (g *APIGoal) String() (string, error) {
	if err := g.open(); err != nil {
		return "", err
	}
	var output bytes.Buffer
	printer, err := ast.NewSimpleASTPrinterContext(g.manager, &output, &output)
	if err != nil {
		return "", err
	}
	output.WriteString("(goal")
	size, err := g.goal.Size()
	if err != nil {
		return "", err
	}
	for i := uint32(0); i < size; i++ {
		formula, formErr := g.goal.Form(i)
		if formErr != nil {
			return "", formErr
		}
		output.WriteString("\n  ")
		if formErr = printer.DisplayExpr(&output, formula, 0); formErr != nil {
			return "", formErr
		}
	}
	output.WriteString(")\n")
	return strings.TrimSuffix(output.String(), "\n"), nil
}
