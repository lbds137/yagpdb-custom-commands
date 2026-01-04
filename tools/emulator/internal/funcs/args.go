package funcs

import (
	"fmt"
)

// ParsedArgs represents parsed command arguments.
type ParsedArgs struct {
	args     []interface{}
	argDefs  []*ArgDef
	rawArgs  []string
	required int
}

// ArgDef defines an argument specification.
type ArgDef struct {
	Type string
	Name string
}

// Get returns the argument at the given index.
func (pa *ParsedArgs) Get(index int) interface{} {
	if index < 0 || index >= len(pa.args) {
		return nil
	}
	return pa.args[index]
}

// IsSet returns true if the argument at the given index was provided.
func (pa *ParsedArgs) IsSet(index int) bool {
	return index >= 0 && index < len(pa.args) && pa.args[index] != nil
}

// ParseArgs creates a ParsedArgs from command arguments.
// In the emulator, we use the context's CmdArgs directly.
type ArgsParser struct {
	cmdArgs []interface{}
}

// NewArgsParser creates a new argument parser.
func NewArgsParser(cmdArgs []interface{}) *ArgsParser {
	return &ArgsParser{cmdArgs: cmdArgs}
}

// ParseArgs parses arguments according to the given definitions.
func (ap *ArgsParser) ParseArgs(numRequired int, failedMessage string, argDefs ...*ArgDef) (*ParsedArgs, error) {
	if len(ap.cmdArgs) < numRequired {
		return nil, fmt.Errorf("%s", failedMessage)
	}

	pa := &ParsedArgs{
		args:     make([]interface{}, len(argDefs)),
		argDefs:  argDefs,
		required: numRequired,
	}

	for i, def := range argDefs {
		if i < len(ap.cmdArgs) {
			pa.args[i] = convertArg(ap.cmdArgs[i], def.Type)
		}
	}

	return pa, nil
}

// Carg creates an argument definition.
func Carg(argType, name string, opts ...interface{}) *ArgDef {
	return &ArgDef{
		Type: argType,
		Name: name,
	}
}

// convertArg converts an argument to the specified type.
func convertArg(arg interface{}, argType string) interface{} {
	switch argType {
	case "int":
		return ToInt(arg)
	case "int64":
		return ToInt64(arg)
	case "float", "float64":
		return ToFloat64(arg)
	case "string":
		return ToString(arg)
	case "user", "userid", "member":
		// Return as-is for now (mock)
		return arg
	case "channel":
		return arg
	case "role":
		return arg
	case "duration":
		return ToDuration(arg)
	default:
		return arg
	}
}
