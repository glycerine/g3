package lp

// Source: src/test/argument_parser.h; src/test/lp.cpp:1883-1929.
// Concrete catalogue and parser reached by the retained LP test driver.

import (
	"io"
	"sort"
	"strconv"
	"strings"
)

type driverOption struct {
	name, help string
	value      bool
}

// DriverArguments retains the source parser's accumulated state across Parse
// calls. Args includes argv[0] when supplied; it is a free argument in source.
// m_used_options_with_after_string is a std::unordered_map whose libstdc++
// node order is printed; it uses the approved StdUnorderedStringMap, whose
// values index valueText (AUDIT.md util_lp_01-F1). Do not copy after use.
type DriverArguments struct {
	args         []string
	usedFlags    map[string]bool
	usedValues   *StdUnorderedStringMap
	valueText    []string
	freeArgs     []string
	ErrorMessage string
}

func NewDriverArguments(args []string) *DriverArguments {
	return &DriverArguments{args: append([]string(nil), args...), usedFlags: make(map[string]bool), usedValues: NewStdUnorderedStringMap()}
}

// setValue is m_used_options_with_after_string[option] = value.
func (a *DriverArguments) setValue(option, value string) {
	a.valueText = append(a.valueText, value)
	a.usedValues.Put(option, uint32(len(a.valueText)-1))
}

func (a *DriverArguments) Parse() bool {
	ok := true
	for i := 0; i < len(a.args); i++ {
		arg := a.args[i]
		var option *driverOption
		for j := range lpDriverOptions {
			if lpDriverOptions[j].name == arg {
				option = &lpDriverOptions[j]
				break
			}
		}
		if option != nil && !option.value {
			a.usedFlags[arg] = true
		} else if option != nil {
			if i == len(a.args)-1 {
				a.ErrorMessage = "Argument is missing after " + arg
				return false
			}
			i++
			a.setValue(arg, a.args[i])
		} else {
			if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "//") {
				ok = false
			}
			a.freeArgs = append(a.freeArgs, arg)
		}
	}
	return ok
}

func (a *DriverArguments) OptionIsUsed(option string) bool {
	if a.usedFlags[option] {
		return true
	}
	_, ok := a.usedValues.Get2(option)
	return ok
}

func (a *DriverArguments) OptionValue(option string) string {
	index, ok := a.usedValues.Get2(option)
	if !ok {
		return ""
	}
	return a.valueText[index]
}

func (a *DriverArguments) UsageString() string {
	var unknown []string
	for _, arg := range a.freeArgs {
		// Source usage and parse deliberately use different second prefixes.
		if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "\\") {
			unknown = append(unknown, arg)
		}
	}
	var text strings.Builder
	if len(unknown) != 0 {
		text.WriteString("Unknown options:")
	}
	for _, arg := range unknown {
		text.WriteString(arg + ",")
	}
	text.WriteString("\nUsage:\n")
	// m_options, then m_options_with_after_string, each walked in libstdc++
	// unordered_map node order (AUDIT.md util_lp_01-F1).
	for _, value := range []bool{false, true} {
		table := NewStdUnorderedStringMap()
		for i, option := range lpDriverOptions {
			if option.value == value {
				table.Put(option.name, uint32(i))
			}
		}
		for name, i := range table.All() {
			help := lpDriverOptions[i].help
			switch {
			case value && help == "":
				text.WriteString(name + "  \"option value\"\n")
			case value:
				text.WriteString(name + " \"" + help + "\"\n")
			case help == "":
				text.WriteString(name + " \n")
			default:
				text.WriteString(name + " /" + help + "\n")
			}
		}
	}
	return text.String()
}

func writeDriverLine(out io.Writer, line string) error {
	if out == nil {
		return nil
	}
	if err := writeConstraintText(out, line+"\n"); err != nil {
		return err
	}
	return dualFlush(out)
}

func (a *DriverArguments) Print(out io.Writer) error {
	if len(a.usedFlags) == 0 && a.usedValues.Len() == 0 && len(a.freeArgs) == 0 {
		return writeDriverLine(out, "no options are given")
	}
	if err := writeDriverLine(out, "options are: "); err != nil {
		return err
	}
	flags := make([]string, 0, len(a.usedFlags))
	for flag := range a.usedFlags {
		flags = append(flags, flag)
	}
	sort.Strings(flags) // source std::set iteration
	for _, flag := range flags {
		if err := writeDriverLine(out, flag); err != nil {
			return err
		}
	}
	for option, index := range a.usedValues.All() {
		if err := writeDriverLine(out, option+" "+a.valueText[index]); err != nil {
			return err
		}
	}
	if len(a.freeArgs) != 0 {
		if err := writeDriverLine(out, "free arguments are: "); err != nil {
			return err
		}
		for _, arg := range a.freeArgs {
			if err := writeDriverLine(out, arg+" "); err != nil {
				return err
			}
		}
	}
	return nil
}

// Driver numeric conversion uses Go's standard parsers. Nonempty malformed or
// overflowing tokens return an error rather than emulate libc prefix parsing.
// Valid signed int32 values preserve the source unsigned assignment, including
// negative wrap. Empty option values remain present but do not supply a number.
func (a *DriverArguments) uintValue(option string) (uint32, bool, error) {
	text := a.OptionValue(option)
	if text == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseInt(strings.TrimSpace(text), 10, 32)
	return uint32(value), true, err
}

func (a *DriverArguments) doubleValue(option string) (float64, bool, error) {
	text := a.OptionValue(option)
	if text == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	return value, true, err
}

// TimeAndIterationLimits preserves source max-iterations-before-time parsing.
// The returned argument order matches get_time_limit_and_max_iters_from_parser.
func (a *DriverArguments) TimeAndIterationLimits() (timeLimit, maxIterations uint32, err error) {
	maxIterations, _, err = a.uintValue("--max_iters")
	if err != nil {
		return
	}
	timeLimit, _, err = a.uintValue("--time_limit")
	return
}
