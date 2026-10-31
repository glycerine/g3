package api

// Source: generated api_commands.cpp, sequence and string constructors.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedSequenceCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{155, "Z3_mk_seq_sort"}, {156, "Z3_is_seq_sort"}, {157, "Z3_mk_re_sort"},
		{158, "Z3_is_re_sort"}, {159, "Z3_mk_string_sort"}, {160, "Z3_is_string_sort"},
		{161, "Z3_mk_string"}, {162, "Z3_is_string"}, {163, "Z3_get_string"},
		{164, "Z3_mk_seq_empty"}, {165, "Z3_mk_seq_unit"}, {166, "Z3_mk_seq_concat"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replaySequenceCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replaySequenceCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	c := owner.context
	var value ast.ASTHandle
	if id != 159 && id != 161 && id != 166 {
		var input *replayASTOwner
		input, err = r.astObject(owner, 1)
		if err != nil {
			return err
		}
		value = input.handle
	}
	var result ast.ASTHandle
	switch id {
	case 155:
		result, err = c.SequenceSort(value)
	case 156:
		_, err = c.IsSequenceSort(value)
	case 157:
		result, err = c.RegexSort(value)
	case 158:
		_, err = c.IsRegexSort(value)
	case 159:
		result, err = c.StringSort()
	case 160:
		_, err = c.IsStringSort(value)
	case 161:
		var text string
		text, err = r.String(1)
		if err == nil {
			result, err = c.String(text)
		}
	case 162:
		_, _, err = c.StringValue(value)
	case 163:
		var matched bool
		_, matched, err = c.StringValue(value)
		if err == nil && !matched {
			err = ast.ErrASTDomain
		}
	case 164:
		result, err = c.SequenceEmpty(value)
	case 165:
		result, err = c.SequenceUnit(value)
	case 166:
		var args []ast.ASTHandle
		args, err = replayASTArray(r, owner, 1, 2)
		if err == nil {
			result, err = c.SequenceConcat(args...)
		}
	}
	if err != nil {
		return err
	}
	switch id {
	case 155, 157, 159, 161, 164, 165, 166:
		return r.storeAST(owner, result)
	}
	return nil
}
