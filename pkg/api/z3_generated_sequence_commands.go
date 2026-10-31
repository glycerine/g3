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
		{167, "Z3_mk_seq_prefix"}, {168, "Z3_mk_seq_suffix"}, {169, "Z3_mk_seq_contains"},
		{170, "Z3_mk_seq_extract"}, {171, "Z3_mk_seq_replace"}, {172, "Z3_mk_seq_at"},
		{173, "Z3_mk_seq_length"}, {174, "Z3_mk_seq_index"}, {175, "Z3_mk_str_to_int"},
		{176, "Z3_mk_int_to_str"}, {177, "Z3_mk_seq_to_re"}, {178, "Z3_mk_seq_in_re"},
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
	var second, third ast.ASTHandle
	if id == 167 || id == 168 || id == 169 || id == 170 || id == 171 || id == 172 || id == 174 || id == 178 {
		input, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		second = input.handle
	}
	if id == 170 || id == 171 || id == 174 {
		input, e := r.astObject(owner, 3)
		if e != nil {
			return e
		}
		third = input.handle
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
	case 167:
		result, err = c.SequencePrefix(value, second)
	case 168:
		result, err = c.SequenceSuffix(value, second)
	case 169:
		result, err = c.SequenceContains(value, second)
	case 170:
		result, err = c.SequenceExtract(value, second, third)
	case 171:
		result, err = c.SequenceReplace(value, second, third)
	case 172:
		result, err = c.SequenceAt(value, second)
	case 173:
		result, err = c.SequenceLength(value)
	case 174:
		result, err = c.SequenceIndex(value, second, third)
	case 175:
		result, err = c.StringToInt(value)
	case 176:
		result, err = c.IntToString(value)
	case 177:
		result, err = c.SequenceToRegex(value)
	case 178:
		result, err = c.SequenceInRegex(value, second)
	}
	if err != nil {
		return err
	}
	switch id {
	case 155, 157, 159, 161, 164, 165, 166, 167, 168, 169, 170, 171, 172, 173, 174, 175, 176, 177, 178:
		return r.storeAST(owner, result)
	}
	return nil
}
